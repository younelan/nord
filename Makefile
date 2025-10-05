# Variables
BINARY_NAME=observer
BUILD_DIR=bin
SOURCE_DIR=.
GO_FILES=$(shell find . -name "*.go" -type f)

# Default target
all: build

# Create build directory
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Build the binary
build: $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(SOURCE_DIR)

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)

# Run the application
run:
	go run $(SOURCE_DIR)

# Build for multiple platforms
build-all: $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(SOURCE_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(SOURCE_DIR)
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(SOURCE_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(SOURCE_DIR)

# Install dependencies
deps:
	go mod tidy

# Run tests
test:
	go test ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  build     - Build the binary"
	@echo "  build-all - Build for multiple platforms"
	@echo "  run       - Run the application"
	@echo "  clean     - Clean build artifacts"
	@echo "  deps      - Install dependencies"
	@echo "  test      - Run tests"
	@echo "  help      - Show this help"

.PHONY: all build build-all run clean deps test help
