#
# Created by Jugal Kishore -- 2025
#
FROM golang:1.26-alpine AS builder

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
RUN go build -ldflags="-s -w" -o rdctl-bot ./cmd/rdctl-bot

# Stage 2: Minimal scratch image
FROM scratch

# Set working directory
WORKDIR /app

# Copy the compiled binary
COPY --from=builder /app/rdctl-bot .

# Copy SSL certificates (required for HTTPS/TLS)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Use exec form to avoid shell
ENTRYPOINT ["./rdctl-bot"]

# Expose Web Dashboard port
EXPOSE 8089
