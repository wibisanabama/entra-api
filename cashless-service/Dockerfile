# Multi-stage Dockerfile for cashless-service
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy Go workspace and dependencies
COPY go.work go.work.sum ./
COPY shared/ ./shared/
COPY cashless-service/ ./cashless-service/

WORKDIR /app/cashless-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/cashless-service ./cmd/api

# Minimal runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/bin/cashless-service /app/cashless-service

EXPOSE 8085

USER nobody:nobody

ENTRYPOINT ["/app/cashless-service"]
