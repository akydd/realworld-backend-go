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
│   │   ├── article.go                # ArticleController + articleRepo interface
│   │   ├── tag.go                    # TagController + tagRepo interface
│   │   └── errors.go                 # ValidationError type
│   └── adapters/
│       ├── in/webserver/             # Inbound: HTTP
│       │   ├── server.go             # Gorilla Mux router setup
│       │   └── handlers.go           # HTTP request/response handling
│       └── out/db/                   # Outbound: PostgreSQL
│           ├── postgres.go           # sqlx-based repository
│           └── migrations/
│               ├── 001_create_users.sql
│               ├── 002_unique_users.sql
│               ├── 003_create_follows.sql
│               ├── 004_create_articles.sql
│               └── 005_create_tags.sql
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
- **`ProfileController`**: Handles profile lookups and follow/unfollow operations. Methods: `GetProfile(ctx, profileUsername, viewerID)`, `FollowUser(ctx, followerID, followeeUsername)`, `UnfollowUser(ctx, followerID, followeeUsername)`. Returns actual `following` status.
- **`profileRepo` interface**: Decouples profile domain from persistence. Methods: `GetProfileByUsername(ctx, profileUsername, viewerID)`, `FollowUser(ctx, followerID, followeeUsername)`, `UnfollowUser(ctx, followerID, followeeUsername)`.
- **`ValidationError`**: Structured field-level validation errors for HTTP consumers.
- **`DuplicateError`**: Error type returned when a unique constraint is violated. Carries the `Field` name and a fixed message (`"has already been taken"`).
- **`CredentialsError`**: Error type returned when login credentials are invalid (wrong password or unknown email).
- **`ProfileNotFoundError`**: Error type returned when a profile lookup finds no matching user.
- **`ArticleNotFoundError`**: Error type returned when an article lookup finds no matching article.
- **`ArticleController`**: Handles article creation, retrieval, and updates. Validates input, deduplicates tags (first-occurrence wins), generates slug from title via exported `GenerateSlug(title)` (kebab-case regex). Methods: `CreateArticle(ctx, authorID, a)`, `GetArticleBySlug(ctx, slug, viewerID)`, `UpdateArticle(ctx, callerID, slug, u)`.
- **`articleRepo` interface**: Decouples article domain from persistence. Methods: `InsertArticle(ctx, authorID, slug, a)`, `GetArticleBySlug(ctx, slug, viewerID)`, `UpdateArticle(ctx, callerID, slug, u)`.
- **`TagController`**: Handles tag listing. Method: `GetTags(ctx)`.
- **`tagRepo` interface**: Decouples tag domain from persistence. Method: `GetAllTags(ctx)`.

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
| POST | `/api/profiles/{username}/follow` | Follow a user (auth required) |
| DELETE | `/api/profiles/{username}/follow` | Unfollow a user (auth required) |
| POST | `/api/articles` | Create an article (auth required) |
| GET | `/api/articles/{slug}` | Get an article by slug (auth optional) |
| PUT | `/api/articles/{slug}` | Update an article (auth required, author only) |
| GET | `/api/tags` | List all tags (no auth) |

**Response codes:** `200 OK`, `201 Created`, `401 Unauthorized`, `404 Not Found`, `409 Conflict`, `422 Unprocessable Entity`, `500 Internal Server Error`

