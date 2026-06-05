package storage

import (
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

func TestSQLiteStore_Purge(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tokens.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("Failed to create SQLiteStore: %v", err)
	}
	defer store.Close()

	// Insert tokens manually to bypass Save filtering
	tx, _ := store.db.Begin()
	now := time.Now().Unix()
	_, _ = tx.Exec("INSERT INTO access_tokens (token_hash, expires_at) VALUES (?, ?)", "valid", now+3600)
	_, _ = tx.Exec("INSERT INTO access_tokens (token_hash, expires_at) VALUES (?, ?)", "expired", now-3600)
	tx.Commit()

	if err := store.Purge(); err != nil {
		t.Fatalf("Purge failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if _, ok := loaded["valid"]; !ok {
		t.Error("valid token missing after purge")
	}
	if _, ok := loaded["expired"]; ok {
		t.Error("expired token still present after purge")
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
