package observability

import (
	"encoding/json"
	"log/slog"
	mcpctx "mcp-runtime-go/internal/context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditLogger_Log(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")
	logger := NewAuditLogger(logPath)

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(mcpctx.WithRequestID(req.Context(), "test-rid"))
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	req.Header.Set("User-Agent", strings.Repeat("a", 300))

	fields := map[string]interface{}{
		"custom": "value",
		"secret": "should-not-be-here",
	}

	logger.LogWithIP("test_event", "1.2.3.4", req, fields)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry["event"] != "test_event" {
		t.Errorf("expected event test_event, got %v", entry["event"])
	}
	if entry["src_ip"] != "1.2.3.4" {
		t.Errorf("expected src_ip 1.2.3.4, got %v", entry["src_ip"])
	}
	if entry["request_id"] != "test-rid" {
		t.Errorf("expected request_id test-rid, got %v", entry["request_id"])
	}
	if ua := entry["ua"].(string); len(ua) != 200 {
		t.Errorf("expected UA length 200, got %d", len(ua))
	}
	if entry["custom"] != "value" {
		t.Errorf("expected custom field, got %v", entry["custom"])
	}
	if entry["secret"] != "[REDACTED]" {
		t.Errorf("expected secret redaction, got %v", entry["secret"])
	}
}

func TestAuditLogger_RedactsSensitiveKeys(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")
	logger := NewAuditLogger(logPath)

	req := httptest.NewRequest("POST", "/", nil)
	fields := map[string]interface{}{
		"access_token":  "abc",
		"client_secret": "def",
		"auth_code":     "ghi",
	}

	logger.LogWithIP("test_event", "1.2.3.4", req, fields)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	for _, key := range []string{"access_token", "client_secret", "auth_code"} {
		if entry[key] != "[REDACTED]" {
			t.Errorf("expected %s redacted, got %v", key, entry[key])
		}
	}
}

func TestAuditLogger_RedactsSensitiveValuesInGenericFields(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")
	logger := NewAuditLogger(logPath)

	req := httptest.NewRequest("POST", "/", nil)
	fields := map[string]interface{}{
		"reason":  "upstream rejected Bearer abc123",
		"details": "client_secret=def456",
		"nested": map[string]interface{}{
			"message": "code_verifier=ghi789",
		},
	}

	logger.LogWithIP("test_event", "1.2.3.4", req, fields)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	for _, key := range []string{"reason", "details"} {
		if entry[key] != "[REDACTED]" {
			t.Errorf("expected %s redacted, got %v", key, entry[key])
		}
	}
	nested := entry["nested"].(map[string]interface{})
	if nested["message"] != "[REDACTED]" {
		t.Errorf("expected nested.message redacted, got %v", nested["message"])
	}
}

func TestAuditLogger_RedactsSliceAndNonString(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")
	logger := NewAuditLogger(logPath)

	fields := map[string]interface{}{
		"list": []interface{}{
			"Bearer secret123",
			map[string]interface{}{"code_verifier": "v123"},
			123, // Non-string should be preserved
		},
		"auth_code": 456, // Sensitive key but non-string value, should still redact based on key
	}

	logger.LogWithIP("test_event", "", nil, fields)

	data, _ := os.ReadFile(logPath)
	var entry map[string]interface{}
	json.Unmarshal(data, &entry)

	list := entry["list"].([]interface{})
	if list[0] != "[REDACTED]" {
		t.Errorf("expected list[0] redacted, got %v", list[0])
	}
	if list[1].(map[string]interface{})["code_verifier"] != "[REDACTED]" {
		t.Errorf("expected list[1].code_verifier redacted")
	}
	if list[2] != float64(123) {
		t.Errorf("expected list[2] preserved as number, got %v", list[2])
	}
	if entry["auth_code"] != "[REDACTED]" {
		t.Errorf("expected auth_code key to redact regardless of value type")
	}
}

func TestAuditLogger_EmptyPath(t *testing.T) {
	logger := NewAuditLogger("")
	// Should not panic or error
	logger.LogWithIP("event", "1.2.3.4", nil, nil)
}

func TestAuditLogger_NoRequest(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")
	logger := NewAuditLogger(logPath)

	logger.LogWithIP("event_no_req", "1.2.3.4", nil, nil)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}

	var entry map[string]interface{}
	json.Unmarshal(data, &entry)

	if entry["src_ip"] != nil {
		t.Error("src_ip should be nil when request is nil")
	}
}

func TestAuditLogger_RemoteAddrFallback(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")
	logger := NewAuditLogger(logPath)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "9.8.7.6:1234"
	logger.LogWithIP("event", "", req, nil)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry["src_ip"] != "9.8.7.6:1234" {
		t.Fatalf("expected src_ip fallback to RemoteAddr, got %v", entry["src_ip"])
	}
}

