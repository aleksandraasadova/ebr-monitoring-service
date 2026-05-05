package service

import (
	"context"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type RecipeService struct {
	recipeRepo domain.RecipeRepo
}

func NewRecipeService(recipeRepo domain.RecipeRepo) *RecipeService {
	return &RecipeService{
		recipeRepo: recipeRepo,
	}
}

func (rs *RecipeService) GetByCode(ctx context.Context, code string) (*domain.GetRecipeByCodeResponse, error) {
	recipe, err := rs.recipeRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	return &domain.GetRecipeByCodeResponse{
		Name:                  recipe.Name,
		Version:               recipe.Version,
		MinVolumeL:            recipe.MinVolumeL,
		MaxVolumeL:            recipe.MaxVolumeL,
		Description:           recipe.Description,
		RequiredEquipmentType: recipe.RequiredEquipmentType,
	}, nil
}
