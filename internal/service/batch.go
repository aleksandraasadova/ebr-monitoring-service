package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type BatchService struct {
	db         *sql.DB
	batchRepo  domain.BatchRepo
	recipeRepo domain.RecipeRepo
}

func NewBatchService(db *sql.DB, batchRepo domain.BatchRepo, recipeRepo domain.RecipeRepo) *BatchService {
	return &BatchService{
		db:         db,
		batchRepo:  batchRepo,
		recipeRepo: recipeRepo,
	}
}

/*
Сервис: CreateBatch
1. Проверка рецепта (существует, архивирован)
2. Валидация объёма (min/max)
3. Начало транзакции
4. Вызов BatchRepo.Create() ← теперь мы уверены, что рецепт валиден
5. Заполнение weighing_log
6. Коммит
*/
func (bs *BatchService) CreateBatch(ctx context.Context, req domain.CreateBatchRequest, registeredBy int) (*domain.CreateBatchResponse, error) {

	recipe, err := bs.recipeRepo.GetByCode(ctx, req.RecipeCode)
	if err != nil {
		if err == domain.ErrRecipeNotFound {
			return nil, domain.ErrRecipeNotFound
		}
		if err == domain.ErrRecipeArchived {
			return nil, domain.ErrRecipeArchived
		} else {
			return nil, err
		}
	}

	if req.TargetVolumeL < recipe.MinVolumeL || req.TargetVolumeL > recipe.MaxVolumeL {
		return nil, domain.ErrInvalidBatchVolume
	}
	fmt.Printf("Before transaction\n") //
	// the start of transaction
	tx, err := bs.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	fmt.Printf("Transaction started\n") //
	defer tx.Rollback()

	batch := &domain.Batch{
		RecipeID:      recipe.ID,
		TargetVolumeL: req.TargetVolumeL,
		RegisteredBy:  registeredBy,
	}

	if err := bs.batchRepo.Create(ctx, tx, batch); err != nil {
		return nil, fmt.Errorf("failed to create batch: %w", err)
	}
	fmt.Printf("Batch created successfully: %v\n", batch) //
	_, err = tx.ExecContext(ctx, `
		INSERT INTO weighing_log (batch_id, ingredient_id, stage_key, required_qty, container_code)
		SELECT 
			$1::INT,
            ri.ingredient_id,
            ri.stage_key,
            ROUND((($2::NUMERIC) * 1000.0 * ri.percentage / 100.0), 2),
            'CNT-' || TO_CHAR(CURRENT_DATE, 'YYYY') || '-' || 
            LPAD($1::text, 6, '0') || '-' || 
            LPAD(ROW_NUMBER() OVER (ORDER BY ri.stage_key, ri.ingredient_id)::text, 3, '0')
        FROM recipe_ingredients ri
        WHERE ri.recipe_id = $3
	`, batch.ID, req.TargetVolumeL, recipe.ID)

	if err != nil {
		fmt.Printf("REAL SQL ERROR: %v\n", err)
		return nil, fmt.Errorf("failed to fill weighing_log: %w", err)
	}
	fmt.Printf("Weighing log filled successfully\n") //

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &domain.CreateBatchResponse{
		BatchCode:   batch.Code,
		BatchStatus: batch.Status,
		CreatedAt:   batch.CreatedAt,
	}, nil
}
