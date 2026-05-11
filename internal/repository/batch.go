package repository

import (
	"context"
	"database/sql"
	"errors"
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

func (br *BatchRepo) GetWeighingLogByBatchCode(ctx context.Context, batchCode string) ([]domain.WeighingLogItem, error) {
	rows, err := br.db.QueryContext(ctx, `
		SELECT
			wl.id,
			b.batch_code,
			b.status,
			wl.ingredient_id,
			i.name,
			wl.stage_key,
			wl.required_qty,
			wl.actual_qty,
			wl.container_code,
			u.user_code,
			wl.weighed_at
		FROM weighing_log wl
		INNER JOIN batches b ON b.id = wl.batch_id
		INNER JOIN ingredients i ON i.id = wl.ingredient_id
		LEFT JOIN users u ON u.id = wl.weighed_by
		WHERE b.batch_code = $1
		ORDER BY wl.stage_key, wl.id
	`, batchCode)
	if err != nil {
		return nil, fmt.Errorf("query weighing log: %w", err)
	}
	defer rows.Close()

	var items []domain.WeighingLogItem
	for rows.Next() {
		var item domain.WeighingLogItem
		var actualQty sql.NullFloat64
		var containerCode sql.NullString
		var weighedByCode sql.NullString
		var weighedAt sql.NullTime

		if err := rows.Scan(
			&item.ID,
			&item.BatchCode,
			&item.BatchStatus,
			&item.IngredientID,
			&item.Ingredient,
			&item.StageKey,
			&item.RequiredQty,
			&actualQty,
			&containerCode,
			&weighedByCode,
			&weighedAt,
		); err != nil {
			return nil, fmt.Errorf("scan weighing log: %w", err)
		}

		if actualQty.Valid {
			v := actualQty.Float64
			item.ActualQty = &v
		}
		if containerCode.Valid {
			item.ContainerCode = containerCode.String
		}
		if weighedByCode.Valid {
			item.WeighedByCode = weighedByCode.String
		}
		if weighedAt.Valid {
			t := weighedAt.Time
			item.WeighedAt = &t
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate weighing log: %w", err)
	}
	if len(items) == 0 {
		return nil, domain.ErrBatchNotFound
	}

	return items, nil
}

func (br *BatchRepo) StartWeighing(ctx context.Context, batchCode string, operatorID int) error {
	res, err := br.db.ExecContext(ctx, `
		UPDATE batches
		SET
			status = 'weighing_in_progress',
			operator_id = $2,
			started_at = COALESCE(started_at, NOW())
		WHERE batch_code = $1
		  AND status IN ('waiting_weighing', 'weighing_in_progress')
	`, batchCode, operatorID)
	if err != nil {
		return fmt.Errorf("update batch status: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows: %w", err)
	}
	if affected == 0 {
		return br.classifyBatchUpdateMiss(ctx, batchCode)
	}

	return nil
}

func (br *BatchRepo) ConfirmWeighingItem(ctx context.Context, batchCode string, itemID int, actualQty float64, operatorID int) (string, error) {
	tx, err := br.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var batchID int
	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT id, status
		FROM batches
		WHERE batch_code = $1
		FOR UPDATE
	`, batchCode).Scan(&batchID, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domain.ErrBatchNotFound
		}
		return "", fmt.Errorf("select batch: %w", err)
	}
	if status != "weighing_in_progress" {
		return "", domain.ErrInvalidBatchStatus
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE weighing_log
		SET actual_qty = $1,
			weighed_by = $2,
			weighed_at = NOW()
		WHERE id = $3
		  AND batch_id = $4
		  AND actual_qty IS NULL
	`, actualQty, operatorID, itemID, batchID)
	if err != nil {
		return "", fmt.Errorf("update weighing item: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("get affected rows: %w", err)
	}
	if affected == 0 {
		return "", domain.ErrWeighingNotFound
	}

	var remaining int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM weighing_log
		WHERE batch_id = $1
		  AND actual_qty IS NULL
	`, batchID).Scan(&remaining); err != nil {
		return "", fmt.Errorf("count remaining weighing items: %w", err)
	}

	nextStatus := status
	if remaining == 0 {
		nextStatus = "ready_for_process"
		if _, err := tx.ExecContext(ctx, `
			UPDATE batches
			SET status = $1
			WHERE id = $2
		`, nextStatus, batchID); err != nil {
			return "", fmt.Errorf("update batch ready status: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return nextStatus, nil
}

func (br *BatchRepo) classifyBatchUpdateMiss(ctx context.Context, batchCode string) error {
	var status string
	err := br.db.QueryRowContext(ctx, `
		SELECT status
		FROM batches
		WHERE batch_code = $1
	`, batchCode).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrBatchNotFound
		}
		return fmt.Errorf("select batch status: %w", err)
	}
	return domain.ErrInvalidBatchStatus
}
