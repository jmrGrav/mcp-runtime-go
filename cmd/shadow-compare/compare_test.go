package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadLog_Error(t *testing.T) {
	_, err := readLog("/non-existent/path")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestReadLog(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	content := `{"event": "token_issued", "request_id": "123"}
{"event": "authorize_approved", "request_id": "456"}
{"event": "proxy_error", "request_id": "789"}`

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := readLog(logPath)
	if err != nil {
		t.Fatalf("readLog failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestReadLog_InvalidJSONFails(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "bad.log")
	content := `{"event": "token_issued"}
invalid json`

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := readLog(logPath); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestCompareEntries(t *testing.T) {
	p := AuditEntry{
		"event": "token_issued",
		"uid":   "user1",
		"ts":    "2023-01-01T10:00:00Z",
	}
	g := AuditEntry{
		"event": "token_issued",
		"uid":   "user1",
		"ts":    "2023-01-01T10:00:05Z", // Ignored
	}

	diffs := compareEntries(p, g)
	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs, got %v", diffs)
	}

	g["uid"] = "user2"
	diffs = compareEntries(p, g)
	if len(diffs) != 1 {
		t.Errorf("expected 1 diff, got %v", diffs)
	}
}

func TestCompareEntries_MissingFields(t *testing.T) {
	p := AuditEntry{"a": 1, "b": 2}
	g := AuditEntry{"a": 1}
	diffs := compareEntries(p, g)
	found := false
	for _, d := range diffs {
		if strings.Contains(d, "missing field \"b\"") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing field diff, got %v", diffs)
	}
}

func TestCompareEntries_ExtraGoFieldsFail(t *testing.T) {
	p := AuditEntry{"event": "token_issued"}
	g := AuditEntry{"event": "token_issued", "extra": "unexpected"}
	diffs := compareEntries(p, g)
	found := false
	for _, d := range diffs {
		if strings.Contains(d, "extra field \"extra\"") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected extra field diff, got %v", diffs)
	}
}

func TestFindFallbackMatches(t *testing.T) {
	p := AuditEntry{
		"event":  "token_issued",
		"src_ip": "1.2.3.4",
		"ts":     "2023-01-01T10:00:00+0000",
	}
	goEntries := []AuditEntry{
		{
			"event":  "token_issued",
			"src_ip": "1.2.3.4",
			"ts":     "2023-01-01T10:00:01+0000",
		},
		{
			"event":  "token_issued",
			"src_ip": "1.2.3.4",
			"ts":     "2023-01-01T10:00:10+0000",
		},
	}

	matches := findFallbackMatches(p, goEntries, make([]bool, len(goEntries)), 2*time.Second)
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}

	matches = findFallbackMatches(p, goEntries, make([]bool, len(goEntries)), 11*time.Second)
	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}

	t.Run("Invalid timestamps", func(t *testing.T) {
		pInv := AuditEntry{"event": "e", "src_ip": "i", "ts": "invalid"}
		if matches := findFallbackMatches(pInv, goEntries, make([]bool, len(goEntries)), time.Second); matches != nil {
			t.Error("expected nil for invalid p timestamp")
		}

		gInv := []AuditEntry{{"event": "e", "src_ip": "i", "ts": "invalid"}}
		pValid := AuditEntry{"event": "e", "src_ip": "i", "ts": "2023-01-01T10:00:00+0000"}
		if matches := findFallbackMatches(pValid, gInv, make([]bool, len(gInv)), time.Second); len(matches) != 0 {
			t.Error("expected 0 matches for invalid g timestamp")
		}
	})
}

func TestRunCompare(t *testing.T) {
	tmpDir := t.TempDir()
	pLog := filepath.Join(tmpDir, "python.log")
	gLog := filepath.Join(tmpDir, "go.log")

	pContent := `{"event": "token_issued", "request_id": "rid1", "user": "alice", "ts": "2023-01-01T10:00:00+0000"}
{"event": "client_registered", "request_id": "rid2", "ts": "2023-01-01T10:00:05+0000"}`
	gContent := `{"event": "token_issued", "request_id": "rid1", "user": "alice", "ts": "2023-01-01T10:00:01+0000"}
{"event": "client_registered", "request_id": "rid2", "ts": "2023-01-01T10:00:06+0000"}`

	os.WriteFile(pLog, []byte(pContent), 0644)
	os.WriteFile(gLog, []byte(gContent), 0644)

	success, err := runCompare(pLog, gLog, false, 2*time.Second)
	if err != nil {
		t.Fatalf("runCompare failed: %v", err)
	}
	if !success {
		t.Error("expected success for matching entries")
	}

	t.Run("Go log read error", func(t *testing.T) {
		success, err := runCompare(pLog, "/non-existent", false, 2*time.Second)
		if err == nil || success {
			t.Error("expected error for missing Go log")
		}
	})

	// Test mismatch
	gContentMismatch := `{"event": "token_issued", "request_id": "rid1", "user": "bob", "ts": "2023-01-01T10:00:01+0000"}`
	os.WriteFile(gLog, []byte(gContentMismatch), 0644)
	success, _ = runCompare(pLog, gLog, false, 2*time.Second)
	if success {
		t.Error("expected failure for mismatching entries")
	}

	t.Run("Duplicate ID in Go", func(t *testing.T) {
		gContentDup := `{"event": "token_issued", "request_id": "rid1", "ts": "2023-01-01T10:00:01+0000"}
{"event": "token_issued", "request_id": "rid1", "ts": "2023-01-01T10:00:02+0000"}`
		os.WriteFile(gLog, []byte(gContentDup), 0644)
		success, _ = runCompare(pLog, gLog, false, 2*time.Second)
		if success {
			t.Error("expected failure for duplicate IDs in Go log")
		}
	})

	t.Run("Critical missing ID", func(t *testing.T) {
		pContentCrit := `{"event": "token_issued", "ts": "2023-01-01T10:00:00+0000"}`
		os.WriteFile(pLog, []byte(pContentCrit), 0644)
		success, _ = runCompare(pLog, gLog, false, 2*time.Second)
		if success {
			t.Error("expected failure for critical event missing ID")
		}
	})

	t.Run("Fallback match non-critical", func(t *testing.T) {
		// Non-critical events without request_id: missing ID is a WARN, not FAIL.
		// With fallback: entry matches via time+event+ip → pass.
		// Without fallback: unmatched non-critical entry → WARN, still pass.
		pContentNoID := `{"event": "proxy_hit", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:00+0000"}`
		gContentNoID := `{"event": "proxy_hit", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:01+0000"}`
		os.WriteFile(pLog, []byte(pContentNoID), 0644)
		os.WriteFile(gLog, []byte(gContentNoID), 0644)

		success, _ = runCompare(pLog, gLog, true, 2*time.Second)
		if !success {
			t.Error("expected pass for non-critical missing ID with fallback (should be WARN only)")
		}

		success, _ = runCompare(pLog, gLog, false, 2*time.Second)
		if !success {
			t.Error("expected pass for non-critical missing ID without fallback (should be WARN only)")
		}
	})

	t.Run("Fallback match critical fails", func(t *testing.T) {
		// Critical event without request_id must still FAIL.
		pCriticalNoID := `{"event": "token_issued", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:00+0000"}`
		gContent := `{"event": "token_issued", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:01+0000"}`
		os.WriteFile(pLog, []byte(pCriticalNoID), 0644)
		os.WriteFile(gLog, []byte(gContent), 0644)

		success, _ = runCompare(pLog, gLog, true, 2*time.Second)
		if success {
			t.Error("expected failure for critical event missing request_id")
		}
	})

	t.Run("Ambiguous fallback", func(t *testing.T) {
		pContentNoID := `{"event": "proxy_hit", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:00+0000"}`
		gContentAmb := `{"event": "proxy_hit", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:01+0000"}
{"event": "proxy_hit", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:00+0000"}`
		os.WriteFile(pLog, []byte(pContentNoID), 0644)
		os.WriteFile(gLog, []byte(gContentAmb), 0644)

		success, _ = runCompare(pLog, gLog, true, 2*time.Second)
		if success {
			t.Error("expected failure for ambiguous fallback")
		}
	})
}

func TestRunCompare_DuplicatePythonRequestIDFails(t *testing.T) {
	tmpDir := t.TempDir()
	pLog := filepath.Join(tmpDir, "python.log")
	gLog := filepath.Join(tmpDir, "go.log")

	pContent := `{"event": "token_issued", "request_id": "rid1", "user": "alice", "ts": "2023-01-01T10:00:00+0000"}
{"event": "token_issued", "request_id": "rid1", "user": "alice", "ts": "2023-01-01T10:00:01+0000"}`
	gContent := `{"event": "token_issued", "request_id": "rid1", "user": "alice", "ts": "2023-01-01T10:00:02+0000"}`

	os.WriteFile(pLog, []byte(pContent), 0644)
	os.WriteFile(gLog, []byte(gContent), 0644)

	success, _ := runCompare(pLog, gLog, false, 2*time.Second)
	if success {
		t.Fatal("expected duplicate Python request_id to fail")
	}
}

func TestRunCompare_DuplicatePythonFallbackNonCritical(t *testing.T) {
	// Two non-critical Python events, one Go match: the unmatched Python entry is a
	// WARN (not FAIL) because non-critical unmatched entries are expected in shadow mode
	// (e.g. mcp_forward in Python has no Go counterpart since tokens aren't shared).
	tmpDir := t.TempDir()
	pLog := filepath.Join(tmpDir, "python.log")
	gLog := filepath.Join(tmpDir, "go.log")

	pContent := `{"event": "proxy_hit", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:00+0000"}
{"event": "proxy_hit", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:00+0000"}`
	gContent := `{"event": "proxy_hit", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:01+0000"}`

	os.WriteFile(pLog, []byte(pContent), 0644)
	os.WriteFile(gLog, []byte(gContent), 0644)

	success, _ := runCompare(pLog, gLog, true, 2*time.Second)
	if !success {
		t.Fatal("expected non-critical unmatched Python entry to be WARN (pass), not FAIL")
	}
}

func TestRunCompare_DuplicatePythonFallbackCriticalFails(t *testing.T) {
	// Two critical Python events, one Go match: the unmatched critical Python entry must FAIL.
	tmpDir := t.TempDir()
	pLog := filepath.Join(tmpDir, "python.log")
	gLog := filepath.Join(tmpDir, "go.log")

	pContent := `{"event": "token_issued", "request_id": "aaa111", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:00+0000"}
{"event": "token_issued", "request_id": "bbb222", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:00+0000"}`
	gContent := `{"event": "token_issued", "request_id": "aaa111", "src_ip": "1.2.3.4", "ts": "2023-01-01T10:00:01+0000"}`

	os.WriteFile(pLog, []byte(pContent), 0644)
	os.WriteFile(gLog, []byte(gContent), 0644)

	success, _ := runCompare(pLog, gLog, false, 2*time.Second)
	if success {
		t.Fatal("expected unmatched critical Python token_issued to FAIL")
	}
}
