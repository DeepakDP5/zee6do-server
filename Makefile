.PHONY: proto-gen proto-lint proto-breaking proto-dep-update build test test-integration lint tidy clean dev dev-stop help

# Proto / Buf targets
proto-gen: ## Generate Go code from proto definitions
	buf generate

proto-lint: ## Lint proto files
	buf lint

proto-breaking: ## Check for breaking proto changes against main
	@if git ls-tree -r --name-only main -- proto/ 2>/dev/null | grep -q '\.proto$$'; then \
		buf breaking --against '.git#branch=main'; \
	else \
		echo "Skipping breaking change check: no .proto files found on main branch (initial import)."; \
	fi

proto-dep-update: ## Update buf.lock with latest dependency versions
	buf dep update

# Go targets
build: ## Build the server binary
	go build -o bin/zee6do-server ./cmd/server/

test: ## Run unit tests
	go test ./... -count=1 -timeout 60s

test-integration: ## Run integration tests (requires MongoDB)
	go test -tags=integration ./... -count=1 -timeout 120s

lint: ## Run golangci-lint
	golangci-lint run ./...

tidy: ## Tidy Go modules
	go mod tidy

clean: ## Remove build artifacts
	rm -rf bin/

# Development helpers
dev: ## Start local dependencies (MongoDB)
	docker compose up -d

dev-stop: ## Stop local dependencies
	docker compose down

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
