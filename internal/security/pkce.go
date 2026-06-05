package security

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

func ValidatePKCE(codeChallenge, codeVerifier string) bool {
	// PKCE S256: BASE64URL-ENCODE(SHA256(ASCII(code_verifier))) == code_challenge
	h := sha256.New()
	h.Write([]byte(codeVerifier))
	hashed := h.Sum(nil)

	// Use RawURLEncoding to avoid padding as per RFC 7636
	expected := base64.RawURLEncoding.EncodeToString(hashed)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(codeChallenge)) == 1
}
