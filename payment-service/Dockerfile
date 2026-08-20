# Multi-stage Dockerfile for payment-service
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy Go workspace and dependencies
COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY payment-service/ ./payment-service/

WORKDIR /app/payment-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/payment-service ./cmd/api

# Minimal runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/bin/payment-service /app/payment-service

EXPOSE 8084

USER nobody:nobody

ENTRYPOINT ["/app/payment-service"]
