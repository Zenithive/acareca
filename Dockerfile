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
    wget gnupg ca-certificates curl \
    && curl -fsSL https://dl.google.com/linux/linux_signing_key.pub \
       | gpg --dearmor -o /usr/share/keyrings/google-chrome.gpg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/google-chrome.gpg] \
       http://dl.google.com/linux/chrome/deb/ stable main" \
       > /etc/apt/sources.list.d/google-chrome.list \
    && apt-get update && apt-get install -y --no-install-recommends \
    google-chrome-stable \
    fonts-liberation \
    fonts-noto-color-emoji \
    && rm -rf /var/lib/apt/lists/*

# Writable tmp for chromium crash dumps + profile
RUN mkdir -p /tmp/chromedp-profile && chmod 1777 /tmp

# Non-root user (carried from builder)
COPY --from=builder /etc/passwd /etc/passwd

RUN mkdir -p /home/appuser/.local/share/applications \
    && touch /home/appuser/.local/share/applications/mimeapps.list \
    && chown -R appuser:appuser /home/appuser \
    && mkdir -p /tmp/chromedp-profile \
    && chmod 1777 /tmp

# Binary
COPY --from=builder /app/server /server

# Migrations (goose reads these at startup via db.RunMigrations)
COPY --from=builder /app/migrations /migrations

# Run as non-root
USER appuser

EXPOSE 8080

ENTRYPOINT ["/server"]