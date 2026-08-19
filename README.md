# Entra Backend API Specification and Architecture Document

An enterprise-grade, microservice-based backend platform for event ticketing, access control, and cashless payment management. Built with Go, PostgreSQL, Redis, Apache Kafka, and MinIO Object Storage.

---

## Architecture Overview

The system is designed following microservices principles, decomposing core business domains into autonomous services. Communication occurs synchronously via HTTP/REST APIs for request-response operations and asynchronously via Apache Kafka for event-driven workflows.

```
                    ┌─────────────────────────┐
                    │      Client Apps        │
                    │  (Mobile App / Web UI)  │
                    └────────────┬────────────┘
                                 │
     ┌───────────────────────────┼───────────────────────────┐
     │                           │                           │
     ▼                           ▼                           ▼
┌──────────────┐          ┌──────────────┐          ┌──────────────┐
│ auth-service │          │event-service │          │ticket-service│
│ (Port 8081)  │          │ (Port 8082)  │          │ (Port 8083)  │
└──────┬───────┘          └──────┬───────┘          └──────┬───────┘
       │                         │                         │
       ▼                         ▼                         ▼
┌──────────────┐          ┌──────────────┐          ┌──────────────┐
│  entra_auth  │          │ entra_event  │          │ entra_ticket │
│ (PostgreSQL) │          │(Pg + Redis)  │          │ (PostgreSQL) │
└──────────────┘          └──────────────┘          └──────┬───────┘
                                                           │ (Kafka: ticket.created)
                                                           ▼
┌──────────────┐          ┌──────────────┐          ┌──────────────┐
│storage-serv. │          │cashless-serv.│          │ gate-service │
│ (Port 8087)  │          │ (Port 8085)  │          │ (Port 8086)  │
└──────┬───────┘          └──────┬───────┘          └──────┬───────┘
       │                         │                         │
       ▼                         ▼                         ▼
┌──────────────┐          ┌──────────────┐          ┌──────────────┐
│    MinIO     │          │entra_cashless│          │  entra_gate  │
│(Obj Storage) │          │ (PostgreSQL) │          │ (PostgreSQL) │
└──────────────┘          └──────────────┘          └──────────────┘
```

---

## Microservices Breakdown

### 1. Auth Service (`auth-service`) - Port 8081
- **Domain**: Identity, authentication, token management, and user profiles.
- **Database**: `entra_auth` (PostgreSQL).
- **Features**:
  - User registration and authentication using JWT (JSON Web Tokens).
  - Access and refresh token rotation mechanism.
  - Profile retrieval, updates, and organizer KYC application processing.
  - Internal batch user lookup API (`POST /api/v1/auth/users/batch`) for cross-service buyer profile enrichment.

### 2. Event Service (`event-service`) - Port 8082
- **Domain**: Event catalog management, categories, venues, and ticket type definitions.
- **Database**: `entra_event` (PostgreSQL) & Redis 7 (Cache).
- **Features**:
  - Event creation, listing, updating, and search functionality.
  - Category and venue management.
  - Ticket quota reservation and release APIs for internal service calls.
  - Redis caching for public event listing endpoints to optimize read latency.

### 3. Ticket Service (`ticket-service`) - Port 8083
- **Domain**: Ticket ordering, payment integration, attendee management, and order analytics.
- **Database**: `entra_ticket` (PostgreSQL).
- **Features**:
  - Order creation and inventory stock deduction.
  - Integration with Midtrans Payment Gateway (Snap API & Webhooks).
  - Attendee list generation with batch user profile enrichment via `auth-service`.
  - Dual lookup support for ticket verification by `id` (UUID) or `ticket_code`.
  - Publishes `ticket.created` events to Kafka upon completed order payment.

### 4. Payment Service (`payment-service`) - Port 8084
- **Domain**: Payment gateway processing and transaction simulation.
- **Database**: `entra_payment` (PostgreSQL).
- **Features**:
  - Payment reference lookups and gateway transaction intent generation.
  - Local sandbox payment simulation endpoint for end-to-end testing without external network dependencies.

### 5. Cashless Service (`cashless-service`) - Port 8085
- **Domain**: On-site event NFC/RFID wristband balance and merchant transactions.
- **Database**: `entra_cashless` (PostgreSQL).
- **Features**:
  - Digital wallet initialization and top-up processing.
  - Merchant payment deduction and real-time transaction ledger.

### 6. Gate Service (`gate-service`) - Port 8086
- **Domain**: Real-time event gate check-in and access control scanning.
- **Database**: `entra_gate` (PostgreSQL).
- **Features**:
  - Ticket QR/barcode check-in scanning.
  - Fallback local database lookup and ticket-service HTTP synchronization.
  - Strict `event_id` verification preventing cross-event ticket fraud.
  - Consumes `ticket.created` Kafka topic for pre-synchronizing tickets to local gate storage.
  - Publishes `ticket.scanned` Kafka events upon valid entry.

