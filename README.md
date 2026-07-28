# Entra — White-Label Event Ticketing Platform (Backend API)

A microservice-based backend for a white-label event ticketing platform, built with Go, PostgreSQL, Redis, and Apache Kafka.

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.26+ (Gin framework) |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| Message Broker | Apache Kafka |
| Query Gen | sqlc |
| Migrations | golang-migrate |

## Project Structure

```
entra/
├── shared/          # Common packages (config, db, kafka, middleware)
├── auth-service/    # Authentication & user management
├── event-service/   # Event, venue, and ticket management
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
git clone <repo-url> && cd entra

# 2. Copy environment variables
cp .env.example .env

# 3. Start infrastructure
make up

# 4. Run database migrations
make migrate-auth-up
make migrate-event-up

# 5. Generate sqlc code
make sqlc-all

# 6. Run services
make run-auth   # Starts on :8081
make run-event  # Starts on :8082
```

## API Endpoints

### Auth Service (`:8081`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/api/v1/auth/register` | Register a new user |
| POST | `/api/v1/auth/login` | Login |
| POST | `/api/v1/auth/refresh` | Refresh access token |
| GET | `/api/v1/auth/profile` | Get current user profile |
| PUT | `/api/v1/auth/profile` | Update profile |

### Event Service (`:8082`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/api/v1/events` | Create event |
| GET | `/api/v1/events` | List events |
| GET | `/api/v1/events/:id` | Get event by ID |
| PUT | `/api/v1/events/:id` | Update event |
| DELETE | `/api/v1/events/:id` | Delete event |
| POST | `/api/v1/venues` | Create venue |
| GET | `/api/v1/venues` | List venues |
| GET | `/api/v1/venues/:id` | Get venue by ID |

## License

Proprietary — All rights reserved.
