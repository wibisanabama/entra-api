# Entra API

Backend microservices platform for event management, high-concurrency ticket reservation, waiting room queue processing, payment gateway orchestration, and gate access validation.

---

## 1. System Overview

Entra API is architected as an event-driven distributed system designed to handle high-traffic ticket sales (ticket war/flash sale) while maintaining strict consistency, zero overselling, and idempotent financial transactions. Communication between microservices relies on synchronous REST API contracts for direct client requests and asynchronous Apache Kafka event streams for transaction and ticket lifecycle events.

### Microservices Portfolio

| Service Name | Port | Database | Primary Responsibilities |
| --- | --- | --- | --- |
| `auth-service` | 8081 | `entra_auth` | User identity lifecycle, JWT authentication, account profile management, and Role-Based Access Control (RBAC). |
| `event-service` | 8082 | `entra_event` | Event catalog, venue specifications, seating tiers, category classifications, and ticket metadata pre-warming. |
| `ticket-service` | 8083 | `entra_ticket` | Order creation, atomic stock reservation, waiting room queue orchestration, Midtrans payment tokenization, and organizer withdrawal processing. |
| `payment-service` | 8084 | `entra_payment` | Payment intent tracking, transaction status webhooks, and billing reconciliation. |
| `gate-service` | 8086 | `entra_gate` | High-throughput QR code scan validation, check-in gate management, and double-entry prevention. |
| `storage-service` | 8087 | MinIO / Object Store | Media asset uploads, image transformations, banner distribution, and pre-signed URL management. |

---

## 2. System Architecture

```mermaid
flowchart TD
    ClientApp["Clients (Web / Mobile)"]

    subgraph GatewayLayer["API Ingress"]
        AuthSvc["auth-service (:8081)"]
        EventSvc["event-service (:8082)"]
        TicketSvc["ticket-service (:8083)"]
        PaymentSvc["payment-service (:8084)"]
        GateSvc["gate-service (:8086)"]
        StorageSvc["storage-service (:8087)"]
    end

    subgraph StateAndCache["State & Concurrency Layer"]
        PostgresDB[("PostgreSQL 16 (Port 5433)")]
        RedisCache[("Redis 7 (Port 6379)\nAtomic Lua + Queue")]
    end

    subgraph MessagingAndStorage["Event Stream & Storage"]
        KafkaBroker["Apache Kafka (:9092)"]
        MinioStore["MinIO Object Storage (:9000)"]
    end

    ClientApp --> AuthSvc
    ClientApp --> EventSvc
    ClientApp --> TicketSvc
    ClientApp --> PaymentSvc
    ClientApp --> GateSvc
    ClientApp --> StorageSvc

    AuthSvc --> PostgresDB
    EventSvc --> PostgresDB
    EventSvc --> RedisCache

    TicketSvc --> PostgresDB
    TicketSvc --> RedisCache
    TicketSvc --> KafkaBroker

    PaymentSvc --> PostgresDB
    PaymentSvc --> KafkaBroker

    GateSvc --> PostgresDB
    GateSvc --> RedisCache

    StorageSvc --> MinioStore
```

---

## 3. Core Architectural Mechanisms

### 3.1 High-Concurrency Ticket War Protection
- Atomic Stock Decrement: Quota checking and inventory reservation are executed inside Redis via Lua scripts (`reserveStockLua`), guaranteeing sub-millisecond execution without database lock contention.
- Zero Overselling Guarantee: Stock cannot drop below zero. Once Redis stock reaches zero, subsequent requests receive HTTP 409 Conflict immediately.
- Anti-Cache Stampede: Implemented via Go `singleflight.Group` to ensure that only a single upstream query hits the primary database during cold-start or cache-miss conditions.
- Idempotency Guard: Client requests include an `Idempotency-Key` HTTP header. Duplicate requests within a 5-minute window are rejected with HTTP 429 or served from the cached transaction record.

### 3.2 Waiting Room Queue Engine
- Queue State Machine: Orders enter an event-specific Redis queue (`queue:event:<id>:waiting`). Only 1 buyer holds the `ACTIVE` status per event at any given time.
- 3-Minute Payment Lease: Active buyers receive an explicit 3-minute payment window (`QueueActiveDuration`). Expiration triggers an automatic rollback of inventory and promotes the next order in line.
- Instant Cancellation: Buyers can release their slot immediately, advancing the queue without waiting for the 3-minute lease timeout.
- Monotonic Expiry Sweeper: Background workers run every 2 seconds to scan for expired orders, rolling back database and Redis states automatically.

