# Multi-stage Dockerfile for event-service
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy Go workspace and dependencies
COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY event-service/ ./event-service/

WORKDIR /app/event-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/event-service ./cmd/api

# Minimal runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/bin/event-service /app/event-service

EXPOSE 8082

USER nobody:nobody

ENTRYPOINT ["/app/event-service"]
