.PHONY: build build-api build-cli run-api run-cli test lint clean setup help

# Build settings
BINARY_API=github-metrics-api
BINARY_CLI=github-metrics
BUILD_DIR=./bin

# Go settings
GOFLAGS=-ldflags="-s -w"

help:
	@echo "Available commands:"
	@echo "  make setup      - Install dependencies"
	@echo "  make build      - Build both API and CLI"
	@echo "  make build-api  - Build API server"
	@echo "  make build-cli  - Build CLI tool"
	@echo "  make run-api    - Run API server"
	@echo "  make run-cli    - Run CLI tool"
	@echo "  make test       - Run tests"
	@echo "  make lint       - Run linter"
	@echo "  make clean      - Clean build artifacts"

setup:
	go mod download
	go mod tidy

build: build-api build-cli

build-api:
	@mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_API) ./cmd/api

build-cli:
	@mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI) ./cmd/cli

run-api:
	go run ./cmd/api

run-cli:
	go run ./cmd/cli $(ARGS)

test:
	go test -v -race -cover ./...

lint:
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, running go vet instead"; \
		go vet ./...; \
	fi

clean:
	rm -rf $(BUILD_DIR)
	rm -f metrics.db

# Development helpers
dev-api:
	go run ./cmd/api

dev-collect:
	go run ./cmd/cli collect $(ORG)

dev-show:
	go run ./cmd/cli show $(ORG)