### 3.3 Transparent Revenue Sharing and Commission
- Platform Commission: A fixed 5.0% platform fee (`PLATFORM_FEE_PERCENT`) is deducted from gross ticket revenue upon successful settlement.
- Free Ticket Exemption: Complimentary tickets (Rp 0) are exempt from platform commission charges.
- Financial Balance Metrics:
  - `gross_revenue`: Cumulative value of paid orders.
  - `platform_fee_amount`: Accumulated 5% platform commission.
  - `net_revenue`: Total organizer entitlement (`gross_revenue - platform_fee_amount`).
  - `available_balance`: Net balance eligible for withdrawal (`net_revenue - total_withdrawn`).

---

## 4. Prerequisites

- Go: Version 1.22 or higher
- Docker: Version 24.0 or higher
- Docker Compose: Version 2.20 or higher
- golang-migrate CLI: For local relational database migrations

---

## 5. Environment Configuration

Copy the example environment configuration to `.env` in the repository root:

```bash
cp .env.example .env
```

Key environment parameters:

| Variable | Description | Default / Example |
| --- | --- | --- |
| `APP_ENV` | Application runtime environment | `development` |
| `INTERNAL_SERVICE_SECRET` | Secret token for secure inter-service communication | `entra-super-secret-internal-token` |
| `POSTGRES_HOST` | Database host address | `127.0.0.1` |
| `POSTGRES_PORT` | Database external host port | `5433` |
| `POSTGRES_USER` | PostgreSQL superuser | `entra` |
| `POSTGRES_PASSWORD` | PostgreSQL password | `entra_secret` |
| `POSTGRES_DB_AUTH` | Auth service database name | `entra_auth` |
| `POSTGRES_DB_EVENT` | Event service database name | `entra_event` |
| `POSTGRES_DB_TICKET` | Ticket service database name | `entra_ticket` |
| `POSTGRES_DB_PAYMENT` | Payment service database name | `entra_payment` |
| `POSTGRES_DB_GATE` | Gate service database name | `entra_gate` |
| `REDIS_HOST` | Redis host address | `localhost` |
| `REDIS_PORT` | Redis port | `6379` |
| `KAFKA_BROKERS` | Kafka broker bootstrap server addresses | `localhost:9092` |
| `MINIO_ENDPOINT` | MinIO S3 API endpoint | `localhost:9000` |
| `MINIO_ROOT_USER` | MinIO access key | `entra_minio` |
| `MINIO_ROOT_PASSWORD` | MinIO secret key | `entra_minio_secret` |
| `JWT_SECRET` | Secret key for signing access tokens | Required |
| `JWT_ACCESS_EXPIRY` | Access token lifespan | `15m` |
| `JWT_REFRESH_EXPIRY` | Refresh token lifespan | `168h` |
| `MIDTRANS_SERVER_KEY` | Midtrans server-side integration key | Configured from Midtrans Dashboard |
| `MIDTRANS_CLIENT_KEY` | Midtrans client-side integration key | Configured from Midtrans Dashboard |
| `PLATFORM_FEE_PERCENT` | Platform commission percentage | `5.0` |

---

## 6. Setup and Execution

### 6.1 Infrastructure Deployment

Start database, cache, message broker, and object storage containers:

```bash
docker compose up -d
```

Verify container readiness:

```bash
docker compose ps
```

### 6.2 Database Migrations

Apply database schemas across all services using `golang-migrate`:

```bash
# Linux / macOS
for svc in auth event ticket payment gate; do
  migrate -path "./${svc}-service/migrations" -database "postgres://entra:entra_secret@localhost:5433/entra_${svc}?sslmode=disable" up
done
```

```powershell
# Windows PowerShell
$services = @('auth', 'event', 'ticket', 'payment', 'gate')
foreach ($svc in $services) {
    migrate -path "./$svc-service/migrations" -database "postgres://entra:entra_secret@localhost:5433/entra_$($svc)?sslmode=disable" up
}
```

### 6.3 Executing Services

Run each microservice independently from the project root:

```bash
# Terminal 1: Auth Service
go run ./auth-service/cmd/api

# Terminal 2: Event Service
go run ./event-service/cmd/api

# Terminal 3: Ticket Service
go run ./ticket-service/cmd/api

# Terminal 4: Payment Service
go run ./payment-service/cmd/api

# Terminal 5: Gate Service
go run ./gate-service/cmd/api

# Terminal 6: Storage Service
go run ./storage-service/cmd/api
```

