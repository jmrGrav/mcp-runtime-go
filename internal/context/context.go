package context

import (
	"context"
)

type contextKey string

const RequestIDKey contextKey = "request_id"
const ClientRequestIDKey contextKey = "client_request_id"

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

func WithClientRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ClientRequestIDKey, id)
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return "missing"
}

func GetClientRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(ClientRequestIDKey).(string); ok {
		return id
	}
	return ""
}