### 7. Storage Service (`storage-service`) - Port 8087
- **Domain**: Media upload management.
- **Storage**: MinIO S3-Compatible Object Storage.
- **Features**:
  - Multipart image file uploads for event banners, venue layouts, and user avatars.
  - Returns public CDN/MinIO URLs for client display.

---

## Infrastructure and Technology Stack

| Layer / Role | Technology Choice | Details |
|---|---|---|
| Programming Language | Go 1.26+ | Built using the `go.work` workspace specification |
| Web Framework | Gin Framework | High-performance HTTP routing and middleware |
| Relational Database | PostgreSQL 16 | 6 isolated databases for service encapsulation |
| In-Memory Cache | Redis 7 | Event catalog caching |
| Message Broker | Apache Kafka 7.6 | Asynchronous event streaming via Zookeeper |
| Object Storage | MinIO | Self-hosted S3-compatible media storage |
| Database Query Tool | sqlc | Type-safe Go code generation from SQL queries |
| Migration Engine | golang-migrate | Version-controlled schema migrations |

---

## Environment Variables Configuration

Copy `.env.example` to `.env` in the root directory prior to running the system:

```ini
# PostgreSQL Infrastructure
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=entra
POSTGRES_PASSWORD=entra_secret
POSTGRES_DB_AUTH=entra_auth
POSTGRES_DB_EVENT=entra_event
POSTGRES_DB_TICKET=entra_ticket
POSTGRES_DB_PAYMENT=entra_payment
POSTGRES_DB_CASHLESS=entra_cashless
POSTGRES_DB_GATE=entra_gate
POSTGRES_SSLMODE=disable

# Redis Cache
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# Apache Kafka
KAFKA_BROKERS=localhost:9092

# Object Storage (MinIO)
MINIO_ENDPOINT=localhost:9000
MINIO_ROOT_USER=entra_minio
MINIO_ROOT_PASSWORD=entra_minio_secret

# Auth Service Settings
AUTH_SERVICE_PORT=8081
JWT_SECRET=your-enterprise-secure-jwt-secret-key
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h

# Service Ports
EVENT_SERVICE_PORT=8082
TICKET_SERVICE_PORT=8083
PAYMENT_SERVICE_PORT=8084
CASHLESS_SERVICE_PORT=8085
GATE_SERVICE_PORT=8086
STORAGE_SERVICE_PORT=8087
```

---

## Installation and Execution Guide

### Prerequisites

Ensure the following dependencies are installed on the target machine:
- Go version 1.26 or higher
- Docker Engine and Docker Compose
- `sqlc` (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)
- `golang-migrate` (`go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`)

### Setup Procedure

1. **Clone the repository**:
   ```bash
   git clone https://github.com/wibisanabama/entra-api.git
   cd entra-api
   ```

2. **Configure environment settings**:
   ```bash
   cp .env.example .env
   ```

3. **Launch containerized infrastructure**:
   ```bash
   make up
   ```
   *Starts PostgreSQL, Redis, Kafka, Zookeeper, and MinIO in detached mode.*

4. **Execute database migrations**:
   ```bash
   make migrate-auth-up
   make migrate-event-up
   make migrate-ticket-up
   make migrate-payment-up
   make migrate-cashless-up
   make migrate-gate-up
   ```

5. **Generate type-safe SQL queries**:
   ```bash
   make sqlc-all
   ```

6. **Compile workspace binaries**:
   ```bash
   make build
   ```

### Execution Commands

To execute individual services locally:

```bash
# Run Auth Service
make run-auth

# Run Event Service
make run-event

# Run Ticket Service
make run-ticket

# Run Payment Service
make run-payment

# Run Cashless Service
make run-cashless

# Run Gate Service
make run-gate

# Run Storage Service
make run-storage
```

---

## API Reference Specifications

### Auth Service (`:8081`)

| Method | Endpoint | Access | Description |
|---|---|---|---|
| GET | `/health` | Public | Service health check |
| POST | `/api/v1/auth/register` | Public | User registration |
| POST | `/api/v1/auth/login` | Public | User login, returns JWT access & refresh tokens |
| POST | `/api/v1/auth/refresh` | Public | Refresh expired access token |
| POST | `/api/v1/auth/forgot-password` | Public | Request password reset token via email |
| POST | `/api/v1/auth/reset-password` | Public | Reset password using email verification token |
| GET | `/api/v1/auth/profile` | Protected | Retrieve active user profile |
| PUT | `/api/v1/auth/profile` | Protected | Update active user profile |
| POST | `/api/v1/auth/change-password` | Protected | Directly change password with old password verification |
| POST | `/api/v1/auth/upgrade` | Protected | Submit KYC data for organizer account upgrade |
| POST | `/api/v1/auth/users/batch` | Internal | Retrieve user details in bulk by ID list |

