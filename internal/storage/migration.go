package storage

import (
	"fmt"
	"os"
)

// MigrateJSONToSQLite performs a one-way migration of tokens from JSON to SQLite.
// It follows the atomic sequence:
// 1. Load tokens from JSON.
// 2. Open SQLite and verify it is empty.
// 3. Save tokens to SQLite in a single transaction.
// 4. Force a WAL checkpoint (TRUNCATE).
// 5. Close SQLite.
// 6. Rename tokens.json to tokens.json.migrated.
func MigrateJSONToSQLite(jsonPath, sqlitePath string) error {
	// Verify source exists
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return fmt.Errorf("source tokens.json not found at %s", jsonPath)
	}

	// Load JSON tokens
	jsonStore := NewTokenStore(jsonPath, false)
	tokens, err := jsonStore.Load()
	if err != nil {
		return fmt.Errorf("failed to load tokens from JSON: %w", err)
	}

	// Create/Open SQLite
	sqliteStore, err := NewSQLiteStore(sqlitePath)
	if err != nil {
		return fmt.Errorf("failed to open SQLite at %s: %w", sqlitePath, err)
	}
	defer sqliteStore.Close()

	// Verify SQLite is empty (to prevent accidental overwrites)
	existing, err := sqliteStore.Load()
	if err != nil {
		return fmt.Errorf("failed to check existing SQLite tokens: %w", err)
	}
	if len(existing) > 0 {
		return fmt.Errorf("SQLite store is not empty, migration aborted to prevent data loss")
	}

	// Save to SQLite (transactional)
	if err := sqliteStore.Save(tokens); err != nil {
		return fmt.Errorf("failed to save tokens to SQLite: %w", err)
	}

	// Force WAL checkpoint to ensure all data is in the main database file
	if _, err := sqliteStore.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("failed to checkpoint SQLite WAL: %w", err)
	}

	// Close SQLite cleanly before proceeding with rename
	if err := sqliteStore.Close(); err != nil {
		return fmt.Errorf("failed to close SQLite: %w", err)
	}

	// Atomic Rename of the source file to mark migration as complete
	backupPath := jsonPath + ".migrated"
	if err := os.Rename(jsonPath, backupPath); err != nil {
		return fmt.Errorf("failed to rename migrated JSON file to %s: %w", backupPath, err)
	}

	return nil
}
