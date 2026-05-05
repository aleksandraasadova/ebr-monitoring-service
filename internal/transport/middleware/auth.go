package middleware

import (
	"context"
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

type Key string

const TokenKey Key = "token_claims"

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

		ctx := context.WithValue(r.Context(), TokenKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRole(requireRole ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Context().Value(TokenKey) // raw - интерфейс и нельзя обратиться к полям внутри
			//fmt.Println(raw)
			claims, ok := raw.(jwt.MapClaims)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			userRole, ok := claims["role"].(string) // type assertion
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			for _, role := range requireRole {
				if role == userRole {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}
