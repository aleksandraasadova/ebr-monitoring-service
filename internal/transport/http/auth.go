package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type authenticator interface {
	Login(ctx context.Context, req domain.LoginRequest) (*domain.LoginResponse, error)
}

// LoginHandler godoc
// @Summary      Авторизация
// @Description  Вход по username/password, возвращает JWT-токен
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body domain.LoginRequest true "учётные данные"
// @Success      200 {object} domain.LoginResponse
// @Failure      400 {string} string "bad request"
// @Failure      401 {string} string "invalid login or password"
// @Failure      403 {string} string "account disabled"
// @Failure      500 {string} string "internal server error"
// @Router       /api/v1/auth/login [post]
func LoginHandler(svc authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req domain.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.Password == "" {
			http.Error(w, "username and password are required", http.StatusBadRequest)
			return
		}

		resp, err := svc.Login(r.Context(), req)
		if err != nil {
			switch err {
			case domain.ErrNoUserFound:
				http.Error(w, "invalid login or password", http.StatusUnauthorized)
				return
			case domain.ErrWrongPassword:
				http.Error(w, "invalid login or password", http.StatusUnauthorized)
				return
			case domain.ErrUserNotActive:
				http.Error(w, "account disabled", http.StatusForbidden)
				return
			default:
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