func TestAuditLogger_MarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")
	logger := NewAuditLogger(logPath)

	req := httptest.NewRequest("GET", "/", nil)
	logger.LogWithIP("event", "1.2.3.4", req, map[string]interface{}{
		"bad": make(chan int),
	})

	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("expected no log file on marshal failure, got err=%v", err)
	}
}

func TestAuditLogger_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")
	logger := NewAuditLogger(logPath)

	req := httptest.NewRequest("GET", "/", nil)
	logger.LogWithIP("perm_test", "1.2.3.4", req, nil)

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("failed to stat audit log: %v", err)
	}
	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("expected audit log permissions 0600, got %04o", mode)
	}
}

func TestAuditLogger_Ping(t *testing.T) {
	t.Run("Empty path returns nil", func(t *testing.T) {
		a := NewAuditLogger("")
		if err := a.Ping(); err != nil {
			t.Errorf("Ping with empty path should return nil, got %v", err)
		}
	})

	t.Run("Valid path succeeds", func(t *testing.T) {
		tmpDir := t.TempDir()
		a := NewAuditLogger(filepath.Join(tmpDir, "audit.log"))
		if err := a.Ping(); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	t.Run("Invalid path fails", func(t *testing.T) {
		a := NewAuditLogger("/nonexistent/path/audit.log")
		if err := a.Ping(); err == nil {
			t.Error("expected Ping to fail for non-existent directory")
		}
	})
}

func TestInitLogger(t *testing.T) {
	InitLogger(slog.LevelInfo)
}

func TestAuditLogger_WriteFailureIncrementsCounter(t *testing.T) {
	// Point to an unwritable path to force an open failure.
	before := AuditWriteFailures.Get()
	a := NewAuditLogger("/nonexistent/path/audit.log")
	req := httptest.NewRequest("GET", "/", nil)
	a.LogWithIP("fail_event", "1.2.3.4", req, nil)

	if AuditWriteFailures.Get() <= before {
		t.Error("expected AuditWriteFailures counter to increment on write failure")
	}
}

func TestAuditLogger_WriteAndNewlineFailures(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")
	logger := NewAuditLogger(logPath)
	req := httptest.NewRequest("GET", "/", nil)

	origWrite := auditWrite
	t.Cleanup(func() { auditWrite = origWrite })

	t.Run("write failure", func(t *testing.T) {
		calls := 0
		auditWrite = func(_ *os.File, _ []byte) (int, error) {
			calls++
			return 0, os.ErrInvalid
		}
		before := AuditWriteFailures.Get()
		logger.LogWithIP("event", "1.2.3.4", req, nil)
		if calls != 1 {
			t.Fatalf("expected one write attempt, got %d", calls)
		}
		if AuditWriteFailures.Get() <= before {
			t.Fatal("expected write failure counter increment")
		}
	})

	t.Run("newline failure", func(t *testing.T) {
		calls := 0
		auditWrite = func(_ *os.File, _ []byte) (int, error) {
			calls++
			if calls == 1 {
				return 1, nil
			}
			return 0, os.ErrInvalid
		}
		before := AuditWriteFailures.Get()
		logger.LogWithIP("event", "1.2.3.4", req, nil)
		if calls != 2 {
			t.Fatalf("expected two write attempts, got %d", calls)
		}
		if AuditWriteFailures.Get() <= before {
			t.Fatal("expected newline failure counter increment")
		}
	})
}

func TestLogger_DefaultNotNil(t *testing.T) {
	// Logger must never be nil — a nil dereference would panic.
	if Logger == nil {
		t.Fatal("observability.Logger is nil before InitLogger — would cause nil-pointer panic")
	}
	// Should not panic.
	Logger.Info("test log from default logger")
}

func TestHandleMetrics_ReturnsPrometheusFormat(t *testing.T) {
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	HandleMetrics(rr, req)

	if rr.Code != 200 {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "text/plain; version=0.0.4; charset=utf-8" {
		t.Errorf("unexpected Content-Type: %q", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "# HELP mcp_audit_write_failures_total") {
		t.Error("missing mcp_audit_write_failures_total in /metrics output")
	}
	if !strings.Contains(body, "# TYPE mcp_audit_write_failures_total counter") {
		t.Error("missing TYPE annotation in /metrics output")
	}
}

func TestHandleMetrics_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest("POST", "/metrics", nil)
	rr := httptest.NewRecorder()
	HandleMetrics(rr, req)
	if rr.Code != 405 {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}
