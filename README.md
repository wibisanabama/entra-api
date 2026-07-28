# Entra - White-Label Event Ticketing Platform (Backend API)

A microservice-based backend for a white-label event ticketing platform, built with Go, PostgreSQL, Redis, and Apache Kafka.

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.26+ (Gin framework) |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| Message Broker | Apache Kafka |
| Object Storage | MinIO |
| Query Gen | sqlc |
| Migrations | golang-migrate |

## Project Structure

The backend is composed of 7 microservices:

```
entra-api/
├── shared/           # Common packages (config, database, redis, kafka, middleware)
├── auth-service/     # Authentication & user management (Port: 8081)
├── event-service/    # Event, venue, and category management (Port: 8082)
├── ticket-service/   # Ticket quota, purchasing, and event producer (Port: 8083)
├── payment-service/  # Payment processing & gateway integration (Port: 8084)
├── cashless-service/ # Top-up & local RFID transactions (Port: 8085)
├── gate-service/     # Event check-in scanning & local DB sync (Port: 8086)
├── storage-service/  # Image uploads to MinIO Object Storage (Port: 8087)
├── docker-compose.yml
├── Makefile
└── go.work
```

## Getting Started

### Prerequisites

- Go 1.26+
- Docker & Docker Compose
- [sqlc](https://sqlc.dev/) (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)
- [golang-migrate](https://github.com/golang-migrate/migrate) (`go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`)

### Setup

```bash
# 1. Clone the repository
git clone <repo-url> && cd entra-api

# 2. Copy environment variables
cp .env.example .env

# 3. Start infrastructure (Postgres, Redis, Kafka, Zookeeper, MinIO)
docker compose up -d

# 4. Run database migrations for all services
make migrate-auth-up
make migrate-event-up
make migrate-ticket-up
make migrate-payment-up
make migrate-cashless-up
make migrate-gate-up

# 5. Generate sqlc code
make sqlc-all

# 6. Build the entire workspace
make build
```

## API Endpoints Overview

### Auth Service (`:8081`)
- `POST /api/v1/auth/register`: Register user
- `POST /api/v1/auth/login`: Login
- `POST /api/v1/auth/refresh`: Refresh token
- `GET /api/v1/auth/profile`: Get profile
- `PUT /api/v1/auth/profile`: Update profile

### Event Service (`:8082`)
- `GET /api/v1/events`: List events (Redis Cached)
- `GET /api/v1/events/:id`: Get event details (Redis Cached)
- `POST /api/v1/events`: Create event
- `PUT /api/v1/events/:id`: Update event
- `DELETE /api/v1/events/:id`: Delete event
- *Includes Venue and Category management routes*

### Ticket Service (`:8083`)
- `POST /api/v1/tickets/purchase`: Buy tickets
- `GET /api/v1/tickets/my`: View owned tickets
- `GET /api/v1/tickets/:id`: View specific ticket
- *Publishes `ticket.created` events to Kafka*
- *Consumes `ticket.scanned` events from Kafka*

### Payment Service (`:8084`)
- `POST /api/v1/payments/process`: Process external gateway payments
- `GET /api/v1/payments/:transaction_id`: Check payment status

### Cashless Service (`:8085`)
- `POST /api/v1/cashless/topup`: Top-up wristband balance
- `POST /api/v1/cashless/pay`: Deduct balance for merchant items
- `GET /api/v1/cashless/balance/:user_id`: Check local balance

### Gate Service (`:8086`)
- `POST /api/v1/gate/scan`: Scan ticket QR code for entry
- *Consumes `ticket.created` events from Kafka to sync local SQLite/PostgreSQL*
- *Publishes `ticket.scanned` events to Kafka*

### Storage Service (`:8087`)
- `POST /api/v1/storage/upload`: Upload image to MinIO (returns public URL)

## License

Proprietary - All rights reserved.
