.PHONY: build build-frontend dev clean install help

VERSION := 0.1.0

# Build everything (frontend first, then Go binary)
build: build-frontend
	@go build -o bin/spec-designer .
	@echo "Built bin/spec-designer"

# Build only the frontend (Vite → internal/ui/dist)
build-frontend:
	@cd web && npm install && npm run build
	@echo "Frontend built to internal/ui/dist"

# Run in development: frontend hot-reload + Go server
dev:
	@echo "Run in separate terminals:"
	@echo "  Terminal 1: cd web && npm run dev"
	@echo "  Terminal 2: go run . open"

# Install globally
install: build
	@sudo cp bin/spec-designer /usr/local/bin/
	@echo "Installed to /usr/local/bin/spec-designer"

# Clean build artifacts
clean:
	@rm -rf bin/
	@rm -rf internal/ui/dist/
	@cd web && rm -rf node_modules dist
	@go clean

# Download Go dependencies
deps:
	@go mod download && go mod tidy

# Format
fmt:
	@go fmt ./...

help:
	@echo "make build          — build frontend + Go binary"
	@echo "make build-frontend — build only the Vite frontend"
	@echo "make dev            — print dev instructions"
	@echo "make install        — install to /usr/local/bin"
	@echo "make clean          — remove build artifacts"
