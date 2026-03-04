package webserver

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const tokenKey contextKey = "token"

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")

		const prefix = "Token "
		if authHeader == "" || !strings.HasPrefix(authHeader, prefix) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write(createErrResponse("token", []string{"is missing"}))
			return
		}

		rawToken := strings.TrimPrefix(authHeader, prefix)
		ctx := context.WithValue(r.Context(), tokenKey, rawToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
