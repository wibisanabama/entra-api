# Multi-stage Dockerfile for ticket-service
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy Go workspace and dependencies
COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY ticket-service/ ./ticket-service/

WORKDIR /app/ticket-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/ticket-service ./cmd/api

# Minimal runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/bin/ticket-service /app/ticket-service

EXPOSE 8083

USER nobody:nobody

ENTRYPOINT ["/app/ticket-service"]
