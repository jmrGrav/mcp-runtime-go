package observability

import (
	"encoding/json"
	"log/slog"
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
