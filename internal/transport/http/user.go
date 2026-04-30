package transport

import (
	"encoding/json"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/service"
)

func CreateUserHandler(svc *service.UserService) http.HandlerFunc {
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
