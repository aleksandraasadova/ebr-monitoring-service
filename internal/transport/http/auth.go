package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type authService interface {
	Login(ctx context.Context, username, password string) (*domain.User, string, error)
}

type AuthHandler struct {
	svc authService
}

func NewAuthHandler(svc authService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Login godoc
// @Summary      Авторизация
// @Description  Вход по username/password, возвращает JWT-токен
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "учётные данные"
// @Success      200 {object} LoginResponse
// @Failure      400 {string} string "bad request"
// @Failure      401 {string} string "invalid login or password"
// @Failure      403 {string} string "account disabled"
// @Failure      500 {string} string "internal server error"
// @Router       /api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	user, token, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		switch err {
		case domain.ErrNoUserFound, domain.ErrWrongPassword:
			http.Error(w, "invalid login or password", http.StatusUnauthorized)
		case domain.ErrUserNotActive:
			http.Error(w, "account disabled", http.StatusForbidden)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token:    token,
		Role:     string(user.Role),
		UserCode: user.UserCode,
		UserName: user.UserName,
		FullName: user.FullName,
		IsActive: user.IsActive,
	})
}
