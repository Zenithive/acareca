# ── Build stage ─────────────────────────────
FROM golang:1.25-alpine3.21 AS builder

WORKDIR /app

# Install git (needed for some Go modules)
RUN apk add --no-cache git

# Cache Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Install swag for swagger docs generation
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Copy project source
COPY . .

# Generate swagger docs (creates /docs folder)
RUN swag init -g cmd/api/main.go

# Build optimized static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o server ./cmd/api


# ── Runtime stage ───────────────────────────
FROM alpine:3.21

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary
COPY --from=builder /app/server /app/server

# Copy migrations if required
COPY --from=builder /app/migrations /app/migrations

# Run as non-root user
RUN adduser -D appuser
USER appuser

EXPOSE 8080

CMD ["./server"]