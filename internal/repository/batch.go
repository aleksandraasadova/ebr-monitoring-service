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
	return &BatchRepo{db: db}
}

func (br *BatchRepo) Create(ctx context.Context, batch *domain.Batch) error {
	recipeID := batch.RecipeID
	tx, err := br.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		INSERT INTO batches (
			recipe_id,
			target_volume_l,
			registered_by
		)
		VALUES ($1, $2, $3)
		RETURNING id, batch_code, status, created_at
	`,
		batch.RecipeID,
		batch.TargetVolumeL,
		batch.RegisteredByID,
	).Scan(&batch.ID, &batch.Code, &batch.Status, &batch.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert batch: %w", err)
	}

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
	`, batch.ID, batch.TargetVolumeL, recipeID)
	if err != nil {
		return fmt.Errorf("insert weighing_log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (br *BatchRepo) GetByStatus(ctx context.Context, status string) ([]domain.Batch, error) {
	rows, err := br.db.QueryContext(ctx, `
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
		ORDER BY created_at DESC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("query batches: %w", err)
	}
	defer rows.Close()

	var batches []domain.Batch
	for rows.Next() {
		var batch domain.Batch
		if err := rows.Scan(
			&batch.ID,
			&batch.Code,
			&batch.RecipeCode,
			&batch.TargetVolumeL,
			&batch.Status,
			&batch.RegisteredByCode,
			&batch.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan batch: %w", err)
		}
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batches: %w", err)
	}

	return batches, nil
}
