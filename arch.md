# Architecture

## Overview

A [RealWorld](https://github.com/gothinkster/realworld) backend implementation in Go using **Hexagonal Architecture** (Ports & Adapters). Business logic is isolated in a domain layer, with adapters for HTTP input and PostgreSQL output.

## Project Structure

```
realworld-backend-go/
├── cmd/server/server.go              # Entry point
├── internal/
│   ├── domain/                       # Business logic (no external dependencies)
│   │   ├── models.go                 # Core data models
│   │   ├── user.go                   # UserController + userRepo interface
│   │   └── errors.go                 # ValidationError type
│   └── adapters/
│       ├── in/webserver/             # Inbound: HTTP
│       │   ├── server.go             # Gorilla Mux router setup
│       │   └── handlers.go           # HTTP request/response handling
│       └── out/db/                   # Outbound: PostgreSQL
│           ├── postgres.go           # sqlx-based repository
│           └── migrations/
│               ├── 001_create_users.sql
│               └── 002_unique_users.sql
├── tests/integration/api_test.go     # Integration tests
├── compose.yaml                      # Docker Compose (prod + test DBs)
├── .env                              # Production environment config
└── .env_test                         # Test environment config
```

## Layers

### Domain (`internal/domain/`)
Pure Go with no framework dependencies. Contains:
- **`UserController`**: Orchestrates user registration — validates input, hashes password with Argon2ID, calls the repository, returns a domain `User`.
- **`userRepo` interface**: Decouples domain from persistence. The DB adapter implements this.
- **`ValidationError`**: Structured field-level validation errors for HTTP consumers.
- **`DuplicateError`**: Error type returned when a unique constraint is violated. Carries the `Field` name and a fixed message (`"has already been taken"`).

### Inbound Adapter — HTTP (`internal/adapters/in/webserver/`)
Handles the HTTP protocol layer:
- **Router**: Gorilla Mux with Gorilla Handlers for Apache-style request logging.
- **Handlers**: Decode JSON, map to domain models, call domain services, encode responses.
- **DTOs**: `RegisterUserRequest` / `UserResponse` wrap payloads in `{"user": {...}}` per the RealWorld spec.

**Current routes:**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/users` | Register a new user |

**Response codes:** `201 Created`, `409 Conflict`, `422 Unprocessable Entity`, `500 Internal Server Error`

### Outbound Adapter — Database (`internal/adapters/out/db/`)
PostgreSQL persistence via `sqlx`:
- Runs embedded Goose migrations automatically on startup.
- Parameterized queries to prevent SQL injection.
- `InsertUser()` inserts a user and returns the created record via `RETURNING`. Detects PostgreSQL unique-violation errors (code `23505`) and maps them to `*domain.DuplicateError` by constraint name.

**Schema (`users` table):**
| Column | Type | Notes |
|--------|------|-------|
| id | SERIAL | Primary key |
| username | VARCHAR(45) | Required, unique |
| email | VARCHAR(45) | Required, unique |
| password | VARCHAR(100) | Argon2ID hash |
| bio | TEXT | Optional |
| image | VARCHAR(100) | Optional (profile picture URL) |

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `gorilla/mux` | HTTP routing |
| `gorilla/handlers` | HTTP request logging |
| `jmoiron/sqlx` | SQL toolkit with row scanning |
| `lib/pq` | PostgreSQL driver |
| `pressly/goose/v3` | Database migrations |
| `alexedwards/argon2id` | Password hashing |
| `golang-jwt/jwt/v5` | JWT token generation (HS256) |
| `joho/godotenv` | `.env` file loading |
| `stretchr/testify` | Test assertions |

## Configuration

Loaded from `.env` / `.env_test` via `godotenv`:

| Variable | Default | Notes |
|----------|---------|-------|
| `SERVER_PORT` | 8090 | HTTP server port (test: 8091) |
| `JWT_SECRET` | — | HMAC signing key for JWT tokens |
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 8095 | PostgreSQL port (test: 8096) |
| `DB_USER` | admin | |
| `DB_PASSWORD` | password | |
| `DB_NAME` | app | Database name (test: test-app) |

## Infrastructure (Docker)

`compose.yaml` defines two PostgreSQL containers:
- **`db`** (port 8095): Production database, volume `app-data`
- **`test_db`** (port 8096): Test database, volume `app-test-data`

Migrations run automatically at app startup via embedded Goose files.

## Testing

Integration tests in `tests/integration/api_test.go`:
- Load `.env_test` config
- Connect to the test PostgreSQL container
- Start an `httptest.Server` with the full handler stack
- Send real HTTP requests and assert responses

## Current State

The project implements user **registration** only. Notable gaps:
- Registered users receive a signed HS256 JWT (claims: `sub`=username, 72h expiry).
- No authentication middleware for protected routes.
- Only one API endpoint exists; login, articles, comments, etc. are not yet built.
