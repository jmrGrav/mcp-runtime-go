BIN_DIR   := bin
RUNTIME   := $(BIN_DIR)/mcp-runtime
COMPARE   := $(BIN_DIR)/shadow-compare
COVER_OUT := coverage.out

.PHONY: all build test race vet coverage clean

all: vet test build

build: $(RUNTIME) $(COMPARE)

$(RUNTIME):
	go build -o $(RUNTIME) ./cmd/mcp-runtime

$(COMPARE):
	go build -o $(COMPARE) ./cmd/shadow-compare

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
	rm -f $(RUNTIME) $(COMPARE) $(COVER_OUT) coverage_security.out
