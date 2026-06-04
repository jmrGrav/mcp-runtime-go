package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TokenStore struct {
	mu           sync.RWMutex
	filePath     string
	allowRecover bool
}

func NewTokenStore(path string, allowRecover bool) *TokenStore {
	return &TokenStore{
		filePath:     path,
		allowRecover: allowRecover,
	}
}

func (s *TokenStore) Load() (map[string]float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return make(map[string]float64), nil
	}

	// Ensure permissions on existing file
	if err := os.Chmod(s.filePath, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] failed to set 0600 on %s: %v\n", s.filePath, err)
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	if len(data) == 0 {
		return make(map[string]float64), nil
	}

	var tokens map[string]float64
	if err := json.Unmarshal(data, &tokens); err != nil {
		if s.allowRecover {
			backupPath := fmt.Sprintf("%s.bak.%d", s.filePath, time.Now().Unix())
			if err := os.Rename(s.filePath, backupPath); err != nil {
				return nil, fmt.Errorf("failed to back up corrupted token file: %w", err)
			}
			fmt.Fprintf(os.Stderr, "[ERROR] token store corrupted, backed up to %s and starting fresh\n", backupPath)
			return make(map[string]float64), nil
		}
		return nil, fmt.Errorf("failed to unmarshal tokens (corruption detected): %w", err)
	}

	now := float64(time.Now().Unix())
	validTokens := make(map[string]float64)
	for k, v := range tokens {
		if v > now {
			validTokens[k] = v
		}
	}

	return validTokens, nil
}

func (s *TokenStore) Save(tokens map[string]float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := float64(time.Now().Unix())
	validTokens := make(map[string]float64)
	for k, v := range tokens {
		if v > now {
			validTokens[k] = v
		}
	}

	data, err := json.MarshalIndent(validTokens, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tmpFile := s.filePath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}

	// Critical: fsync the file before closing
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("failed to fsync temporary file: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	// Critical: fsync the parent directory to ensure rename is durable
	df, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("failed to open parent directory for fsync: %w", err)
	}
	defer df.Close()
	if err := df.Sync(); err != nil {
		return fmt.Errorf("failed to fsync parent directory: %w", err)
	}

	return nil
}

// Close is a no-op for the JSON TokenStore.
func (s *TokenStore) Close() error {
	return nil
}
