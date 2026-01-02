#
# Created by Jugal Kishore -- 2025
#
FROM golang:1.25-alpine AS builder

# Enable Go module support & install dependencies
ENV CGO_ENABLED=0

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Download dependencies and build the binary
RUN go build -o rdctl-bot ./cmd/rdctl-bot

# Stage 2: Minimal runtime image
FROM alpine:3.23

# Set working directory
WORKDIR /app

# Copy the compiled binary from builder
COPY --from=builder /app/rdctl-bot .

# Default arguments (can be overridden via CMD or entrypoint)
CMD ["./rdctl-bot"]

# Expose Web Dashboard port
EXPOSE 8089
