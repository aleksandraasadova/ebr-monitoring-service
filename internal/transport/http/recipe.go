package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/repository"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/middleware"
)

type recipeService interface {
	GetByCode(ctx context.Context, code string) (*domain.Recipe, error)
	GetAll(ctx context.Context) ([]domain.Recipe, error)
	Create(ctx context.Context, recipe *domain.Recipe, ingredients []repository.RecipeIngredientInput) error
	Archive(ctx context.Context, code string) error
	GetIngredients(ctx context.Context) ([]repository.Ingredient, error)
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

func (h *RecipeHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Name                  string  `json:"name"`
		Version               string  `json:"version"`
		MinVolumeL            int     `json:"min_volume_l"`
		MaxVolumeL            int     `json:"max_volume_l"`
		Description           string  `json:"description"`
		RequiredEquipmentType string  `json:"required_equipment_type"`
		Ingredients           []struct {
			IngredientID int     `json:"ingredient_id"`
			StageKey     string  `json:"stage_key"`
			Percentage   float64 `json:"percentage"`
		} `json:"ingredients"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Version == "" {
		http.Error(w, "name and version are required", http.StatusBadRequest)
		return
	}

	recipe := &domain.Recipe{
		Name:                  req.Name,
		Version:               req.Version,
		MinVolumeL:            req.MinVolumeL,
		MaxVolumeL:            req.MaxVolumeL,
		Description:           req.Description,
		RequiredEquipmentType: req.RequiredEquipmentType,
		CreatedBy:             user.UserID,
		IsActive:              true,
	}

	ings := make([]repository.RecipeIngredientInput, len(req.Ingredients))
	for i, ing := range req.Ingredients {
		ings[i] = repository.RecipeIngredientInput{
			IngredientID: ing.IngredientID,
			StageKey:     ing.StageKey,
			Percentage:   ing.Percentage,
		}
	}

	if err := h.svc.Create(r.Context(), recipe, ings); err != nil {
		http.Error(w, "failed to create recipe", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"recipe_code": recipe.RecipeCode,
	})
}

func (h *RecipeHandler) Archive(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if err := h.svc.Archive(r.Context(), code); err != nil {
		if errors.Is(err, domain.ErrRecipeNotFound) {
			http.Error(w, "recipe not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to archive recipe", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RecipeHandler) GetIngredients(w http.ResponseWriter, r *http.Request) {
	ings, err := h.svc.GetIngredients(r.Context())
	if err != nil {
		http.Error(w, "failed to get ingredients", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ings)
}

func (h *RecipeHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	recipes, err := h.svc.GetAll(r.Context())
	if err != nil {
		http.Error(w, "failed to list recipes", http.StatusInternalServerError)
		return
	}

	type item struct {
		RecipeCode            string `json:"recipe_code"`
		Name                  string `json:"name"`
		Version               string `json:"version"`
		MinVolumeL            int    `json:"min_volume_l"`
		MaxVolumeL            int    `json:"max_volume_l"`
		Description           string `json:"description"`
		RequiredEquipmentType string `json:"required_equipment_type"`
		IsActive              bool   `json:"is_active"`
	}

	resp := make([]item, len(recipes))
	for i, r := range recipes {
		resp[i] = item{
			RecipeCode:            r.RecipeCode,
			Name:                  r.Name,
			Version:               r.Version,
			MinVolumeL:            r.MinVolumeL,
			MaxVolumeL:            r.MaxVolumeL,
			Description:           r.Description,
			RequiredEquipmentType: r.RequiredEquipmentType,
			IsActive:              r.IsActive,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
