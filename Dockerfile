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
FROM alpine:3.19

# MOVE THE INSTALLATION HERE
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    harfbuzz \
    ca-certificates \
    ttf-freefont

# Create user and a HOME directory
RUN adduser -D -u 1001 appuser && \
    mkdir -p /home/appuser && \
    chown -R 1001:1001 /home/appuser

# Setup /tmp with the "Sticky Bit" 
RUN mkdir -p /tmp && chmod 1777 /tmp

# Create the specific profile dir for your Go code and give ownership
RUN mkdir -p /tmp/chromedp-profile && chown -R 1001:1001 /tmp/chromedp-profile

WORKDIR /

# Non-root user (carried from builder)
COPY --from=builder /etc/passwd /etc/passwd

# SSL certificates (required for HTTPS outbound calls)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Binary
COPY --from=builder /app/server /server

# Migrations (goose reads these at startup via db.RunMigrations)
COPY --from=builder /app/migrations /migrations

ENV HOME=/home/appuser
ENV TMPDIR=/tmp

# Run as non-root
USER appuser

EXPOSE 8080

ENTRYPOINT ["/server"]