package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/middleware"
)

type batchCreator interface {
	CreateBatch(ctx context.Context, req domain.CreateBatchRequest, registeredByID int) (*domain.CreateBatchResponse, error)
}

type batchLister interface {
	GetByStatus(ctx context.Context, status string) ([]domain.Batch, error)
}

// CreateBatchHandler godoc
// @Summary      Создать партию (batch)
// @Description  Создаёт партию по рецепту. registered_by берётся из JWT.
// @Tags         batches
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body domain.CreateBatchRequest true "данные партии"
// @Success      201 {object} domain.CreateBatchResponse
// @Failure      400 {string} string "invalid json or invalid batch volume"
// @Failure      401 {string} string "unauthorized"
// @Failure      404 {string} string "recipe not found or archived"
// @Failure      500 {string} string "failed to create batch"
// @Router       /api/v1/batches [post]
func CreateBatchHandler(svc batchCreator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req domain.CreateBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		user, ok := middleware.UserFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		resp, err := svc.CreateBatch(r.Context(), req, user.UserID)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrRecipeNotFound):
				http.Error(w, "recipe not found", http.StatusNotFound)
			case errors.Is(err, domain.ErrRecipeArchived):
				http.Error(w, "recipe archived", http.StatusNotFound)
			case errors.Is(err, domain.ErrInvalidBatchVolume):
				http.Error(w, "invalid batch volume", http.StatusBadRequest)
			default:
				http.Error(w, "failed to create batch", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

// ListBatchesByStatusHandler godoc
// @Summary      Список партий по статусу
// @Tags         batches
// @Produce      json
// @Security     BearerAuth
// @Param        status query string true "статус партии"
// @Success      200 {array} domain.GetBatchesByStatusResponse
// @Failure      400 {string} string "query parameter is required"
// @Failure      401 {string} string "unauthorized"
// @Failure      500 {string} string "failed to list batches"
// @Router       /api/v1/batches [get]
func ListBatchesByStatusHandler(svc batchLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		if status == "" {
			http.Error(w, "query parameter is required", http.StatusBadRequest)
			return
		}

		batches, err := svc.GetByStatus(r.Context(), status)
		if err != nil {
			http.Error(w, "failed to list batches", http.StatusInternalServerError)
			return
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
