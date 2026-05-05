package repository

import (
	"context"
	"database/sql"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type RecipeRepo struct {
	db *sql.DB
}

func NewRecipeRepo(db *sql.DB) *RecipeRepo {
	return &RecipeRepo{db: db}
}

func (rr *RecipeRepo) GetByCode(ctx context.Context, code string) (*domain.Recipe, error) {
	var recipe domain.Recipe

	err := rr.db.QueryRowContext(ctx, `
		SELECT 
			id, 
			recipe_code, 
			name, 
			version,
			min_volume,
			max_volume,
			description, 
			required_equipment_type, 
			created_by, 
			created_at, 
			is_active
		FROM recipes
		WHERE recipe_code = $1
	`, code).Scan(
		&recipe.ID,
		&recipe.RecipeCode,
		&recipe.Name,
		&recipe.Version,
		&recipe.MinVolumeL,
		&recipe.MaxVolumeL,
		&recipe.Description,
		&recipe.RequiredEquipmentType,
		&recipe.CreatedBy,
		&recipe.CreatedAt,
		&recipe.IsActive,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrRecipeNotFound
		}
		return nil, err
	}

	if !recipe.IsActive {
		return nil, domain.ErrRecipeArchived
	}

	return &recipe, nil
}
