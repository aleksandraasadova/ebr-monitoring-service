package transport

import (
	"encoding/json"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/service"
)

// Из исходного кода net/http
// type HandlerFunc func(http.ResponseWriter, *http.Request)

func LoginHandler(svc *service.AuthService) http.HandlerFunc {
	//замыкание?
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
				http.Error(w, "internal server error", http.StatusInternalServerError) // скорее всего с генерацией
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
