package security

import (
	"crypto/rand"
	"encoding/base64"
	"io"
)

var cryptoRandReader = rand.Reader

// GenerateRandomString creates a secure random string of length n (bytes).
// The resulting string will be longer than n due to base64 encoding.
func GenerateRandomString(n int) string {
	b := make([]byte, n)
	if _, err := io.ReadFull(cryptoRandReader, b); err != nil {
		panic("failed to generate secure random string: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
