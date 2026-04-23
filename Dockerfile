# ── Build stage ──────────────────────────────────────────────
# Use bullseye or bookworm (Debian) instead of Alpine to match runtime
FROM golang:1.21-bookworm AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Install swag
RUN go install github.com/swaggo/swag/cmd/swag@v1.8.12

# Copy source
COPY . .

# Generate swagger docs
RUN swag init -g cmd/api/main.go

# Build binary
# Note: We don't use CGO_ENABLED=0 if we want to use the system's root certs easily
RUN GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o server ./cmd/api

# ── Runtime stage ─────────────────────────────────────────────
# DO NOT USE scratch. Use a slim Debian image so we can install Chromium.
FROM debian:bookworm-slim

WORKDIR /app

# 1. Install Chromium and all necessary shared libraries for headless mode
RUN apt-get update && apt-get install -y \
    chromium \
    ca-certificates \
    fonts-liberation \
    libnss3 \
    libatk-bridge2.0-0 \
    libxcomposite1 \
    libxdamage1 \
    libxrandr2 \
    libgbm1 \
    libasound2 \
    --no-install-recommends && \
    rm -rf /var/lib/apt/lists/*

# 2. Copy the binary from builder
COPY --from=builder /app/server /app/server
COPY --from=builder /app/migrations /app/migrations

# 3. Create a non-root user (Debian syntax)
RUN useradd -u 1001 appuser
USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/server"]