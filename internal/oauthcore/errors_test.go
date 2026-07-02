package oauthcore

import (
	"errors"
	"net/http"
	"testing"
)

func TestMapAuthorizeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		wantDesc string
	}{
		{"unsupported response type", errors.New("unsupported_response_type"), "unsupported_response_type", ""},
		{"invalid request keeps detail", errors.New("invalid_request: missing state parameter"), "invalid_request", "missing state parameter"},
		{"unknown error becomes invalid request", errors.New("unexpected"), "invalid_request", "unexpected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, desc := MapAuthorizeError(tt.err)
			if code != tt.wantCode || desc != tt.wantDesc {
				t.Fatalf("MapAuthorizeError() = (%q, %q), want (%q, %q)", code, desc, tt.wantCode, tt.wantDesc)
			}
		})
	}
}

func TestMapTokenError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
	}{
		{"unsupported grant", errors.New("unsupported_grant_type"), "unsupported_grant_type", http.StatusBadRequest},
		{"invalid client", errors.New("invalid_client"), "invalid_client", http.StatusUnauthorized},
		{"invalid grant", errors.New("invalid_grant: bad code"), "invalid_grant", http.StatusBadRequest},
		{"server error", errors.New("server_error"), "server_error", http.StatusInternalServerError},
		{"unknown error", errors.New("unexpected"), "invalid_request", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, status := MapTokenError(tt.err)
			if code != tt.wantCode || status != tt.wantStatus {
				t.Fatalf("MapTokenError() = (%q, %d), want (%q, %d)", code, status, tt.wantCode, tt.wantStatus)
			}
		})
	}
}
