package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements the Store interface using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore initializes a new SQLiteStore with the given path.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	// Apply WAL and other PRAGMAs according to the blueprint.
	// busy_timeout is set to 5000ms.
	// journal_size_limit is set to 10MB.
	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA busy_timeout = 5000;
		PRAGMA journal_size_limit = 10485760;
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply pragmas: %w", err)
	}

	// Tun Go connection pool: single writer for single-node robustness.
	db.SetMaxOpenConns(1)

	// Initialize schema according to the blueprint.
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS access_tokens (
			token_hash TEXT PRIMARY KEY,
			expires_at INTEGER NOT NULL
		) WITHOUT ROWID;
		CREATE INDEX IF NOT EXISTS idx_access_tokens_expiry ON access_tokens(expires_at);

		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Load retrieves all valid tokens from the SQLite database.
func (s *SQLiteStore) Load() (map[string]float64, error) {
	rows, err := s.db.Query("SELECT token_hash, expires_at FROM access_tokens WHERE expires_at > ?", time.Now().Unix())
	if err != nil {
		return nil, fmt.Errorf("failed to query tokens: %w", err)
	}
	defer rows.Close()

	tokens := make(map[string]float64)
	for rows.Next() {
		var hash string
		var expiry int64
		if err := rows.Scan(&hash, &expiry); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		tokens[hash] = float64(expiry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return tokens, nil
}

// Save replaces the current token set with the provided one.
func (s *SQLiteStore) Save(tokens map[string]float64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing tokens to match the behavior of the JSON store snapshotting.
	if _, err := tx.Exec("DELETE FROM access_tokens"); err != nil {
		return fmt.Errorf("failed to clear tokens: %w", err)
	}

	stmt, err := tx.Prepare("INSERT INTO access_tokens (token_hash, expires_at) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for hash, expiry := range tokens {
		if int64(expiry) > now {
			if _, err := stmt.Exec(hash, int64(expiry)); err != nil {
				return fmt.Errorf("failed to insert token: %w", err)
			}
		}
	}

	return tx.Commit()
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Purge manually removes expired tokens.
func (s *SQLiteStore) Purge() error {
	_, err := s.db.Exec("DELETE FROM access_tokens WHERE expires_at < ?", time.Now().Unix())
	return err
}
