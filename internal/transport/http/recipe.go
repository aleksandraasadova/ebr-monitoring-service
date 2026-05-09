package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type recipeService interface {
	GetByCode(ctx context.Context, code string) (*domain.Recipe, error)
}

type RecipeHandler struct {
	svc recipeService
}

func NewRecipeHandler(svc recipeService) *RecipeHandler {
	return &RecipeHandler{svc: svc}
}

// GetByCode godoc
// @Summary      Получить рецепт по коду
// @Tags         recipes
// @Produce      json
// @Security     BearerAuth
// @Param        code path string true "код рецептуры"
// @Success      200 {object} GetRecipeByCodeResponse
// @Failure      401 {string} string "unauthorized"
// @Failure      404 {string} string "recipe not found"
// @Failure      409 {string} string "recipe archived"
// @Failure      500 {string} string "internal server error"
// @Router       /api/v1/recipes/{code} [get]
func (h *RecipeHandler) GetByCode(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	recipe, err := h.svc.GetByCode(r.Context(), code)
	if err != nil {
		switch err {
		case domain.ErrRecipeNotFound:
			http.Error(w, "recipe not found", http.StatusNotFound)
		case domain.ErrRecipeArchived:
			http.Error(w, "recipe archived", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetRecipeByCodeResponse{
		Name:                  recipe.Name,
		Version:               recipe.Version,
		MinVolumeL:            recipe.MinVolumeL,
		MaxVolumeL:            recipe.MaxVolumeL,
		Description:           recipe.Description,
		RequiredEquipmentType: recipe.RequiredEquipmentType,
	})
}
