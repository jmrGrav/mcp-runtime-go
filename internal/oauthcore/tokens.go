package oauthcore

import (
	"crypto/sha256"
	"encoding/hex"
)

type TokenStore interface {
	Load() (map[string]float64, error)
	Save(map[string]float64) error
	Close() error
}

func HashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}
