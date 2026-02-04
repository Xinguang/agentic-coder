.PHONY: build run test clean install

BINARY_NAME=agentic
BUILD_DIR=./build
CMD_DIR=./cmd/agentic

# Version info
VERSION ?= 0.1.0
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Run
run: build
	@$(BUILD_DIR)/$(BINARY_NAME)

# Test
test:
	go test -v ./...

# Test with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean
clean:
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Install to GOPATH/bin
install: build
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

# Lint
lint:
	golangci-lint run ./...

# Format
fmt:
	go fmt ./...

# Tidy dependencies
tidy:
	go mod tidy

# Generate (for future code generation)
generate:
	go generate ./...

# Development build (with race detector)
dev:
	go build -race $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  run            - Build and run"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  clean          - Clean build artifacts"
	@echo "  install        - Install to GOPATH/bin"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  tidy           - Tidy dependencies"
	@echo "  dev            - Development build with race detector"