Health check endpoints are available on every service port:
- `http://localhost:8081/health`
- `http://localhost:8082/health`
- `http://localhost:8083/health`
- `http://localhost:8084/health`
- `http://localhost:8086/health`
- `http://localhost:8087/health`

---

## 7. Testing and Code Generation

### 7.1 Running Automated Tests

Run the complete test suite across all services:

```bash
go test -count=1 ./...
```

Run ticket concurrency and flash-sale benchmarks:

```bash
go test -v -run TestTicketWarConcurrency ./ticket-service/internal/service
go test -bench=. ./ticket-service/internal/service
```

### 7.2 Static Code Analysis

```bash
go vet ./...
```

### 7.3 Regenerating SQL Code (sqlc)

When modifying query definitions in `queries/*.sql` or schema definitions in `migrations/`:

```bash
cd <service-directory>
sqlc generate
```

---

## 8. API Specification Summary

### 8.1 Auth Service (`:8081`)
- `POST /api/v1/auth/register` — Register a new account.
- `POST /api/v1/auth/login` — Authenticate and retrieve token pair.
- `POST /api/v1/auth/refresh` — Refresh expired access token.
- `POST /api/v1/auth/forgot-password` — Request password reset token via email.
- `POST /api/v1/auth/reset-password` — Reset password using token.
- `GET /api/v1/auth/profile` — Retrieve current authenticated user profile.
- `PUT /api/v1/auth/profile` — Update user profile details.
- `PUT /api/v1/auth/change-password` — Update current password.

### 8.2 Event Service (`:8082`)
- `GET /api/v1/events` — List public events with pagination, category, and date filters.
- `GET /api/v1/events/:id` — Retrieve detailed event specification and ticket tiers.
- `POST /api/v1/events` — Create new event (Organizer role required).
- `PUT /api/v1/events/:id` — Update event metadata.
- `DELETE /api/v1/events/:id` — Cancel / delete event.
- `GET /api/v1/venues` — List available event venues.
- `POST /api/v1/venues` — Register a new venue.
- `POST /api/v1/events/:id/tickets` — Create ticket tier category.
- `PUT /api/v1/events/:id/tickets/:ticket_id` — Update ticket tier quota and price.

### 8.3 Ticket Service (`:8083`)
- `POST /api/v1/tickets/orders` — Create new order and reserve stock via Redis Lua.
- `GET /api/v1/tickets/orders/:id/queue` — Query waiting room queue position and remaining lease.
- `POST /api/v1/tickets/orders/:id/cancel` — Cancel order, release queue slot, and restore stock.
- `POST /api/v1/tickets/orders/:id/pay` — Generate Midtrans Snap token for active order.
- `GET /api/v1/tickets/my-tickets` — Retrieve user digital ticket passes and order history.
- `GET /api/v1/tickets/organizer/balance` — Query organizer gross revenue, fee, and available balance.
- `POST /api/v1/tickets/organizer/withdrawals` — Submit withdrawal request to bank account.
- `GET /api/v1/tickets/admin/stats/platform` — Aggregated platform financial performance metrics.
- `POST /api/v1/tickets/midtrans/webhook` — Process Midtrans payment notifications.

### 8.4 Gate Service (`:8086`)
- `POST /api/v1/gate/validate` — Validate QR ticket code at gate entrance.
- `GET /api/v1/gate/events/:id/attendees` — Retrieve live event attendee manifest and check-in status.

### 8.5 Storage Service (`:8087`)
- `POST /api/v1/storage/upload` — Upload event banner or avatar image to MinIO.
- `GET /api/v1/storage/files/:id` — Retrieve public image file.

---

## 9. Directory Structure

```text
entra-api/
├── auth-service/           # Identity, JWT, RBAC, and credential recovery
├── event-service/          # Catalog, venue relations, and ticket tier definitions
├── ticket-service/         # Concurrency engine, queue waiting room, and financial ledger
├── payment-service/        # Payment gateway abstraction and intent tracking
├── gate-service/           # High-speed QR scanner validation and check-in audit
├── storage-service/        # Object storage integration (MinIO/S3)
├── shared/                 # Shared utilities, configuration, middleware, and models
├── scripts/                # Database bootstrap and migration scripts
├── docker-compose.yml      # Local containerized infrastructure definitions
├── go.work                 # Multi-module Go workspace definition
└── Makefile                # Engineering build and test automation tasks
```
