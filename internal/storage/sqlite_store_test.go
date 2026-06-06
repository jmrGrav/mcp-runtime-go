package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSQLiteStore_LoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("Failed to create SQLiteStore: %v", err)
	}
	defer store.Close()

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

func TestSQLiteStore_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("Failed to create SQLiteStore: %v", err)
	}
	defer store.Close()

	const iterations = 50
	const goroutines = 5
	var wg sync.WaitGroup

	// Readers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = store.Load()
			}
		}()
	}

	// Writers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = store.Save(map[string]float64{
					fmt.Sprintf("token-%d-%d", id, j): float64(time.Now().Add(1 * time.Hour).Unix()),
				})
			}
		}(i)
	}

	wg.Wait()
}

func TestSQLiteStore_InterfaceCompliance(t *testing.T) {
	var _ Store = (*SQLiteStore)(nil)
}

func TestNewSQLiteStore_PragmaError(t *testing.T) {
	orig := applySQLitePragmasFn
	t.Cleanup(func() { applySQLitePragmasFn = orig })
	applySQLitePragmasFn = func(*sql.DB) error {
		return fmt.Errorf("pragma fail")
	}

	if _, err := NewSQLiteStore(":memory:"); err == nil {
		t.Fatal("expected pragma error")
	}
}

func TestNewSQLiteStore_SchemaError(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := initSQLiteSchemaFn(db); err == nil {
		t.Fatal("expected schema error from closed DB")
	}
}

func TestNewSQLiteStore_SchemaError_Branch(t *testing.T) {
	orig := initSQLiteSchemaFn
	t.Cleanup(func() { initSQLiteSchemaFn = orig })
	initSQLiteSchemaFn = func(*sql.DB) error {
		return fmt.Errorf("schema fail")
	}

	if _, err := NewSQLiteStore(":memory:"); err == nil {
		t.Fatal("expected schema error branch from NewSQLiteStore")
	}
}

func TestSQLiteStore_Load_Error(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, _ := NewSQLiteStore(path)

	// Drop table to cause query error
	store.db.Exec("DROP TABLE access_tokens")

	_, err := store.Load()
	if err == nil {
		t.Error("expected error from Load after dropping table")
	}
}

func TestSQLiteStore_Load_ScanError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_, err = store.db.Exec("DROP TABLE access_tokens; CREATE TABLE access_tokens(token_hash TEXT, expires_at INTEGER); INSERT INTO access_tokens(token_hash, expires_at) VALUES (NULL, ?)", time.Now().Add(time.Hour).Unix())
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Load()
	if err == nil {
		t.Fatal("expected scan error from NULL token_hash")
	}
}

func TestSQLiteStore_Load_RowsError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Save(map[string]float64{"token": float64(time.Now().Add(time.Hour).Unix())}); err != nil {
		t.Fatal(err)
	}

	orig := loadRowsErrFn
	t.Cleanup(func() { loadRowsErrFn = orig })
	loadRowsErrFn = func(*sql.Rows) error {
		return fmt.Errorf("rows fail")
	}

	if _, err := store.Load(); err == nil {
		t.Fatal("expected rows error")
	}
}

func TestSQLiteStore_Save_TransactionError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, _ := NewSQLiteStore(path)

	// Close DB to cause transaction error
	store.db.Close()

	err := store.Save(map[string]float64{"t": 123})
	if err == nil {
		t.Error("expected error from Save after closing DB")
	}
}

func TestSQLiteStore_Save_ClearError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, _ := NewSQLiteStore(path)
	defer store.Close()

	if err := store.Save(map[string]float64{"existing": float64(time.Now().Add(time.Hour).Unix())}); err != nil {
		t.Fatal(err)
	}

	_, err := store.db.Exec(`CREATE TRIGGER deny_delete BEFORE DELETE ON access_tokens BEGIN SELECT RAISE(ABORT, 'no delete'); END;`)
	if err != nil {
		t.Fatal(err)
	}

	err = store.Save(map[string]float64{"t": float64(time.Now().Add(time.Hour).Unix())})
	if err == nil {
		t.Fatal("expected delete/clear error")
	}
}

func TestSQLiteStore_Save_DeleteError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, _ := NewSQLiteStore(path)

	// Drop table within transaction? No, Save starts its own.
	// We can drop it before Save.
	store.db.Exec("DROP TABLE access_tokens")

	err := store.Save(map[string]float64{"t": 123})
	if err == nil {
		t.Error("expected error from Save after dropping table")
	}
}

func TestSQLiteStore_Save_PrepareError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, _ := NewSQLiteStore(path)
	defer store.Close()

	_, err := store.db.Exec("DROP TABLE access_tokens; CREATE TABLE access_tokens(token_hash TEXT PRIMARY KEY)")
	if err != nil {
		t.Fatal(err)
	}

	err = store.Save(map[string]float64{"t": float64(time.Now().Add(time.Hour).Unix())})
	if err == nil {
		t.Error("expected prepare error")
	}
}

func TestSQLiteStore_Save_InsertError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, _ := NewSQLiteStore(path)
	defer store.Close()

	_, err := store.db.Exec(`CREATE TRIGGER deny_insert BEFORE INSERT ON access_tokens BEGIN SELECT RAISE(ABORT, 'no insert'); END;`)
	if err != nil {
		t.Fatal(err)
	}

	err = store.Save(map[string]float64{"t": float64(time.Now().Add(time.Hour).Unix())})
	if err == nil {
		t.Fatal("expected insert error")
	}
}
