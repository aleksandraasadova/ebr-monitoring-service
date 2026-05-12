package service

import (
	"context"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/repository"
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

func (rs *RecipeService) GetAll(ctx context.Context) ([]domain.Recipe, error) {
	return rs.recipeRepo.GetAll(ctx)
}

func (rs *RecipeService) Create(ctx context.Context, recipe *domain.Recipe, ingredients []repository.RecipeIngredientInput) error {
	return rs.recipeRepo.Create(ctx, recipe, ingredients)
}

func (rs *RecipeService) Archive(ctx context.Context, code string) error {
	return rs.recipeRepo.Archive(ctx, code)
}

func (rs *RecipeService) GetIngredients(ctx context.Context) ([]repository.Ingredient, error) {
	return rs.recipeRepo.GetIngredients(ctx)
}
