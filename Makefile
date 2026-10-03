VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w -X conductor/internal/version.Version=$(VERSION) -X conductor/internal/version.Commit=$(COMMIT)

.PHONY: help install dev dev-api dev-web build test lint fmt clean

help: ## Show available targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[33m%-10s\033[0m %s\n", $$1, $$2}'

install: ## Install frontend dependencies
	cd web && pnpm install

dev: ## Run API and UI with hot reload (http://localhost:5180)
	@trap 'kill 0' INT TERM; \
	$(MAKE) --no-print-directory dev-api & \
	$(MAKE) --no-print-directory dev-web & \
	wait

dev-api: ## Run the Go API on :7070
	go run ./cmd/conductor --dev --data-dir .data

dev-web: ## Run the Vite dev server on :5180
	cd web && pnpm dev

build: ## Build a single binary with the UI embedded into ./bin/conductor
	cd web && pnpm build
	go build -tags embedui -ldflags "$(LDFLAGS)" -o bin/conductor ./cmd/conductor

test: ## Run all tests
	go test ./...
	cd web && pnpm typecheck && pnpm test

lint: ## Vet Go code and lint the frontend
	go vet ./...
	cd web && pnpm lint

fmt: ## Format Go and frontend code
	gofmt -w cmd internal web/*.go
	cd web && pnpm format

clean: ## Remove build output
	rm -rf bin web/dist
