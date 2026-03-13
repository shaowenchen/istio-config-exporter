.PHONY: build clean test run

BINARY_NAME=istio-config-exporter
VERSION?=0.1.0
BUILD_DIR=build

build:
	@echo "Building $(BINARY_NAME)..."
	@if [ -d "vendor" ]; then \
		go build -mod=vendor -o $(BUILD_DIR)/$(BINARY_NAME) .; \
	else \
		go build -o $(BUILD_DIR)/$(BINARY_NAME) .; \
	fi
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean
	@echo "Clean complete"

test:
	@echo "Running tests..."
	@go test -v ./...

run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

install:
	@echo "Installing $(BINARY_NAME)..."
	@go install .
	@echo "Install complete"

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@go mod vendor
	@echo "Dependencies updated and vendor directory created"

vendor:
	@echo "Creating vendor directory..."
	@go mod vendor
	@echo "Vendor directory created"

.DEFAULT_GOAL := build
