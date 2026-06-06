package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeMigrationStore struct {
	tokens        map[string]float64
	loadErr       error
	saveErr       error
	checkpointErr error
	closeErr      error
}

func (f *fakeMigrationStore) Load() (map[string]float64, error) {
	return f.tokens, f.loadErr
}

func (f *fakeMigrationStore) Save(tokens map[string]float64) error {
	f.tokens = tokens
	return f.saveErr
}

func (f *fakeMigrationStore) Checkpoint() error {
	return f.checkpointErr
}

func (f *fakeMigrationStore) Close() error {
	return f.closeErr
}

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

func TestMigrateJSONToSQLite_SaveError(t *testing.T) {
	origFactory := newSQLiteStoreFn
	t.Cleanup(func() { newSQLiteStoreFn = origFactory })

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "tokens.json")
	sqlitePath := filepath.Join(tmpDir, "tokens.db")
	if err := os.WriteFile(jsonPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	newSQLiteStoreFn = func(string) (sqliteMigrationStore, error) {
		return &fakeMigrationStore{saveErr: os.ErrPermission}, nil
	}

	if err := MigrateJSONToSQLite(jsonPath, sqlitePath); err == nil {
		t.Fatal("expected save error")
	}
}

func TestMigrateJSONToSQLite_CheckpointError(t *testing.T) {
	origFactory := newSQLiteStoreFn
	t.Cleanup(func() { newSQLiteStoreFn = origFactory })

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "tokens.json")
	sqlitePath := filepath.Join(tmpDir, "tokens.db")
	if err := os.WriteFile(jsonPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	newSQLiteStoreFn = func(string) (sqliteMigrationStore, error) {
		return &fakeMigrationStore{checkpointErr: os.ErrInvalid}, nil
	}

	if err := MigrateJSONToSQLite(jsonPath, sqlitePath); err == nil {
		t.Fatal("expected checkpoint error")
	}
}

func TestMigrateJSONToSQLite_LoadError_Store(t *testing.T) {
	origFactory := newSQLiteStoreFn
	t.Cleanup(func() { newSQLiteStoreFn = origFactory })

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "tokens.json")
	sqlitePath := filepath.Join(tmpDir, "tokens.db")
	if err := os.WriteFile(jsonPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	newSQLiteStoreFn = func(string) (sqliteMigrationStore, error) {
		return &fakeMigrationStore{loadErr: os.ErrInvalid}, nil
	}

	if err := MigrateJSONToSQLite(jsonPath, sqlitePath); err == nil {
		t.Fatal("expected store load error")
	}
}

func TestMigrateJSONToSQLite_CloseError(t *testing.T) {
	origFactory := newSQLiteStoreFn
	t.Cleanup(func() { newSQLiteStoreFn = origFactory })

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "tokens.json")
	sqlitePath := filepath.Join(tmpDir, "tokens.db")
	if err := os.WriteFile(jsonPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	newSQLiteStoreFn = func(string) (sqliteMigrationStore, error) {
		return &fakeMigrationStore{closeErr: os.ErrClosed}, nil
	}

	if err := MigrateJSONToSQLite(jsonPath, sqlitePath); err == nil {
		t.Fatal("expected close error")
	}
}

func TestMigrateJSONToSQLite_RenameError(t *testing.T) {
	origFactory := newSQLiteStoreFn
	t.Cleanup(func() { newSQLiteStoreFn = origFactory })

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "tokens.json")
	sqlitePath := filepath.Join(tmpDir, "tokens.db")
	if err := os.WriteFile(jsonPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath+".migrated", []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(jsonPath + ".migrated"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(jsonPath+".migrated", 0755); err != nil {
		t.Fatal(err)
	}

	newSQLiteStoreFn = func(string) (sqliteMigrationStore, error) {
		return &fakeMigrationStore{}, nil
	}

	if err := MigrateJSONToSQLite(jsonPath, sqlitePath); err == nil {
		t.Fatal("expected rename error")
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

func TestMigrateJSONToSQLite_SourceMissing(t *testing.T) {
	err := MigrateJSONToSQLite("/nonexistent/path/json", "/tmp/any.db")
	if err == nil {
		t.Error("expected error for missing source file")
	}
}

func TestMigrateJSONToSQLite_LoadError(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "tokens.json")
	os.WriteFile(jsonPath, []byte("invalid json"), 0600)

	err := MigrateJSONToSQLite(jsonPath, filepath.Join(tmpDir, "any.db"))
	if err == nil {
		t.Error("expected error when JSON load fails")
	}
}

func TestMigrateJSONToSQLite_OpenSQLiteError(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "tokens.json")
	os.WriteFile(jsonPath, []byte("{}"), 0600)

	err := MigrateJSONToSQLite(jsonPath, "/nonexistent/directory/any.db")
	if err == nil {
		t.Error("expected error when SQLite open fails")
	}
}
