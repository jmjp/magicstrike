# Build stage
FROM docker.io/golang:1.26.3-alpine AS builder

WORKDIR /app

# Copy module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the worker CLI and API server
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/worker cmd/worker/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api cmd/api/main.go

# Production stage
FROM docker.io/alpine:latest

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy the built binaries
COPY --from=builder /app/worker /app/worker
COPY --from=builder /app/api /app/api

USER appuser

# Default entrypoint — override via docker-compose command
ENTRYPOINT ["/app/worker"]
CMD ["-mode", "consumer"]
