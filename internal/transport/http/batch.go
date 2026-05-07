package transport

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/service"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/middleware"
	"github.com/golang-jwt/jwt/v5"
)

func CreateBatchHandler(bs *service.BatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("CreateBatchHandler called") //
		var req domain.CreateBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		// из токена достаем будущий Token.RegisteredBy
		raw := r.Context().Value(middleware.TokenKey)
		claims, ok := raw.(jwt.MapClaims) // проверка токена была в middleware, дополнительная зашита на случай измены роутинга
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		registeredBy, ok := claims["user_id"].(float64)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Printf("User ID from token: %v\n", registeredBy) //

		resp, err := bs.CreateBatch(r.Context(), req, int(registeredBy))
		if err != nil {
			if errors.Is(err, domain.ErrRecipeNotFound) {
				http.Error(w, "recipe not found", http.StatusNotFound)
				return
			}
			if errors.Is(err, domain.ErrRecipeArchived) {
				http.Error(w, "recipe archived", http.StatusNotFound) // скрытие информации о доступных и архивированных рецептурах
				return
			}
			if errors.Is(err, domain.ErrInvalidBatchVolume) {
				http.Error(w, "invalid batch volume", http.StatusBadRequest)
				return
			}
			http.Error(w, "failed to create batch", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

func ListBatchesByStatusHandler(bs *service.BatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")

		if status == "" {
			http.Error(w, "query parameter is required", http.StatusBadRequest)
			return
		}

		batches, err := bs.GetByStatus(r.Context(), status)
		if err != nil {
			http.Error(w, "failed to list batches", http.StatusInternalServerError)
		}
		resp := make([]domain.GetBatchesByStatusResponse, len(batches))
		for i, b := range batches {
			resp[i] = domain.GetBatchesByStatusResponse{
				ID:            b.ID,
				BatchCode:     b.Code,
				RecipeCode:    b.RecipeCode,
				TargetVolumeL: b.TargetVolumeL,
				BatchStatus:   b.Status,
				RegisteredBy:  b.RegisteredByCode,
				CreatedAt:     b.CreatedAt,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
