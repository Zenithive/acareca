# ── Build stage ─────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git (needed for private/public modules)
RUN apk add --no-cache git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Install swag (pinned version for reproducibility)
RUN go install github.com/swaggo/swag/cmd/swag@v1.8.12

# Copy source
COPY . .

# Generate swagger docs
RUN swag init -g cmd/api/main.go

# Build static binary (smallest possible)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -buildid=" -trimpath -o server ./cmd/api


# ── Runtime stage (SMALLEST) ────────────────
FROM scratch

WORKDIR /

# SSL certificates (required for HTTPS calls)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /app/server /server

# Copy migrations (optional, remove if not needed)
COPY --from=builder /app/migrations /migrations

# Expose port
EXPOSE 8080

# Run app
ENTRYPOINT ["/server"]