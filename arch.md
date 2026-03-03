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
├── compose.yaml                      # Docker Compose (prod DB)
├── compose.test.yaml                 # Docker Compose (test DB)
├── Makefile                          # make int-tests runner
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
- **`CredentialsError`**: Error type returned when login credentials are invalid (wrong password or unknown email).

### Inbound Adapter — HTTP (`internal/adapters/in/webserver/`)
Handles the HTTP protocol layer:
- **Router**: Gorilla Mux with Gorilla Handlers for Apache-style request logging.
- **Handlers**: Decode JSON, map to domain models, call domain services, encode responses.
- **DTOs**: `RegisterUserRequest` / `UserResponse` wrap payloads in `{"user": {...}}` per the RealWorld spec.

**Current routes:**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/users` | Register a new user |
| POST | `/api/users/login` | Log in an existing user |
| GET | `/api/user` | Get the authenticated user |

**Response codes:** `200 OK`, `201 Created`, `401 Unauthorized`, `409 Conflict`, `422 Unprocessable Entity`, `500 Internal Server Error`

### Outbound Adapter — Database (`internal/adapters/out/db/`)
PostgreSQL persistence via `sqlx`:
- Runs embedded Goose migrations automatically on startup.
- Parameterized queries to prevent SQL injection.
- `InsertUser()` inserts a user and returns the created record via `RETURNING`. Detects PostgreSQL unique-violation errors (code `23505`) and maps them to `*domain.DuplicateError` by constraint name.
- `GetUserByEmail()` fetches a user and their hashed password by email. Returns `*domain.CredentialsError` when no row is found.
- `GetUserByUsername()` fetches a user by username. Returns `*domain.CredentialsError` when no row is found.

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

Docker Compose is split into two files:
- **`compose.yaml`**: Production database (`db`, port 8095, volume `app-data`)
- **`compose.test.yaml`**: Test database (`test_db`, port 8096, volume `app-test-data`)

Migrations run automatically at app startup via embedded Goose files.

## Testing

Integration tests are run via `make int-tests`, which:
1. Starts the test DB (`compose.test.yaml`)
2. Polls `pg_isready` until the DB is accepting connections
3. Builds the binary (`go build ./cmd/server`)
4. Starts the server with the test env (`./server -env .env_test`)
5. Runs the hurl API test suite (`../realworld/specs/api/run-api-tests-hurl.sh`)
6. Truncates the `users` table and stops the test DB container

The server accepts a `-env` flag (default `.env`) to select the env file at startup.

## Current State

The project implements user **registration**, **login**, and **get current user**. Notable gaps:
- Registered and logged-in users receive a signed HS256 JWT (claims: `sub`=username, 72h expiry).
- `GET /api/user` validates the JWT from the `Authorization: Token {jwt}` header, looks up the user by username, and returns a fresh token.
- No authentication middleware for protected routes.
- Articles, comments, and other RealWorld endpoints are not yet built.
