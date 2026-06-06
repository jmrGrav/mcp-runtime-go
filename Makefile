BIN_DIR   := bin
RUNTIME   := $(BIN_DIR)/mcp-runtime
COVER_OUT := coverage.out

.PHONY: all build test race vet coverage clean

all: vet test build

build: $(RUNTIME)

$(RUNTIME):
	go build -o $(RUNTIME) ./cmd/mcp-runtime

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

coverage:
	go test -coverprofile=$(COVER_OUT) ./...
	go tool cover -func=$(COVER_OUT)

clean:
	rm -f $(RUNTIME) $(COVER_OUT) coverage_security.out
