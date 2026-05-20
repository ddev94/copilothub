.PHONY: build build-frontend dev dev-knowledge knowledge-deps clean install help

VERSION := 0.1.0

# Build everything (frontend first, then Go binary)
build: build-frontend
	@go build -o bin/copilothub .
	@echo "Built bin/copilothub"

build-all:
	@echo "Building for all platforms..."
	@mkdir -p bin
	@GOOS=linux GOARCH=amd64 go build -o bin/copilothub-linux-amd64 .
	@GOOS=darwin GOARCH=amd64 go build -o bin/copilothub-darwin-amd64 .
	@GOOS=darwin GOARCH=arm64 go build -o bin/copilothub-darwin-arm64 .
	@GOOS=windows GOARCH=amd64 go build -o bin/copilothub-windows-amd64.exe .
	@echo "Build completed for all platforms!"

# Build only the frontend (Vite → internal/ui/dist)
build-frontend:
	@cd web && npm install && npm run build
	@echo "Frontend built to internal/ui/dist"

# Run in development: frontend hot-reload + Go server (+ optional knowledge sidecar)
dev:
	@echo "Run in separate terminals:"
	@echo "  Terminal 1: cd web && npm run dev"
	@echo "  Terminal 2: go run . open"
	@echo "  Terminal 3 (knowledge): make dev-knowledge"

# Install Python knowledge service dependencies into local venv
knowledge-deps:
	@cd python/knowledge_service && python3 -m venv .venv
	@cd python/knowledge_service && .venv/bin/python -m pip install -r requirements.txt

# Run Python knowledge sidecar (LangChain + ChromaDB, port 8001)
dev-knowledge:
	@echo "Starting knowledge sidecar on http://localhost:8001 ..."
	@cd python/knowledge_service && .venv/bin/python -m uvicorn main:app --host 0.0.0.0 --port 8001

# Setup venv deps + run sidecar
knowledge-setup-and-run: knowledge-deps dev-knowledge

# Install globally
install: build
	@sudo cp bin/copilothub /usr/local/bin/
	@echo "Installed to /usr/local/bin/copilothub"

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
	@echo "make build           — build frontend + Go binary"
	@echo "make build-frontend  — build only the Vite frontend"
	@echo "make dev             — print dev instructions"
	@echo "make knowledge-deps  — install Python knowledge service dependencies"
	@echo "make dev-knowledge   — run Python knowledge sidecar (port 8001)"
	@echo "make knowledge-setup-and-run — setup venv deps and run sidecar"
	@echo "make install         — install to /usr/local/bin"
	@echo "make clean           — remove build artifacts"

start:
	./bin/copilothub open -w ../Ferry
