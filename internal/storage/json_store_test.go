package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenStore_LoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.json")
	store := NewTokenStore(path, false)

	tokens := map[string]float64{
		"token1": float64(time.Now().Add(1 * time.Hour).Unix()),
		"token2": float64(time.Now().Add(-1 * time.Hour).Unix()), // Expired
	}

	if err := store.Save(tokens); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if _, ok := loaded["token1"]; !ok {
		t.Error("token1 missing")
	}
	if _, ok := loaded["token2"]; ok {
		t.Error("token2 should have been filtered out (expired)")
	}
}

func TestTokenStore_CorruptionRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.json")

	t.Run("Recovery disabled", func(t *testing.T) {
		if err := os.WriteFile(path, []byte("invalid json"), 0600); err != nil {
			t.Fatal(err)
		}
		store := NewTokenStore(path, false)
		_, err := store.Load()
		if err == nil {
			t.Error("expected error for corrupted file when recovery is disabled")
		}
	})

	t.Run("Recovery enabled", func(t *testing.T) {
		if err := os.WriteFile(path, []byte("invalid json"), 0600); err != nil {
			t.Fatal(err)
		}
		store := NewTokenStore(path, true)
		loaded, err := store.Load()
		if err != nil {
			t.Fatalf("Load failed with recovery enabled: %v", err)
		}
		if len(loaded) != 0 {
			t.Errorf("expected empty map on recovery, got %d items", len(loaded))
		}
		
		// Verify backup exists
		files, _ := os.ReadDir(tmpDir)
		foundBackup := false
		for _, f := range files {
			if len(f.Name()) > len("tokens.json") && f.Name()[:len("tokens.json")] == "tokens.json" {
				if f.Name() != "tokens.json" && f.Name() != "tokens.json.tmp" {
					foundBackup = true
				}
			}
		}
		if !foundBackup {
			t.Error("backup file not found after recovery")
		}
	})
}

func TestTokenStore_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	store := NewTokenStore(path, false)
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed for empty file: %v", err)
	}
	if len(loaded) != 0 {
		t.Error("expected empty map for empty file")
	}
}

func TestTokenStore_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "missing.json")
	store := NewTokenStore(path, false)
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed for non-existent file: %v", err)
	}
	if len(loaded) != 0 {
		t.Error("expected empty map for non-existent file")
	}
}

func TestTokenStore_SaveError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "some-dir")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	
	store := NewTokenStore(filepath.Join(path, "tokens.json"), false)
	
	// Make directory read-only to cause Save to fail
	os.Chmod(path, 0555)
	defer os.Chmod(path, 0755)

	err := store.Save(map[string]float64{"t": 123})
	if err == nil {
		t.Error("expected error when saving to read-only directory")
	}
}

func TestTokenStore_Load_UnmarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.json")
	if err := os.WriteFile(path, []byte("{invalid}"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Run("Recovery disabled", func(t *testing.T) {
		store := NewTokenStore(path, false)
		_, err := store.Load()
		if err == nil {
			t.Error("expected unmarshal error")
		}
	})
}

func TestTokenStore_Save_MkdirError(t *testing.T) {
	tmpDir := t.TempDir()
	parentFile := filepath.Join(tmpDir, "parent")
	if err := os.WriteFile(parentFile, []byte("im a file"), 0644); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(parentFile, "tokens.json")
	store := NewTokenStore(path, false)
	err := store.Save(map[string]float64{"t": 123})
	if err == nil {
		t.Error("expected mkdir error")
	}
}

func TestTokenStore_LoadPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.json")
	if err := os.WriteFile(path, []byte("{}"), 0777); err != nil {
		t.Fatal(err)
	}

	store := NewTokenStore(path, false)
	_, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %v", info.Mode().Perm())
	}
}

func TestTokenStore_Load_StatError(t *testing.T) {
	tmpDir := t.TempDir()
	subdir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subdir, 0755)
	path := filepath.Join(subdir, "tokens.json")
	os.WriteFile(path, []byte("{}"), 0600)
	
	store := NewTokenStore(path, false)
	
	// Make subdir unsearchable
	os.Chmod(subdir, 0000)
	defer os.Chmod(subdir, 0755)
	
	_, err := store.Load()
	if err == nil {
		t.Error("expected stat error")
	}
}

func TestTokenStore_Save_ParentSyncError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "tokens.json")
	store := NewTokenStore(path, false)
	
	if err := store.Save(map[string]float64{"t": 123}); err != nil {
		t.Fatal(err)
	}
	
	os.Chmod(filepath.Dir(path), 0000)
	defer os.Chmod(filepath.Dir(path), 0755)
	
	err := store.Save(map[string]float64{"t": 123})
	if err == nil {
		t.Error("expected error when parent directory is not openable")
	}
}
