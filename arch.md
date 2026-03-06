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
│   │   ├── profile.go                # ProfileController + profileRepo interface
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
- **`UserController`**: Orchestrates user registration — validates input, hashes password with Argon2ID, calls the repository, returns a domain `User`. JWTs use the user's immutable integer `ID` as the `sub` claim (stored as a decimal string per the JWT spec).
- **`userRepo` interface**: Decouples domain from persistence. The DB adapter implements this. Key methods: `InsertUser`, `GetUserByEmail`, `GetUserByID`, `GetFullUserByID`, `UpdateUser`.
- **`ProfileController`**: Handles profile lookups. Always returns `following: false` on this branch (the `follows` table is not present; follow/unfollow support will be merged from the `8-follow-user` branch).
- **`profileRepo` interface**: Decouples profile domain from persistence. Method: `GetProfileByUsername(profileUsername string)`.
- **`ValidationError`**: Structured field-level validation errors for HTTP consumers.
- **`DuplicateError`**: Error type returned when a unique constraint is violated. Carries the `Field` name and a fixed message (`"has already been taken"`).
- **`CredentialsError`**: Error type returned when login credentials are invalid (wrong password or unknown email).
- **`ProfileNotFoundError`**: Error type returned when a profile lookup finds no matching user.

### Inbound Adapter — HTTP (`internal/adapters/in/webserver/`)
Handles the HTTP protocol layer:
- **Router**: Gorilla Mux with Gorilla Handlers for Apache-style request logging. Protected routes are grouped in a subrouter with `authMiddleware` applied.
- **Middleware**: `authMiddleware(jwtSecret)` extracts the JWT from the `Authorization: Token {jwt}` header (401 if missing), validates the signature and expiry (401 if invalid), parses the `sub` claim as an integer user ID (401 if not a valid integer), and stores it in the request context under `userIDKey`. Protected handlers read the user ID from context and pass it directly to the domain service. `optionalAuthMiddleware(jwtSecret)` performs the same validation but silently ignores absent or invalid tokens — used for routes where authentication is optional.
- **Handlers**: Decode JSON, map to domain models, call domain services, encode responses.
- **DTOs**: `RegisterUserRequest` / `UserResponse` wrap payloads in `{"user": {...}}` per the RealWorld spec.

**Current routes:**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/users` | Register a new user |
| POST | `/api/users/login` | Log in an existing user |
| GET | `/api/user` | Get the authenticated user |
| PUT | `/api/user` | Update the authenticated user |
| GET | `/api/profiles/{username}` | Get a user's public profile (auth optional) |

**Response codes:** `200 OK`, `201 Created`, `401 Unauthorized`, `409 Conflict`, `422 Unprocessable Entity`, `500 Internal Server Error`

### Outbound Adapter — Database (`internal/adapters/out/db/`)
PostgreSQL persistence via `sqlx`:
- Runs embedded Goose migrations automatically on startup.
- Parameterized queries to prevent SQL injection.
- `InsertUser()` inserts a user and returns the created record via `RETURNING`. Detects PostgreSQL unique-violation errors (code `23505`) and maps them to `*domain.DuplicateError` by constraint name.
- `GetUserByEmail()` fetches a user and their hashed password by email. Returns `*domain.CredentialsError` when no row is found.
- `GetUserByID()` fetches a user by ID. Returns `*domain.CredentialsError` when no row is found.
- `GetFullUserByID()` fetches a user and their hashed password by ID. Returns `*domain.CredentialsError` when no row is found.
- `UpdateUser()` updates all user fields by user ID and returns the updated record via `RETURNING`. Maps unique-violation errors to `*domain.DuplicateError`.
- `GetProfileByUsername(profileUsername)` fetches a user's public profile fields by username. Always returns `following=false` on this branch. Returns `*domain.ProfileNotFoundError` when no row is found.

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

The project implements user **registration**, **login**, **get current user**, **update current user**, and **get profile**. Notable gaps:
- Registered and logged-in users receive a signed HS256 JWT (claims: `sub`=user ID as decimal string, 72h expiry). Using the immutable user ID means tokens remain valid even if the user changes their username.
- `GET /api/user` and `PUT /api/user` are protected by `authMiddleware`, which centralises both token extraction and JWT validation. The domain service receives the authenticated user ID (int) directly and no longer handles tokens.
- `PUT /api/user` supports partial updates (all fields optional); fetches current values, applies changes, and writes all fields back in one query.
- Future protected routes can be added to the protected subrouter with a single line; optionally-authenticated routes go on the optional-auth subrouter.
- `GET /api/profiles/{username}` always returns `following: false`; follow/unfollow endpoints are not yet built.
- Articles, comments, and other RealWorld endpoints are not yet built.
