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
		batch.RecipeID,       // $1
		batch.TargetVolumeL,  // $2
		batch.RegisteredByID, // $3
	).Scan(&batch.ID, &batch.Code, &batch.Status, &batch.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create batch: %w", err)
	}
	return nil
}

func (br *BatchRepo) GetByStatus(ctx context.Context, status string) ([]domain.Batch, error) {
	query := `
	SELECT 
		b.id, 
		b.batch_code, 
		r.recipe_code, 
		b.target_volume_l, 
		b.status, 
		u.user_code, 
		b.created_at
	FROM batches b
	INNER JOIN recipes r ON b.recipe_id = r.id
	INNER JOIN users u ON b.registered_by = u.id
	WHERE status = $1
	ORDER BY created_at DESC`

	rows, err := br.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to find rows: %w", err)
	}
	defer rows.Close()

	var batches []domain.Batch

	for rows.Next() {
		var batch domain.Batch
		if err := rows.Scan(&batch.ID, &batch.Code, &batch.RecipeCode, &batch.TargetVolumeL, &batch.Status, &batch.RegisteredByCode, &batch.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan row failed: %w", err)
		}
		batches = append(batches, batch)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return batches, nil
}
