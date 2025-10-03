# Gomini - Unified Go LLM Client

.PHONY: help build run test clean deps example

# Default target
help: ## Show this help message
	@echo "Gomini - Unified Go LLM Client"
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Install dependencies
	go mod tidy
	go mod download

build: ## Build the example application
	go build -o bin/example ./cmd/example

run: build ## Run the example application
	./bin/example

example: ## Run the example with environment setup reminder
	@echo "🚀 Running example..."
	@echo "Make sure you have set up your environment variables:"
	@echo "  OPENAI_API_KEY=your_openai_key"
	@echo "  GEMINI_API_KEY=your_gemini_key"
	@echo "  # OR for Vertex AI:"
	@echo "  GOOGLE_GENAI_USE_VERTEXAI=true"
	@echo "  GOOGLE_CLOUD_PROJECT=your-project"
	@echo "  GOOGLE_CLOUD_LOCATION=us-central1"
	@echo ""
	@$(MAKE) run

test: ## Run tests
	go test -v ./pkg/gomini/...
	go test -v ./pkg/gomini/providers/...

clean: ## Clean build artifacts
	rm -rf bin/
	go clean

format: ## Format code
	go fmt ./...
	goimports -w .

lint: ## Run linters
	golangci-lint run ./...

docs: ## Generate documentation
	godoc -http=:6060
	@echo "Documentation available at http://localhost:6060/pkg/gomini/pkg/gomini/"

# Development helpers
dev-setup: ## Set up development environment
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

check: format lint test ## Run all checks (format, lint, test)

# Release helpers
version ?= v0.1.0
tag: ## Create a git tag (usage: make tag version=v0.1.0)
	git tag -a $(version) -m "Release $(version)"
	git push origin $(version)

release: check ## Prepare for release
	@echo "✅ All checks passed - ready for release!"

.DEFAULT_GOAL := help