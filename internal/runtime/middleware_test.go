package runtime

import (
	"mcp-runtime-go/internal/context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware(t *testing.T) {
	t.Run("Client Request ID is not canonical", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Request-ID", "test-id")
		rr := httptest.NewRecorder()

		var capturedID, capturedClientID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = context.GetRequestID(r.Context())
			capturedClientID = context.GetClientRequestID(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		handler := RequestIDMiddleware(next)
		handler.ServeHTTP(rr, req)

		if capturedID == "test-id" || capturedID == "" {
			t.Errorf("expected generated canonical request id, got %s", capturedID)
		}
		if capturedClientID != "test-id" {
			t.Errorf("expected client request id test-id, got %s", capturedClientID)
		}
		if rr.Header().Get("X-Request-ID") != capturedID {
			t.Errorf("expected response header %s, got %s", capturedID, rr.Header().Get("X-Request-ID"))
		}
		if rr.Header().Get("X-Client-Request-ID") != "test-id" {
			t.Errorf("expected client request header test-id, got %s", rr.Header().Get("X-Client-Request-ID"))
		}
	})

	t.Run("Generated Request ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		var capturedID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = context.GetRequestID(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		handler := RequestIDMiddleware(next)
		handler.ServeHTTP(rr, req)

		if capturedID == "" {
			t.Error("expected generated request id, got empty")
		}
		if rr.Header().Get("X-Request-ID") != capturedID {
			t.Errorf("response header %s does not match captured id %s", rr.Header().Get("X-Request-ID"), capturedID)
		}
		if len(capturedID) != 32 { // hex encoded 16 bytes
			t.Errorf("expected 32 chars, got %d", len(capturedID))
		}
	})
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == id2 {
		t.Error("generateID produced duplicate IDs")
	}
	if len(id1) != 32 {
		t.Errorf("expected length 32, got %d", len(id1))
	}
}
