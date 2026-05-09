package service

import (
	"context"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type RecipeService struct {
	recipeRepo recipeRepo
}

func NewRecipeService(r recipeRepo) *RecipeService {
	return &RecipeService{recipeRepo: r}
}

func (rs *RecipeService) GetByCode(ctx context.Context, code string) (*domain.Recipe, error) {
	return rs.recipeRepo.GetByCode(ctx, code)
}
