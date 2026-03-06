# Plan: Follow User (Feature 8)

## Overview

Add two protected endpoints (`POST` and `DELETE /api/profiles/{username}/follow`) and update `GET /api/profiles/{username}` to return the correct `following` value based on whether the authenticated viewer follows the profile user.

---

## 1. DB Migration

**File:** `internal/adapters/out/db/migrations/003_follows.sql`

Create a `follows` table that tracks follower/followee relationships. Storing IDs (not usernames) keeps the table in 4NF and avoids update anomalies when usernames change.

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS follows (
    follower_id INTEGER NOT NULL REFERENCES users(id),
    followee_id INTEGER NOT NULL REFERENCES users(id),
    PRIMARY KEY (follower_id, followee_id)
);

-- +goose Down
DROP TABLE IF EXISTS follows;
```

---

## 2. Domain Layer (`internal/domain/profile.go`)

### 2a. Update `profileRepo` interface

- Change `GetProfileByUsername(ctx context.Context, username string)` → `GetProfileByUsername(ctx context.Context, profileUsername string, viewerUsername string)`
  (Pass the viewer username so the repo can compute `following`; empty string means unauthenticated.)
- Add `FollowUser(ctx context.Context, followerUsername, followeeUsername string) error`
- Add `UnfollowUser(ctx context.Context, followerUsername, followeeUsername string) error`

### 2b. Update `ProfileController`

- **`GetProfile`**: pass `viewerUsername` through to `repo.GetProfileByUsername`.
- **Add `FollowUser(ctx, followerUsername, followeeUsername string) (*Profile, error)`**:
  1. Call `repo.FollowUser(ctx, followerUsername, followeeUsername)`.
  2. On success, call `repo.GetProfileByUsername(ctx, followeeUsername, followerUsername)` and return result.
- **Add `UnfollowUser(ctx, followerUsername, followeeUsername string) (*Profile, error)`**:
  1. Call `repo.UnfollowUser(ctx, followerUsername, followeeUsername)`.
  2. On success, call `repo.GetProfileByUsername(ctx, followeeUsername, followerUsername)` and return result.

---

## 3. DB Adapter (`internal/adapters/out/db/postgres.go`)

### 3a. Update `GetProfileByUsername`

Change signature to `(ctx, profileUsername, viewerUsername string)` and update the SQL to compute the `following` field via a LEFT JOIN:

```sql
SELECT u.username, u.bio, u.image,
    CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following
FROM users u
LEFT JOIN follows f
    ON f.followee_id = u.id
    AND f.follower_id = (SELECT id FROM users WHERE username = $2)
WHERE u.username = $1
```

When `viewerUsername` is empty, the subquery returns NULL, the LEFT JOIN finds no match, and `following` is false — correct for unauthenticated callers.

Update the local `user` struct to add a `Following bool` field, or use a dedicated struct for this query.

### 3b. Implement `FollowUser`

```
func (p *Postgres) FollowUser(ctx, followerUsername, followeeUsername string) error
```

1. Check followee exists:
   ```sql
   SELECT id FROM users WHERE username = $1
   ```
   If no row → return `&domain.ProfileNotFoundError{}`.
2. Insert the follow relationship (idempotent via `ON CONFLICT DO NOTHING`):
   ```sql
   INSERT INTO follows (follower_id, followee_id)
   SELECT f.id, e.id FROM users f, users e
   WHERE f.username = $1 AND e.username = $2
   ON CONFLICT DO NOTHING
   ```

### 3c. Implement `UnfollowUser`

```
func (p *Postgres) UnfollowUser(ctx, followerUsername, followeeUsername string) error
```

1. Check followee exists (same SELECT as above) → `ProfileNotFoundError` if missing.
2. Delete the follow relationship (idempotent if row absent):
   ```sql
   DELETE FROM follows
   WHERE follower_id = (SELECT id FROM users WHERE username = $1)
     AND followee_id = (SELECT id FROM users WHERE username = $2)
   ```

---

## 4. HTTP Handlers (`internal/adapters/in/webserver/handlers.go`)

### 4a. Update `profileService` interface

Add:
```go
FollowUser(ctx context.Context, followerUsername, followeeUsername string) (*domain.Profile, error)
UnfollowUser(ctx context.Context, followerUsername, followeeUsername string) (*domain.Profile, error)
```

### 4b. Add `FollowUser` handler

- Extract authenticated username from context (`usernameKey`).
- Extract `{username}` path variable (the profile to follow).
- Call `h.profileService.FollowUser(ctx, authUsername, profileUsername)`.
- On `ProfileNotFoundError` → 404 with `{"errors": {"profile": ["not found"]}}`.
- On success → 200 with `ProfileResponse`.

### 4c. Add `UnfollowUser` handler

- Same shape as `FollowUser` handler, calls `UnfollowUser`, returns 200 with `ProfileResponse`.

---

## 5. HTTP Routing (`internal/adapters/in/webserver/server.go`)

### 5a. Update `ServerHandlers` interface

Add:
```go
FollowUser(http.ResponseWriter, *http.Request)
UnfollowUser(http.ResponseWriter, *http.Request)
```

### 5b. Register routes

Add to the `protected` subrouter:
```go
protected.HandleFunc("/api/profiles/{username}/follow", h.FollowUser).Methods("POST")
protected.HandleFunc("/api/profiles/{username}/follow", h.UnfollowUser).Methods("DELETE")
```

---

## 6. Update `Makefile` (`int-tests` target)

The `follows` table has FK references to `users`, so it must be cleared before `users` is truncated. Update the cleanup step to truncate both tables in dependency order:

```makefile
docker compose -f compose.test.yaml exec -T test_db psql -U admin -d test-app -c "TRUNCATE TABLE follows, users;"; \
```

---

## 7. Update `arch.md`

- Add `003_follows.sql` migration to the project structure listing.
- Add `follows` table schema.
- Add two new routes to the routes table.
- Update `GetProfileByUsername()` description to include the viewer username and `following` computation.
- Document `FollowUser()` and `UnfollowUser()` on the `Postgres` struct.
- Remove the "always returns `following: false`" note from Current State.
