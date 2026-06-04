package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrateJSONToSQLite(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "tokens.json")
	sqlitePath := filepath.Join(tmpDir, "tokens.db")

	// 1. Prepare JSON data
	jsonStore := NewTokenStore(jsonPath, false)
	tokens := map[string]float64{
		"token1": float64(time.Now().Add(1 * time.Hour).Unix()),
		"token2": float64(time.Now().Add(2 * time.Hour).Unix()),
	}
	if err := jsonStore.Save(tokens); err != nil {
		t.Fatalf("Failed to save JSON tokens: %v", err)
	}

	// 2. Run migration
	if err := MigrateJSONToSQLite(jsonPath, sqlitePath); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// 3. Verify SQLite data
	sqliteStore, err := NewSQLiteStore(sqlitePath)
	if err != nil {
		t.Fatalf("Failed to open SQLite after migration: %v", err)
	}
	defer sqliteStore.Close()

	loaded, err := sqliteStore.Load()
	if err != nil {
		t.Fatalf("Failed to load tokens from SQLite: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("Expected 2 tokens in SQLite, got %d", len(loaded))
	}
	if _, ok := loaded["token1"]; !ok {
		t.Error("token1 missing in SQLite")
	}

	// 4. Verify JSON file was renamed
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("Original tokens.json should not exist after migration")
	}
	if _, err := os.Stat(jsonPath + ".migrated"); os.IsNotExist(err) {
		t.Error("tokens.json.migrated should exist after migration")
	}
}

func TestMigrateJSONToSQLite_NotEmptyAbort(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "tokens.json")
	sqlitePath := filepath.Join(tmpDir, "tokens.db")

	// Prepare JSON
	os.WriteFile(jsonPath, []byte("{}"), 0600)

	// Prepare SQLite with data
	sqliteStore, _ := NewSQLiteStore(sqlitePath)
	sqliteStore.Save(map[string]float64{"existing": float64(time.Now().Add(1 * time.Hour).Unix())})
	sqliteStore.Close()

	// Run migration
	err := MigrateJSONToSQLite(jsonPath, sqlitePath)
	if err == nil {
		t.Error("Expected migration to fail because SQLite is not empty")
	}
    
	// Verify JSON still exists
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Error("tokens.json should still exist after aborted migration")
	}
}
