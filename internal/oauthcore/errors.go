package oauthcore

import (
	"net/http"
	"strings"
)

// MapAuthorizeError maps internal authorization errors to RFC 6749 §4.1.2.1
// error codes. The HTTP adapter decides whether to redirect or write directly.
func MapAuthorizeError(err error) (code, description string) {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "unsupported_response_type"):
		return "unsupported_response_type", ""
	case strings.HasPrefix(msg, "invalid_request"):
		return "invalid_request", strings.TrimPrefix(msg, "invalid_request: ")
	default:
		return "invalid_request", msg
	}
}

// MapTokenError maps internal token exchange errors to RFC 6749 §5.2 error
// codes and HTTP status values.
func MapTokenError(err error) (code string, status int) {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "unsupported_grant_type"):
		return "unsupported_grant_type", http.StatusBadRequest
	case strings.HasPrefix(msg, "invalid_client"):
		return "invalid_client", http.StatusUnauthorized
	case strings.HasPrefix(msg, "invalid_grant"):
		return "invalid_grant", http.StatusBadRequest
	case strings.HasPrefix(msg, "server_error"):
		return "server_error", http.StatusInternalServerError
	default:
		return "invalid_request", http.StatusBadRequest
	}
}
