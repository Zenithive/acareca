# acareca

A Go backend service built with a modular monolith architecture, using Gin and PostgreSQL.

## Prerequisites

- [Go](https://golang.org/dl/) 1.25.3 or later
- [PostgreSQL](https://www.postgresql.org/) 14 or later
- [goose](https://github.com/pressly/goose) (for running migrations)

## Project Structure

```
backend/
├── cmd/
│   └── api/
│       └── main.go                        # Entrypoint — wires all modules together
├── internal/
│   ├── modules/
│   │   └── auth/
│   │       ├── model.go                   # Domain structs & request/response types
│   │       ├── repository.go              # DB queries (sqlx)
│   │       ├── service.go                 # Business logic
│   │       ├── handler.go                 # Gin HTTP handlers
│   │       └── routes.go                  # Route registration
│   └── shared/
│       ├── db/
│       │   └── db.go                      # Singleton DB connection (sqlx)
│       ├── middleware/
│       │   └── auth.go                    # JWT validation middleware
│       └── response/
│           └── response.go                # Uniform JSON response helpers
├── pkg/
│   └── config/
│       └── config.go                      # Config loaded from env vars
├── migrations/                            # goose SQL migrations
├── go.mod
└── go.sum
```

## Getting Started

### 1. Clone and install dependencies

```bash
git clone https://github.com/iamarpitzala/acareca.git
cd acareca
go mod tidy
```

### 2. Configure environment variables

Copy and edit the example env file:

```bash
cp .env.example .env
```

| Variable      | Default     | Description              |
|---------------|-------------|--------------------------|
| `DB_HOST`     | `localhost` | PostgreSQL host          |
| `DB_PORT`     | `5432`      | PostgreSQL port          |
| `DB_USER`     | `postgres`  | Database user            |
| `DB_PASSWORD` |             | Database password        |
| `DB_NAME`     | `acareca`   | Database name            |
| `DB_SSLMODE`  | `disable`   | SSL mode                 |
| `SERVER_PORT` | `8080`      | HTTP server port         |
| `JWT_SECRET`  | `change-me` | Secret for signing JWTs  |

### 3. Run migrations

```bash
goose -dir migrations postgres "host=$DB_HOST port=$DB_PORT user=$DB_USER password=$DB_PASSWORD dbname=$DB_NAME sslmode=$DB_SSLMODE" up
```

### 4. Run the server

```bash
go run ./cmd/api
```

## API Endpoints

| Method | Path                  | Description            | Auth required |
|--------|-----------------------|------------------------|---------------|
| GET    | `/health`             | Health check           | No            |
| POST   | `/api/v1/auth/register` | Register a new user  | No            |
| POST   | `/api/v1/auth/login`    | Login, returns tokens | No            |

### Register

```http
POST /api/v1/auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123",
  "first_name": "John",
  "last_name": "Doe",
  "phone": "+1234567890"
}
```

### Login

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

Response:

```json
{
  "data": {
    "access_token": "<jwt>",
    "refresh_token": "<jwt>",
    "is_superadmin": false
  }
}
```

### Protected routes

Pass the access token in the `Authorization` header:

```
Authorization: Bearer <access_token>
```

## Development

Build the binary:

```bash
go build -o acareca ./cmd/api
```

Run tests:

```bash
go test ./...
```

## Architecture

Each domain feature lives in its own package under `internal/modules/`. The layers within a module are:

```
handler  →  service  →  repository  →  database
```

Modules never import each other. Cross-cutting concerns (DB, middleware, response helpers) live in `internal/shared/`. Code with no internal dependencies lives in `pkg/`.

## License

MIT
