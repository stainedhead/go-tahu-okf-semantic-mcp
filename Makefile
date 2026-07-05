BINARY     := tahu
MODULE     := github.com/stainedhead/go-tahu-okf-semantic-mcp
BUILD_DIR  := bin
CMD_DIR    := cmd/tahu

.PHONY: all build test test-race test-integration lint fmt vet clean

all: build

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./$(CMD_DIR)

test:
	go test ./...

test-race:
	go test -race ./...

test-integration:
	go test -tags integration ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR) coverage.out

# Run the MCP server in stdio mode (default for CLI agents)
run:
	go run ./$(CMD_DIR) serve --transport stdio

# Run the MCP server in HTTP mode (for orchestration agents)
run-http:
	go run ./$(CMD_DIR) serve --transport http --port 3000
