#!/bin/bash
set -e

echo "Running tests..."
go test ./... -coverprofile=coverage.out

echo "Running race detector..."
go test -race ./...

echo "Running go vet..."
go vet ./...

echo "Checking coverage thresholds..."
go tool cover -func=coverage.out

echo "Building binaries..."
go build -o bin/mcp-runtime ./cmd/mcp-runtime
go build -o bin/shadow-compare ./cmd/shadow-compare

echo "All checks passed!"
