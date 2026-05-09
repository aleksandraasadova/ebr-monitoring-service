package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type recipeGetter interface {
	GetByCode(ctx context.Context, code string) (*domain.GetRecipeByCodeResponse, error)
}

// GetRecipeByCodeHandler godoc
// @Summary      Получить рецепт по коду
// @Tags         recipes
// @Produce      json
// @Security     BearerAuth
// @Param        code path string true "код рецепта"
// @Success      200 {object} domain.GetRecipeByCodeResponse
// @Failure      401 {string} string "unauthorized"
// @Failure      404 {string} string "recipe not found"
// @Failure      409 {string} string "recipe archived"
// @Failure      500 {string} string "internal server error"
// @Router       /api/v1/recipes/{code} [get]
func GetRecipeByCodeHandler(svc recipeGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		resp, err := svc.GetByCode(r.Context(), code)
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
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}
