package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type userService interface {
	Create(ctx context.Context, role domain.UserRole, surname, name, fatherName string) (*domain.User, error)
}

type UserHandler struct {
	svc userService
}

func NewUserHandler(svc userService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Create godoc
// @Summary      Создать пользователя
// @Description  Создаёт нового пользователя (роль admin или operator). Доступно только админу.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body CreateUserRequest true "данные пользователя"
// @Success      201 {object} CreateUserResponse
// @Failure      400 {string} string "validation error"
// @Failure      401 {string} string "unauthorized"
// @Failure      403 {string} string "forbidden / invalid role"
// @Failure      409 {string} string "user already exists"
// @Failure      500 {string} string "internal server error"
// @Router       /api/v1/users [post]
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	user, err := h.svc.Create(r.Context(), domain.UserRole(req.Role), req.Surname, req.Name, req.FatherName)
	if err != nil {
		switch err {
		case domain.ErrInvalidRole:
			http.Error(w, "invalid role", http.StatusForbidden)
		case domain.ErrFullNameRequired:
			http.Error(w, "full name required", http.StatusBadRequest)
		case domain.ErrUserExists:
			http.Error(w, "user already exists", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateUserResponse{
		UserCode: user.UserCode,
		UserName: user.UserName,
	})
}
