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
	docker compose exec postgres migrate -path /migrations/auth -database "postgresql://postgres:postgres@localhost:5432/entra_auth?sslmode=disable" -verbose up

migrate-auth-down: ## Rollback auth-service migrations
	docker compose exec postgres migrate -path /migrations/auth -database "postgresql://postgres:postgres@localhost:5432/entra_auth?sslmode=disable" -verbose down

migrate-event-up: ## Run event-service migrations (up)
	docker compose exec postgres migrate -path /migrations/event -database "postgresql://postgres:postgres@localhost:5432/entra_event?sslmode=disable" -verbose up

migrate-event-down: ## Rollback event-service migrations
	docker compose exec postgres migrate -path /migrations/event -database "postgresql://postgres:postgres@localhost:5432/entra_event?sslmode=disable" -verbose down

migrate-ticket-up: ## Run ticket-service migrations (up)
	docker compose exec postgres migrate -path /migrations/ticket -database "postgresql://postgres:postgres@localhost:5432/entra_ticket?sslmode=disable" -verbose up

migrate-ticket-down: ## Rollback ticket-service migrations
	docker compose exec postgres migrate -path /migrations/ticket -database "postgresql://postgres:postgres@localhost:5432/entra_ticket?sslmode=disable" -verbose down

migrate-payment-up: ## Run payment-service migrations (up)
	docker compose exec postgres migrate -path /migrations/payment -database "postgresql://postgres:postgres@localhost:5432/entra_payment?sslmode=disable" -verbose up

migrate-payment-down: ## Rollback payment-service migrations
	docker compose exec postgres migrate -path /migrations/payment -database "postgresql://postgres:postgres@localhost:5432/entra_payment?sslmode=disable" -verbose down

migrate-cashless-up: ## Run cashless-service migrations (up)
	docker compose exec postgres migrate -path /migrations/cashless -database "postgresql://postgres:postgres@localhost:5432/entra_cashless?sslmode=disable" -verbose up

migrate-cashless-down: ## Rollback cashless-service migrations
	docker compose exec postgres migrate -path /migrations/cashless -database "postgresql://postgres:postgres@localhost:5432/entra_cashless?sslmode=disable" -verbose down

migrate-gate-up: ## Run gate-service migrations (up)
	docker compose exec postgres migrate -path /migrations/gate -database "postgresql://postgres:postgres@localhost:5432/entra_gate?sslmode=disable" -verbose up

migrate-gate-down: ## Rollback gate-service migrations
	docker compose exec postgres migrate -path /migrations/gate -database "postgresql://postgres:postgres@localhost:5432/entra_gate?sslmode=disable" -verbose down

# ─── sqlc ─────────────────────────────────────

sqlc-auth: ## Generate sqlc code for auth-service
	cd auth-service && sqlc generate

sqlc-event: ## Generate sqlc code for event-service
	cd event-service && sqlc generate

sqlc-all: sqlc-auth sqlc-event ## Generate sqlc code for all services
	cd ticket-service && sqlc generate
	cd payment-service && sqlc generate
	cd cashless-service && sqlc generate
	cd gate-service && sqlc generate

# ─── Build & Run ──────────────────────────────

build: ## Build all services
	go build ./auth-service/... ./event-service/... ./ticket-service/... ./payment-service/... ./cashless-service/... ./gate-service/...

run-auth: ## Run auth-service locally
	go run ./auth-service/cmd/api

run-event: ## Run event-service locally
	go run ./event-service/cmd/api

run-ticket: ## Run ticket-service locally
	go run ./ticket-service/cmd/api

run-payment: ## Run payment-service locally
	go run ./payment-service/cmd/api

run-cashless: ## Run cashless-service locally
	go run ./cashless-service/cmd/api

run-gate: ## Run gate-service locally
	go run ./gate-service/cmd/api

# ─── Lint & Test ──────────────────────────────

test: ## Run all tests
	go test ./...

lint: ## Run golangci-lint
	golangci-lint run ./...
