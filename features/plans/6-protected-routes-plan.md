# Plan: Protected Routes (Feature 6)

## Overview

Extract the duplicated `Authorization` header parsing from `GetUser` and `UpdateUser` into a reusable middleware. Future protected routes will use the same middleware without any additional boilerplate.

## Current duplication

Both `GetUser` and `UpdateUser` open with identical code:

```go
authHeader := r.Header.Get("Authorization")
w.Header().Set("Content-Type", "application/json")

const prefix = "Token "
if authHeader == "" || !strings.HasPrefix(authHeader, prefix) {
    w.WriteHeader(http.StatusUnauthorized)
    _, _ = w.Write(createErrResponse("token", []string{"is missing"}))
    return
}
rawToken := strings.TrimPrefix(authHeader, prefix)
```

## Design

The middleware handles only the "token is missing" case (absent/malformed `Authorization` header). JWT signature validation and user lookup remain in the domain service, unchanged.

The raw token string is stored in the request context so protected handlers can retrieve it without touching the header.

### Context key

A package-private type is used for the context key to avoid collisions with other packages:

```go
type contextKey string
const tokenKey contextKey = "token"
```

### Middleware behaviour

```
request arrives
│
├── Authorization header missing or lacks "Token " prefix?
│     └── 401  {"errors": {"token": ["is missing"]}}
│
└── Extract raw JWT, store in context, call next handler
```

### Router setup

A Gorilla Mux subrouter groups protected routes and applies the middleware once:

```go
r.HandleFunc("/api/users", h.RegisterUser).Methods("POST")
r.HandleFunc("/api/users/login", h.LoginUser).Methods("POST")

protected := r.NewRoute().Subrouter()
protected.Use(authMiddleware)
protected.HandleFunc("/api/user", h.GetUser).Methods("GET")
protected.HandleFunc("/api/user", h.UpdateUser).Methods("PUT")
```

Adding a future protected route requires only one line in the `protected` subrouter.

## Changes required

### 1. New file `internal/adapters/in/webserver/middleware.go`

- Define `contextKey` type and `tokenKey` constant.
- Implement `authMiddleware(next http.Handler) http.Handler`:
  - Read the `Authorization` header.
  - If empty or does not start with `"Token "`: write `Content-Type: application/json`, 401, `{"errors": {"token": ["is missing"]}}`, return.
  - Strip the prefix, store the raw JWT in context via `r.WithContext`.
  - Call `next.ServeHTTP(w, r)` with the updated request.

### 2. `internal/adapters/in/webserver/handlers.go`

- **`GetUser`**: remove the Authorization header block; retrieve the token from context with `r.Context().Value(tokenKey).(string)`.
- **`UpdateUser`**: same removal and context retrieval.
- The `strings` import can be removed if it is no longer used elsewhere (it is only used in the two removed blocks).

### 3. `internal/adapters/in/webserver/server.go`

- After registering the public routes, create a protected subrouter:
  ```go
  protected := r.NewRoute().Subrouter()
  protected.Use(authMiddleware)
  ```
- Move `GET /api/user` and `PUT /api/user` registrations to `protected`.

### 4. `arch.md`

- Add a note about the authentication middleware to the inbound adapter section.
- Update Current State.

## Order of implementation

1. Create `middleware.go` with `authMiddleware`.
2. Update `GetUser` and `UpdateUser` in `handlers.go` to read the token from context.
3. Update `server.go` to use a protected subrouter.
4. Run `make lint` and fix any errors.
5. Update `arch.md`.
