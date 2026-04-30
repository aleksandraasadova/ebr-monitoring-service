package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// http.Handler - интерфейс для обработки HTTP-запросов
// http.Handler содержит метод ServeHTTP
// type HandlerFunc func(ResponseWriter, *Request)
// func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
//    f(w, r)
//}

// type Middleware func(http.Handler) http.Handler

type key string

const token_key key = "token_claims"

func JWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "missing token/not expected type of token", http.StatusUnauthorized)
			return
		}

		JWTToken := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(JWTToken, func(parsed_token *jwt.Token) (interface{}, error) { // anonymous func - callback
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		//fmt.Println(token.Raw, token.Method, token.Header, token.Claims, token.Signature, token.Valid)
		claims, ok := token.Claims.(jwt.MapClaims) // type assertion
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), token_key, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRole(requireRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Context().Value(token_key)
			fmt.Println(raw)
			claims, ok := raw.(jwt.MapClaims)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userRole := claims["role"].(string)
			if userRole != requireRole {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
