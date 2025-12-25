.PHONY: build run test clean deps

APP_NAME=backup-server
BUILD_DIR=./bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GORUN=$(GOCMD) run

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server

# Run the application
run:
	$(GORUN) ./cmd/server

# Run tests
test:
	$(GOTEST) -v ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)

# Build for Linux
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 ./cmd/server

# Docker build
docker-build:
	docker build -t $(APP_NAME) .

# Create storage directory
init-storage:
	@mkdir -p /storage
	@chmod 755 /storage

# Database migration (uses GORM auto-migrate on startup)
migrate:
	@echo "Database migration happens automatically on server start"

# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build the application"
	@echo "  run          - Run the application"
	@echo "  test         - Run tests"
	@echo "  deps         - Download dependencies"
	@echo "  clean        - Clean build artifacts"
	@echo "  build-linux  - Build for Linux"
	@echo "  docker-build - Build Docker image"
	@echo "  init-storage - Create storage directory"
	@echo "  help         - Show this help"
