# Stage 1: Build
FROM golang:1.25-alpine AS builder

# CGO is required for github.com/mattn/go-sqlite3
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
# We build the api-server from cmd/api-server
RUN CGO_ENABLED=1 GOOS=linux go build -o mangahub ./cmd/api-server/main.go

# Stage 2: Runtime
FROM alpine:latest

# Install dependencies
RUN apk add --no-cache sqlite ca-certificates

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/mangahub .

# Copy config file (if it exists)
COPY config.yaml* ./

# Copy initial data (for seeding)
COPY data/ ./data/

# Create logs directory
RUN mkdir -p logs

# Expose all necessary ports
# HTTP API: 8080
# TCP Sync: 8081
# UDP Notification: 9091 (UDP)
# gRPC Server: 9092
# WebSocket Chat: 9093
EXPOSE 8080 8081 9091/udp 9092 9093

# Command to run the server
ENTRYPOINT ["./mangahub"]
CMD ["server", "start"]
