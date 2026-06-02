package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"mcp-runtime-go/internal/context"
	"net/http"
	"strings"
)

// RequestIDMiddleware extracts or generates a unique ID for each request.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateID()
		if clientID := strings.TrimSpace(r.Header.Get("X-Request-ID")); clientID != "" {
			ctx := context.WithClientRequestID(r.Context(), clientID)
			ctx = context.WithRequestID(ctx, id)
			w.Header().Set("X-Request-ID", id)
			w.Header().Set("X-Client-Request-ID", clientID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		ctx := context.WithRequestID(r.Context(), id)
		w.Header().Set("X-Request-ID", id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateID() string {
	b := make([]byte, 16) // 128-bit entropy
	if _, err := rand.Read(b); err != nil {
		// Fail closed/loud if crypto rand fails
		panic("failed to generate secure request id: " + err.Error())
	}
	return hex.EncodeToString(b)
}
