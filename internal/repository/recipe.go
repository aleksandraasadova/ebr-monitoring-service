package repository

import (
	"context"
	"database/sql"
	"fmt"

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

type RecipeIngredientInput struct {
	IngredientID int
	StageKey     string
	Percentage   float64
}

func (rr *RecipeRepo) Create(ctx context.Context, recipe *domain.Recipe, ingredients []RecipeIngredientInput) error {
	tx, err := rr.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		INSERT INTO recipes (name, version, min_volume, max_volume, description, required_equipment_type, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, recipe_code, created_at
	`, recipe.Name, recipe.Version, recipe.MinVolumeL, recipe.MaxVolumeL,
		recipe.Description, recipe.RequiredEquipmentType, recipe.CreatedBy,
	).Scan(&recipe.ID, &recipe.RecipeCode, &recipe.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert recipe: %w", err)
	}

	for _, ing := range ingredients {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO recipe_ingredients (recipe_id, ingredient_id, stage_key, percentage)
			VALUES ($1, $2, $3, $4)
		`, recipe.ID, ing.IngredientID, ing.StageKey, ing.Percentage); err != nil {
			return fmt.Errorf("insert ingredient: %w", err)
		}
	}

	return tx.Commit()
}

func (rr *RecipeRepo) Archive(ctx context.Context, code string) error {
	res, err := rr.db.ExecContext(ctx, `
		UPDATE recipes SET is_active = false WHERE recipe_code = $1 AND is_active = true
	`, code)
	if err != nil {
		return fmt.Errorf("archive recipe: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrRecipeNotFound
	}
	return nil
}

type Ingredient struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Unit string `json:"unit"`
}

func (rr *RecipeRepo) GetIngredients(ctx context.Context) ([]Ingredient, error) {
	rows, err := rr.db.QueryContext(ctx, `SELECT id, name, unit FROM ingredients ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("get ingredients: %w", err)
	}
	defer rows.Close()
	var result []Ingredient
	for rows.Next() {
		var i Ingredient
		if err := rows.Scan(&i.ID, &i.Name, &i.Unit); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (rr *RecipeRepo) GetAll(ctx context.Context) ([]domain.Recipe, error) {
	rows, err := rr.db.QueryContext(ctx, `
		SELECT id, recipe_code, name, version, min_volume, max_volume,
		       description, required_equipment_type, created_by, created_at, is_active
		FROM recipes
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query recipes: %w", err)
	}
	defer rows.Close()

	var recipes []domain.Recipe
	for rows.Next() {
		var r domain.Recipe
		if err := rows.Scan(
			&r.ID, &r.RecipeCode, &r.Name, &r.Version,
			&r.MinVolumeL, &r.MaxVolumeL, &r.Description,
			&r.RequiredEquipmentType, &r.CreatedBy, &r.CreatedAt, &r.IsActive,
		); err != nil {
			return nil, fmt.Errorf("scan recipe: %w", err)
		}
		recipes = append(recipes, r)
	}
	return recipes, rows.Err()
}
