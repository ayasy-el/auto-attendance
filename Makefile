.PHONY: all build test run once dry-run dry-run-once clean docker-build docker-run docker-stop help

APP_NAME := auto-attendance
BIN_DIR := bin
BINARY := $(BIN_DIR)/$(APP_NAME)
CONFIG := config.yaml

all: test build

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build binary executable
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/attendance

test: ## Run unit tests
	go test -v -count=1 ./...

run: ## Run daemon continuously
	go run ./cmd/attendance -config $(CONFIG)

once: ## Run a single attendance check pass
	go run ./cmd/attendance -config $(CONFIG) -once

dry-run: ## Run in simulation mode (read-only)
	go run ./cmd/attendance -config $(CONFIG) -dry-run

dry-run-once: ## Run a single simulation check pass
	go run ./cmd/attendance -config $(CONFIG) -once -dry-run

docker-build: ## Build Docker container image locally
	docker build -t $(APP_NAME):latest .

docker-run: ## Run Docker container with docker compose
	docker compose up -d

docker-stop: ## Stop Docker container
	docker compose down

clean: ## Clean build artifacts
	rm -rf $(BIN_DIR)
