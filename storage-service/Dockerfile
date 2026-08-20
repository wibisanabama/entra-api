# Multi-stage Dockerfile for storage-service
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy Go workspace and dependencies
COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY storage-service/ ./storage-service/

WORKDIR /app/storage-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/storage-service ./cmd/api

# Minimal runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/bin/storage-service /app/storage-service

EXPOSE 8087

USER nobody:nobody

ENTRYPOINT ["/app/storage-service"]