### Event Service (`:8082`)

| Method | Endpoint | Access | Description |
|---|---|---|---|
| GET | `/api/v1/events` | Public | List published events (Redis cached) |
| GET | `/api/v1/events/:id` | Public | Retrieve detailed event metadata |
| GET | `/api/v1/events/:id/tickets` | Public | List available ticket types for an event |
| GET | `/api/v1/categories` | Public | List event categories |
| GET | `/api/v1/venues` | Public | List available venues |
| GET | `/api/v1/organizer/events` | Protected | List events owned by the authenticated organizer |
| POST | `/api/v1/events` | Protected | Create a new event draft |
| PUT | `/api/v1/events/:id` | Protected | Update event information |
| DELETE | `/api/v1/events/:id` | Protected | Remove an event |

### Ticket Service (`:8083`)

| Method | Endpoint | Access | Description |
|---|---|---|---|
| POST | `/api/v1/tickets/orders` | Protected | Place a new ticket order |
| GET | `/api/v1/tickets/orders` | Protected | List order history of authenticated user |
| POST | `/api/v1/tickets/orders/:id/pay` | Protected | Generate Midtrans Snap payment token |
| POST | `/api/v1/tickets/promo/validate` | Public | Validate promotional discount voucher and calculate final price |
| POST | `/api/v1/tickets/midtrans/webhook` | Public | Midtrans payment notification webhook listener |
| GET | `/api/v1/tickets` | Protected | List active tickets owned by user |
| POST | `/api/v1/tickets/:id/transfer` | Protected | Transfer unused ticket to another user via email |
| GET | `/api/v1/tickets/organizer/stats` | Protected | Retrieve organizer sales and revenue statistics |
| GET | `/api/v1/tickets/organizer/trend` | Protected | Retrieve daily sales trend for organizer |
| GET | `/api/v1/tickets/organizer/orders` | Protected | List all orders across organizer's events |
| GET | `/api/v1/tickets/organizer/orders/:id` | Protected | Get single order detail with line items & tickets |
| GET | `/api/v1/tickets/organizer/events/:eventId/attendees` | Protected | List checked-in and total attendees with buyer name enrichment |
| GET | `/api/v1/internal/events/:eventId/gate-stats` | Internal | Retrieve real-time attendance ratios and gate count for gate-service |
| GET | `/api/v1/tickets/organizer/balance` | Protected | Retrieve organizer available balance, total revenue, and withdrawn amounts |
| POST | `/api/v1/tickets/organizer/withdrawals` | Protected | Submit a withdrawal request with bank account details |
| GET | `/api/v1/tickets/organizer/withdrawals` | Protected | List withdrawal request history for organizer |
| GET | `/api/v1/tickets/organizer/withdrawals/:id` | Protected | Retrieve single withdrawal request detail |
| GET | `/api/v1/tickets/admin/withdrawals` | Protected | Admin endpoint to list all withdrawal requests |
| PATCH | `/api/v1/tickets/admin/withdrawals/:id/status` | Protected | Admin endpoint to approve, reject, or mark withdrawal as paid |

### Gate Service (`:8086`)

| Method | Endpoint | Access | Description |
|---|---|---|---|
| POST | `/api/v1/gate/scan` | Protected | Validate and check-in ticket via QR code or ticket ID |
| GET | `/api/v1/gate/stats/:eventId` | Public | Retrieve real-time attendance percentage, total checked-in, and remaining attendees |

### Cashless Service (`:8085`)

| Method | Endpoint | Access | Description |
|---|---|---|---|
| GET | `/api/v1/cashless/wallet` | Protected | Retrieve wristband wallet balance |
| POST | `/api/v1/cashless/topup` | Protected | Process balance top-up |
| POST | `/api/v1/cashless/pay` | Protected | Deduct balance for merchant purchase |
| POST | `/api/v1/cashless/refund` | Protected | Request refund of remaining wristband balance to bank account |
| GET | `/api/v1/cashless/transactions` | Protected | Retrieve transaction ledger history |

### Storage Service (`:8087`)

| Method | Endpoint | Access | Description |
|---|---|---|---|
| POST | `/api/v1/storage/upload` | Protected | Upload image asset to MinIO bucket |
| GET | `/api/v1/storage/media` | Protected | List stored media assets |

---

## Event Streaming Specs (Apache Kafka)

| Topic Name | Producer Service | Consumer Service | Payload Contract |
|---|---|---|---|
| `ticket.created` | `ticket-service` | `gate-service` | `{ "id": "UUID", "ticket_code": "STRING", "status": "ACTIVE" }` |
| `ticket.scanned` | `gate-service` | `ticket-service` | `{ "ticket_id": "UUID", "ticket_code": "STRING" }` |

---

## License

Proprietary Software. All rights reserved. Unauthorized copying, distribution, or modification of this software is strictly prohibited.
