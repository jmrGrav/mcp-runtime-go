package storage

// Store defines the interface for token persistence.
type Store interface {
	// Load retrieves all valid tokens from the store.
	Load() (map[string]float64, error)
	// Save persists the provided token map.
	Save(tokens map[string]float64) error
	// Close releases any resources held by the store.
	Close() error
}
