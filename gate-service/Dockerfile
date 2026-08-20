# Multi-stage Dockerfile for gate-service
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy Go workspace and dependencies
COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY gate-service/ ./gate-service/

WORKDIR /app/gate-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/gate-service ./cmd/api

# Minimal runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/bin/gate-service /app/gate-service

EXPOSE 8086

USER nobody:nobody

ENTRYPOINT ["/app/gate-service"]
