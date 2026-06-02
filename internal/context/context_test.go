package context

import (
	"context"
	"testing"
)

func TestRequestID(t *testing.T) {
	ctx := context.Background()
	
	// Test default
	if id := GetRequestID(ctx); id != "missing" {
		t.Errorf("expected 'missing' for missing request id, got %s", id)
	}

	// Test with ID
	ctxWithID := WithRequestID(ctx, "test-id")
	if id := GetRequestID(ctxWithID); id != "test-id" {
		t.Errorf("expected test-id, got %s", id)
	}

	// Test original context remains unchanged
	if id := GetRequestID(ctx); id != "missing" {
		t.Errorf("original context should have 'missing' request id, got %s", id)
	}
}
