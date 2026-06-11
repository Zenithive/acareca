# ── Build stage ──────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

RUN go install github.com/swaggo/swag/cmd/swag@v1.8.12

COPY . .
RUN swag init -g cmd/api/main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-s -w -buildid=" \
      -trimpath \
      -o server \
      ./cmd/api


# ── Runtime stage ─────────────────────────────────────────────
FROM debian:bookworm-slim

WORKDIR /

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl gnupg ca-certificates \
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

RUN useradd -m -u 1001 appuser \
    && mkdir -p /home/appuser/.local/share/applications \
    && touch /home/appuser/.local/share/applications/mimeapps.list \
    && mkdir -p /tmp/chromedp-profile \
    && chown -R appuser:appuser /home/appuser /tmp/chromedp-profile \
    && chmod 1777 /tmp

COPY --from=builder /app/server /server
COPY --from=builder /app/migrations /migrations

USER appuser

EXPOSE 8080

ENTRYPOINT ["/server"]