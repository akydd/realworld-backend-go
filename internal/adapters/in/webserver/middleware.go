package webserver

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const usernameKey contextKey = "username"

func optionalAuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			const prefix = "Token "
			if authHeader != "" && strings.HasPrefix(authHeader, prefix) {
				rawToken := strings.TrimPrefix(authHeader, prefix)
				token, err := jwt.ParseWithClaims(rawToken, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, jwt.ErrSignatureInvalid
					}
					return []byte(jwtSecret), nil
				})
				if err == nil && token.Valid {
					if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && claims.Subject != "" {
						ctx := context.WithValue(r.Context(), usernameKey, claims.Subject)
						r = r.WithContext(ctx)
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func authMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			authHeader := r.Header.Get("Authorization")
			const prefix = "Token "
			if authHeader == "" || !strings.HasPrefix(authHeader, prefix) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write(createErrResponse("token", []string{"is missing"}))
				return
			}

			rawToken := strings.TrimPrefix(authHeader, prefix)

			token, err := jwt.ParseWithClaims(rawToken, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
				return
			}

			claims, ok := token.Claims.(*jwt.RegisteredClaims)
			if !ok || claims.Subject == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
				return
			}

			ctx := context.WithValue(r.Context(), usernameKey, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
