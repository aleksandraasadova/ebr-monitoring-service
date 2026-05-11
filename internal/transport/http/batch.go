package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/middleware"
)

type batchService interface {
	CreateBatch(ctx context.Context, recipeCode string, targetVolumeL int, registeredByID int) (*domain.Batch, error)
	GetByStatus(ctx context.Context, status string) ([]domain.Batch, error)
	GetWeighingLogByBatchCode(ctx context.Context, batchCode string) ([]domain.WeighingLogItem, error)
	StartWeighing(ctx context.Context, batchCode string, operatorID int) error
	ConfirmWeighingItem(ctx context.Context, batchCode string, itemID int, actualQty float64, operatorID int) (string, error)
}

type BatchHandler struct {
	svc batchService
}

func NewBatchHandler(svc batchService) *BatchHandler {
	return &BatchHandler{svc: svc}
}

// Create godoc
// @Summary      Создать партию (batch)
// @Description  Создаёт партию по рецепту. registered_by берётся из JWT.
// @Tags         batches
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body CreateBatchRequest true "данные партии"
// @Success      201 {object} CreateBatchResponse
// @Failure      400 {string} string "invalid json or invalid batch volume"
// @Failure      401 {string} string "unauthorized"
// @Failure      404 {string} string "recipe not found or archived"
// @Failure      500 {string} string "failed to create batch"
// @Router       /api/v1/batches [post]
func (h *BatchHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	batch, err := h.svc.CreateBatch(r.Context(), req.RecipeCode, req.TargetVolumeL, user.UserID)
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
	json.NewEncoder(w).Encode(CreateBatchResponse{
		BatchCode:    batch.Code,
		BatchStatus:  batch.Status,
		CreatedAt:    batch.CreatedAt,
		RegisteredBy: batch.RegisteredByID,
	})
}

// ListByStatus godoc
// @Summary      Список партий по статусу
// @Tags         batches
// @Produce      json
// @Security     BearerAuth
// @Param        status query string true "статус партии"
// @Success      200 {array} GetBatchesByStatusResponse
// @Failure      400 {string} string "query parameter is required"
// @Failure      401 {string} string "unauthorized"
// @Failure      500 {string} string "failed to list batches"
// @Router       /api/v1/batches [get]
func (h *BatchHandler) ListByStatus(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		http.Error(w, "query parameter is required", http.StatusBadRequest)
		return
	}

	batches, err := h.svc.GetByStatus(r.Context(), status)
	if err != nil {
		http.Error(w, "failed to list batches", http.StatusInternalServerError)
		return
	}

	resp := make([]GetBatchesByStatusResponse, len(batches))
	for i, b := range batches {
		resp[i] = GetBatchesByStatusResponse{
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

// GetWeighingLog godoc
// @Summary      Получить строки взвешивания по коду партии
// @Tags         batches
// @Produce      json
// @Security     BearerAuth
// @Param        code path string true "код партии"
// @Success      200 {array} WeighingLogItemResponse
// @Failure      401 {string} string "unauthorized"
// @Failure      404 {string} string "batch not found"
// @Failure      500 {string} string "failed to get weighing log"
// @Router       /api/v1/batches/{code}/weighing [get]
func (h *BatchHandler) GetWeighingLog(w http.ResponseWriter, r *http.Request) {
	batchCode := r.PathValue("code")

	items, err := h.svc.GetWeighingLogByBatchCode(r.Context(), batchCode)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrBatchNotFound):
			http.Error(w, "batch not found", http.StatusNotFound)
		default:
			http.Error(w, "failed to get weighing log", http.StatusInternalServerError)
		}
		return
	}

	resp := make([]WeighingLogItemResponse, len(items))
	for i, item := range items {
		resp[i] = WeighingLogItemResponse{
			ID:            item.ID,
			BatchCode:     item.BatchCode,
			BatchStatus:   item.BatchStatus,
			IngredientID:  item.IngredientID,
			Ingredient:    item.Ingredient,
			StageKey:      item.StageKey,
			RequiredQty:   item.RequiredQty,
			ActualQty:     item.ActualQty,
			ContainerCode: item.ContainerCode,
			WeighedBy:     item.WeighedByCode,
			WeighedAt:     item.WeighedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// StartWeighing godoc
// @Summary      Начать взвешивание партии
// @Tags         batches
// @Security     BearerAuth
// @Param        code path string true "код партии"
// @Success      204
// @Failure      401 {string} string "unauthorized"
// @Failure      404 {string} string "batch not found"
// @Failure      409 {string} string "invalid batch status"
// @Failure      500 {string} string "failed to start weighing"
// @Router       /api/v1/batches/{code}/weighing/start [post]
func (h *BatchHandler) StartWeighing(w http.ResponseWriter, r *http.Request) {
	batchCode := r.PathValue("code")

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.svc.StartWeighing(r.Context(), batchCode, user.UserID); err != nil {
		switch {
		case errors.Is(err, domain.ErrBatchNotFound):
			http.Error(w, "batch not found", http.StatusNotFound)
		case errors.Is(err, domain.ErrInvalidBatchStatus):
			http.Error(w, "invalid batch status", http.StatusConflict)
		default:
			http.Error(w, "failed to start weighing", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ConfirmWeighingItem godoc
// @Summary      Подтвердить строку взвешивания
// @Tags         batches
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        code path string true "код партии"
// @Param        itemID path int true "id строки weighing_log"
// @Param        request body ConfirmWeighingItemRequest true "фактическая масса"
// @Success      200 {object} ConfirmWeighingItemResponse
// @Failure      400 {string} string "invalid json or actual_qty"
// @Failure      401 {string} string "unauthorized"
// @Failure      404 {string} string "batch or weighing item not found"
// @Failure      409 {string} string "invalid batch status"
// @Failure      500 {string} string "failed to confirm weighing item"
// @Router       /api/v1/batches/{code}/weighing/{itemID}/confirm [post]
func (h *BatchHandler) ConfirmWeighingItem(w http.ResponseWriter, r *http.Request) {
	batchCode := r.PathValue("code")
	itemID, err := strconv.Atoi(r.PathValue("itemID"))
	if err != nil || itemID <= 0 {
		http.Error(w, "invalid weighing item id", http.StatusBadRequest)
		return
	}

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req ConfirmWeighingItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.ActualQty < 0 {
		http.Error(w, "invalid actual_qty", http.StatusBadRequest)
		return
	}

	status, err := h.svc.ConfirmWeighingItem(r.Context(), batchCode, itemID, req.ActualQty, user.UserID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrBatchNotFound), errors.Is(err, domain.ErrWeighingNotFound):
			http.Error(w, "batch or weighing item not found", http.StatusNotFound)
		case errors.Is(err, domain.ErrInvalidBatchStatus):
			http.Error(w, "invalid batch status", http.StatusConflict)
		default:
			http.Error(w, "failed to confirm weighing item", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfirmWeighingItemResponse{BatchStatus: status})
}
