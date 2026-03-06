# Plan: Better JWT (Feature better-jwt)

## Overview

The JWT `sub` claim currently stores the username, which is mutable. Changing it to use the user ID (an immutable integer) means a renamed user's token remains valid. This requires propagating the user ID through the domain model, repository interfaces, DB queries, and HTTP middleware.

---

## 1. Domain Model (`internal/domain/models.go`)

Add `ID int` to the `User` struct.

---

## 2. Domain — User (`internal/domain/user.go`)

### 2a. Update `userRepo` interface

- Replace `GetUserByUsername(ctx, username string) (*User, error)` with `GetUserByID(ctx, id int) (*User, error)`.
- Replace `GetFullUserByUsername(ctx, username string) (*User, string, error)` with `GetFullUserByID(ctx, id int) (*User, string, error)`.
- Change `UpdateUser(ctx, currentUsername string, u *UpdateUserData)` → `UpdateUser(ctx, userID int, u *UpdateUserData)`.

### 2b. Update `generateToken`

Change signature from `generateToken(username string, secret string)` to `generateToken(id int, secret string)`. Store `strconv.Itoa(id)` in the `sub` claim.

### 2c. Update `GetUser`

Call `repo.GetUserByID(ctx, userID)` (receiving `userID int`) instead of `GetUserByUsername`.

### 2d. Update `UpdateUser`

Call `repo.GetFullUserByID(ctx, userID)` and `repo.UpdateUser(ctx, userID, &data)`.

### 2e. All `generateToken` call sites

Pass `user.ID` instead of `user.Username` in `RegisterUser`, `LoginUser`, `GetUser`, and `UpdateUser`.

---

## 3. Domain — Profile (`internal/domain/profile.go`)

### 3a. Update `profileRepo` interface

- `GetProfileByUsername(ctx, profileUsername string, viewerUsername string)` → `GetProfileByUsername(ctx, profileUsername string, viewerID int)`.
- `FollowUser(ctx, followerUsername, followeeUsername string)` → `FollowUser(ctx, followerID int, followeeUsername string)`.
- `UnfollowUser(ctx, followerUsername, followeeUsername string)` → `UnfollowUser(ctx, followerID int, followeeUsername string)`.

### 3b. Update `ProfileController` methods

- `GetProfile`: pass `viewerID int` (from caller) through to `repo.GetProfileByUsername`.
- `FollowUser`: accept `followerID int`; pass through to repo and to subsequent `GetProfileByUsername` call.
- `UnfollowUser`: same as `FollowUser`.

---

## 4. DB Adapter (`internal/adapters/out/db/postgres.go`)

### 4a. Update `user` struct

Add `ID int \`db:"id"\`` field.

### 4b. Update `convertUser`

Copy `u.ID` into the returned `domain.User`.

### 4c. Add `id` to all SELECT / RETURNING clauses

Update `InsertUser`, `GetUserByEmail`, and `UpdateUser` queries to include `id` in `RETURNING` / `SELECT`.

### 4d. Add `GetUserByID` and `GetFullUserByID`

```sql
-- GetUserByID
SELECT id, username, email, bio, image FROM users WHERE id = $1

-- GetFullUserByID
SELECT id, username, email, bio, image, password FROM users WHERE id = $1
```

Both return `*domain.CredentialsError` when no row is found.

### 4e. Remove `GetUserByUsername` and `GetFullUserByUsername`

These are no longer in the `userRepo` interface; remove the implementations.

### 4f. Update `UpdateUser`

Change `WHERE username = $6` → `WHERE id = $6` (passing `userID int`).

### 4g. Update `GetProfileByUsername`

Change signature to `(ctx, profileUsername string, viewerID int)`. Simplify the LEFT JOIN — no subquery needed, use `$2` directly:

```sql
SELECT u.username, u.bio, u.image,
    CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following
FROM users u
LEFT JOIN follows f
    ON f.followee_id = u.id
    AND f.follower_id = $2
WHERE u.username = $1
```

When `viewerID` is 0 (unauthenticated), no `follows` row has `follower_id = 0`, so `following` is always false.

### 4h. Update `FollowUser` and `UnfollowUser`

Change `followerUsername string` → `followerID int`. The follower ID is now known directly, so:

- `FollowUser`: remove the existence pre-check for the follower (ID comes from a validated JWT); existence check for followee stays. Simplify the INSERT:
  ```sql
  INSERT INTO follows (follower_id, followee_id)
  SELECT $1, id FROM users WHERE username = $2
  ON CONFLICT DO NOTHING
  ```
  Check rows affected: if 0, the followee does not exist → return `ProfileNotFoundError`.

- `UnfollowUser`: remove follower lookup. Simplify the DELETE:
  ```sql
  DELETE FROM follows
  WHERE follower_id = $1
    AND followee_id = (SELECT id FROM users WHERE username = $2)
  ```
  Check followee existence separately (same SELECT as before) when needed.

---

## 5. HTTP Middleware (`internal/adapters/in/webserver/middleware.go`)

- Rename `usernameKey` → `userIDKey` (context key, type `contextKey`).
- After validating the JWT, parse `claims.Subject` as an integer with `strconv.Atoi`. If parsing fails, treat as invalid token.
- Store the resulting `int` user ID in context under `userIDKey`.

Both `authMiddleware` and `optionalAuthMiddleware` need updating. For `optionalAuthMiddleware`, store user ID only when parsing succeeds; otherwise leave context unchanged (zero value 0 means unauthenticated).

---

## 6. HTTP Handlers (`internal/adapters/in/webserver/handlers.go`)

### 6a. Update `userService` interface

- `GetUser(ctx, username string)` → `GetUser(ctx, userID int)`
- `UpdateUser(ctx, username string, u *domain.UpdateUser)` → `UpdateUser(ctx, userID int, u *domain.UpdateUser)`

### 6b. Update `profileService` interface

- `GetProfile(ctx, profileUsername string, viewerUsername string)` → `GetProfile(ctx, profileUsername string, viewerID int)`
- `FollowUser(ctx, followerUsername, followeeUsername string)` → `FollowUser(ctx, followerID int, followeeUsername string)`
- `UnfollowUser(ctx, followerUsername, followeeUsername string)` → `UnfollowUser(ctx, followerID int, followeeUsername string)`

### 6c. Update handlers

Replace all `r.Context().Value(usernameKey).(string)` reads with `r.Context().Value(userIDKey).(int)` (protected handlers) or `r.Context().Value(userIDKey)` with a zero-value int fallback (optional-auth handlers).

---

## 7. Update `arch.md`

- Note that `User` model now includes `ID`.
- Update JWT description: `sub` claim now holds the user ID (integer as string).
- Update middleware description: extracts user ID (int) from `sub` and stores it in context.
- Update DB adapter descriptions for changed/added/removed methods.
