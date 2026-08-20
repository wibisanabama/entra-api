# Multi-stage Dockerfile for auth-service
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy Go workspace and dependencies
COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY auth-service/ ./auth-service/

WORKDIR /app/auth-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/auth-service ./cmd/api

# Minimal runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/bin/auth-service /app/auth-service

EXPOSE 8081

USER nobody:nobody

ENTRYPOINT ["/app/auth-service"]
