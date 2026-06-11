# ── Build stage ──────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install git (needed for module downloads)
RUN apk add --no-cache git

# Create non-root user for runtime
RUN adduser -D -u 1001 appuser

# Cache dependencies — layer is reused unless go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

# Install swag (pinned for reproducibility)
RUN go install github.com/swaggo/swag/cmd/swag@v1.8.12

# Copy source
COPY . .

# Generate swagger docs
RUN swag init -g cmd/api/main.go

# Build fully static binary — stripped, trimmed, reproducible
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-s -w -buildid=" \
      -trimpath \
      -o server \
      ./cmd/api


# ── Runtime stage ─────────────────────────────────────────────
FROM debian:bookworm-slim

WORKDIR /

# Install Chromium + all deps chromedp needs
RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    chromium-sandbox \
    ca-certificates \
    fonts-liberation \
    fonts-noto-color-emoji \
    libnss3 \
    libatk-bridge2.0-0 \
    libdrm2 \
    libxkbcommon0 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxrandr2 \
    libgbm1 \
    libasound2 \
    && rm -rf /var/lib/apt/lists/*

# Non-root user (carried from builder)
COPY --from=builder /etc/passwd /etc/passwd

# Binary
COPY --from=builder /app/server /server

# Migrations (goose reads these at startup via db.RunMigrations)
COPY --from=builder /app/migrations /migrations

# Run as non-root
USER appuser

EXPOSE 8080

ENTRYPOINT ["/server"]