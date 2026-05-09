package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int
	Role   string
}

type ctxKey struct{}

var claimsCtxKey = ctxKey{}

func UserFromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsCtxKey).(Claims)
	return c, ok
}

func JWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "missing token/not expected type of token", http.StatusUnauthorized)
			return
		}

		raw := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(raw, func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		mapClaims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}

		userID, ok := mapClaims["user_id"].(float64)
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}
		role, ok := mapClaims["role"].(string)
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}

		claims := Claims{
			UserID: int(userID),
			Role:   role,
		}

		ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			for _, role := range roles {
				if user.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}
