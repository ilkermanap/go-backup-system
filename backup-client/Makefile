.PHONY: dev build clean install

# Install dependencies
install:
	cd frontend && npm install
	go mod tidy

# Development mode
dev:
	wails dev

# Build for production
build:
	wails build

# Build frontend only
frontend:
	cd frontend && npm run build

# Clean build artifacts
clean:
	rm -rf build/
	rm -rf frontend/dist/
	rm -rf frontend/node_modules/

# Run Go tests
test:
	go test ./...