### Outbound Adapter — Database (`internal/adapters/out/db/`)
PostgreSQL persistence via `sqlx`:
- Runs embedded Goose migrations automatically on startup.
- Parameterized queries to prevent SQL injection.
- `InsertUser()` inserts a user and returns the created record via `RETURNING`. Detects PostgreSQL unique-violation errors (code `23505`) and maps them to `*domain.DuplicateError` by constraint name.
- `GetUserByEmail()` fetches a user and their hashed password by email. Returns `*domain.CredentialsError` when no row is found.
- `GetUserByID()` fetches a user by ID. Returns `*domain.CredentialsError` when no row is found.
- `GetFullUserByID()` fetches a user and their hashed password by ID. Returns `*domain.CredentialsError` when no row is found.
- `UpdateUser()` updates all user fields by user ID and returns the updated record via `RETURNING`. Maps unique-violation errors to `*domain.DuplicateError`.
- `GetProfileByUsername(ctx, profileUsername, viewerID)` fetches a user's public profile fields by username using a LEFT JOIN on `follows` to compute the real `following` status for the viewer. Pass `viewerID=0` for unauthenticated requests. Returns `*domain.ProfileNotFoundError` when no row is found.
- `FollowUser(ctx, followerID, followeeUsername)` inserts a row into `follows` (idempotent via `ON CONFLICT DO NOTHING`) then calls `GetProfileByUsername` to return the full profile. Returns `*domain.ProfileNotFoundError` when the followee username does not exist.
- `UnfollowUser(ctx, followerID, followeeUsername)` deletes the corresponding `follows` row then calls `GetProfileByUsername` to return the full profile. Returns `*domain.ProfileNotFoundError` when the followee username does not exist.
- `InsertArticle(ctx, authorID, slug, a)` wraps all operations in a transaction. Inserts the article, upserts tags (via `INSERT ... ON CONFLICT DO NOTHING`), links tags to the article via `article_tags`, then fetches the author profile. Maps PostgreSQL unique-violation errors on `articles_title_unique` or `articles_slug_unique` to `*domain.DuplicateError{Field: "title"}`. Returns `TagList` from the (deduplicated) input; `Favorited` is always `false`, `FavoritesCount` is always `0`.
- `GetArticleBySlug(ctx, slug, viewerID)` fetches a single article by slug in one query: JOINs `users` for the author, LEFT JOINs `follows` for the `following` status (`viewerID=0` always yields `false`), and LEFT JOINs `article_tags`+`tags` with `ARRAY_AGG` to collect the tag list. Returns `*domain.ArticleNotFoundError` when no row is found.
- `UpdateArticle(ctx, callerID, slug, u)` wraps the update in a transaction: fetches the current article (→ `ArticleNotFoundError` if missing), checks `author_id == callerID` (→ `CredentialsError` if not), merges partial fields, recomputes slug via `domain.GenerateSlug` if title changed, runs `UPDATE`, maps unique-violation errors to `DuplicateError{Field: "title"}`, commits, then calls `GetArticleBySlug` to return the full response.
- `GetAllTags(ctx)` returns all tag names ordered alphabetically. Returns `[]string{}` (never nil) when there are no tags.

**Schema (`users` table):**
| Column | Type | Notes |
|--------|------|-------|
| id | SERIAL | Primary key |
| username | VARCHAR(45) | Required, unique |
| email | VARCHAR(45) | Required, unique |
| password | VARCHAR(100) | Argon2ID hash |
| bio | TEXT | Optional |
| image | VARCHAR(100) | Optional (profile picture URL) |

**Schema (`follows` table):**
| Column | Type | Notes |
|--------|------|-------|
| follower_id | INTEGER | FK → users.id, part of PK |
| followee_id | INTEGER | FK → users.id, part of PK |

**Schema (`articles` table):**
| Column | Type | Notes |
|--------|------|-------|
| id | SERIAL | Primary key |
| slug | VARCHAR(255) | Required, unique |
| title | VARCHAR(255) | Required, unique |
| description | TEXT | Required |
| body | TEXT | Required |
| author_id | INTEGER | FK → users.id |
| created_at | TIMESTAMPTZ | Auto-set to now() |
| updated_at | TIMESTAMPTZ | Auto-set to now() |

**Schema (`tags` table):**
| Column | Type | Notes |
|--------|------|-------|
| id | SERIAL | Primary key |
| name | VARCHAR(255) | Required, unique |

**Schema (`article_tags` table):**
| Column | Type | Notes |
|--------|------|-------|
| article_id | INTEGER | FK → articles.id ON DELETE CASCADE, part of PK |
| tag_id | INTEGER | FK → tags.id ON DELETE CASCADE, part of PK |

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

The project implements user **registration**, **login**, **get current user**, **update current user**, **get profile**, **follow user**, and **unfollow user**. Notable details:
- Registered and logged-in users receive a signed HS256 JWT (claims: `sub`=user ID as decimal string, 72h expiry). Using the immutable user ID means tokens remain valid even if the user changes their username.
- `GET /api/user` and `PUT /api/user` are protected by `authMiddleware`, which centralises both token extraction and JWT validation. The domain service receives the authenticated user ID (int) directly and no longer handles tokens.
- `PUT /api/user` supports partial updates (all fields optional); fetches current values, applies changes, and writes all fields back in one query.
- Future protected routes can be added to the protected subrouter with a single line; optionally-authenticated routes go on the optional-auth subrouter.
- `GET /api/profiles/{username}` returns the real `following` status for an authenticated viewer, or `false` for unauthenticated requests.
- `POST /api/profiles/{username}/follow` and `DELETE /api/profiles/{username}/follow` are protected endpoints that create/remove rows in the `follows` table.
- `POST /api/articles` creates an article; slug is generated from the title (kebab-case). `tagList` is stored in the `tags` and `article_tags` tables and returned in the response. `favorited` and `favoritesCount` are always `false`/`0`.
- `GET /api/tags` returns all tags ordered alphabetically.
- `GET /api/articles/{slug}` returns a single article by slug (auth optional); `author.following` reflects the viewer's follow state.
- `PUT /api/articles/{slug}` updates an article's title, description, and/or body (at least one required); title change regenerates the slug. Only the author may update; returns 401 otherwise.
- List, delete article and other RealWorld endpoints are not yet built.
