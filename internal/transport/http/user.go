package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type userCreator interface {
	Create(ctx context.Context, req domain.CreateUserRequest) (*domain.CreateUserResponse, error)
}

// CreateUserHandler godoc
// @Summary      Создать пользователя
// @Description  Создаёт нового пользователя (роль admin или operator). Доступно только админу.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body domain.CreateUserRequest true "данные пользователя"
// @Success      201 {object} domain.CreateUserResponse
// @Failure      400 {string} string "validation error"
// @Failure      401 {string} string "unauthorized"
// @Failure      403 {string} string "forbidden / invalid role"
// @Failure      409 {string} string "user already exists"
// @Failure      500 {string} string "internal server error"
// @Router       /api/v1/users [post]
func CreateUserHandler(svc userCreator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req domain.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if req.Role == "" || req.Surname == "" || req.Name == "" || req.FatherName == "" {
			http.Error(w, "role, surname, name and father name are required", http.StatusBadRequest)
			return
		}

		resp, err := svc.Create(r.Context(), req)
		if err != nil {
			switch err {
			case domain.ErrInvalidRole:
				http.Error(w, "invalid role", http.StatusForbidden)
				return
			case domain.ErrFullNameRequired:
				http.Error(w, "full name required", http.StatusBadRequest)
				return
			case domain.ErrUserExists:
				http.Error(w, "user already exists", http.StatusConflict)
				return
			default:
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}
