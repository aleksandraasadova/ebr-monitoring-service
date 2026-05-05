package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type BatchRepo struct {
	db *sql.DB
}

func NewBatchRepo(db *sql.DB) *BatchRepo {
	return &BatchRepo{
		db: db,
	}
}

func (br *BatchRepo) Create(ctx context.Context, tx *sql.Tx, batch *domain.Batch) error {

	err := tx.QueryRowContext(ctx, `
		INSERT INTO batches ( 
			recipe_id, 
			target_volume_l,
			registered_by
		)
		VALUES ($1, $2, $3)
		RETURNING id, batch_code, status, created_at
	`,
		batch.RecipeID,      // $1
		batch.TargetVolumeL, // $2
		batch.RegisteredBy,  // $3
	).Scan(&batch.ID, &batch.Code, &batch.Status, &batch.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create batch: %w", err)
	}
	return nil
}
