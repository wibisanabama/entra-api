.PHONY: help up down logs migrate-auth migrate-event sqlc-auth sqlc-event build run-auth run-event

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ─── Docker ───────────────────────────────────

up: ## Start all infrastructure containers
	docker compose up -d

down: ## Stop all infrastructure containers
	docker compose down

logs: ## Tail infrastructure container logs
	docker compose logs -f

# ─── Database Migrations ─────────────────────

migrate-auth-up: ## Run auth-service migrations (up)
	migrate -path auth-service/migrations -database "postgres://entra:entra_secret@localhost:5432/entra_auth?sslmode=disable" up

migrate-auth-down: ## Rollback auth-service migrations
	migrate -path auth-service/migrations -database "postgres://entra:entra_secret@localhost:5432/entra_auth?sslmode=disable" down 1

migrate-event-up: ## Run event-service migrations (up)
	migrate -path event-service/migrations -database "postgres://entra:entra_secret@localhost:5432/entra_event?sslmode=disable" up

migrate-event-down: ## Rollback event-service migrations
	migrate -path event-service/migrations -database "postgres://entra:entra_secret@localhost:5432/entra_event?sslmode=disable" down 1

# ─── sqlc ─────────────────────────────────────

sqlc-auth: ## Generate sqlc code for auth-service
	cd auth-service && sqlc generate

sqlc-event: ## Generate sqlc code for event-service
	cd event-service && sqlc generate

sqlc-all: sqlc-auth sqlc-event ## Generate sqlc code for all services

# ─── Build & Run ──────────────────────────────

build: ## Build all services
	go build ./auth-service/... ./event-service/...

run-auth: ## Run auth-service locally
	go run ./auth-service/cmd/api

run-event: ## Run event-service locally
	go run ./event-service/cmd/api

# ─── Lint & Test ──────────────────────────────

test: ## Run all tests
	go test ./...

lint: ## Run golangci-lint
	golangci-lint run ./...
