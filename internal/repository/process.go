package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type ProcessRepo struct {
	db *sql.DB
}

func NewProcessRepo(db *sql.DB) *ProcessRepo {
	return &ProcessRepo{db: db}
}

func (r *ProcessRepo) CreateStage(ctx context.Context, stage *domain.BatchStage) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO batch_stages (batch_id, stage_number, stage_key, stage_name)
		VALUES ($1, $2, $3, $4)
		RETURNING id, started_at
	`, stage.BatchID, stage.StageNumber, stage.StageKey, stage.StageName).
		Scan(&stage.ID, &stage.StartedAt)
}

func (r *ProcessRepo) GetStagesByBatchID(ctx context.Context, batchID int) ([]domain.BatchStage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, batch_id, stage_number, stage_key, stage_name,
		       started_at, completed_at, signed_by, signed_at
		FROM batch_stages
		WHERE batch_id = $1
		ORDER BY stage_number
	`, batchID)
	if err != nil {
		return nil, fmt.Errorf("query stages: %w", err)
	}
	defer rows.Close()

	var stages []domain.BatchStage
	for rows.Next() {
		var s domain.BatchStage
		var completedAt sql.NullTime
		var signedBy sql.NullInt64
		var signedAt sql.NullTime

		if err := rows.Scan(
			&s.ID, &s.BatchID, &s.StageNumber, &s.StageKey, &s.StageName,
			&s.StartedAt, &completedAt, &signedBy, &signedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stage: %w", err)
		}
		if completedAt.Valid {
			t := completedAt.Time
			s.CompletedAt = &t
		}
		if signedBy.Valid {
			v := int(signedBy.Int64)
			s.SignedBy = &v
		}
		if signedAt.Valid {
			t := signedAt.Time
			s.SignedAt = &t
		}
		stages = append(stages, s)
	}
	return stages, rows.Err()
}

func (r *ProcessRepo) GetCurrentStageByBatchID(ctx context.Context, batchID int) (*domain.BatchStage, error) {
	var s domain.BatchStage
	var completedAt sql.NullTime
	var signedBy sql.NullInt64
	var signedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, `
		SELECT id, batch_id, stage_number, stage_key, stage_name,
		       started_at, completed_at, signed_by, signed_at
		FROM batch_stages
		WHERE batch_id = $1
		ORDER BY stage_number DESC
		LIMIT 1
	`, batchID).Scan(
		&s.ID, &s.BatchID, &s.StageNumber, &s.StageKey, &s.StageName,
		&s.StartedAt, &completedAt, &signedBy, &signedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrStageNotFound
		}
		return nil, fmt.Errorf("query current stage: %w", err)
	}
	if completedAt.Valid {
		t := completedAt.Time
		s.CompletedAt = &t
	}
	if signedBy.Valid {
		v := int(signedBy.Int64)
		s.SignedBy = &v
	}
	if signedAt.Valid {
		t := signedAt.Time
		s.SignedAt = &t
	}
	return &s, nil
}

func (r *ProcessRepo) SignAndCompleteStage(ctx context.Context, batchID int, stageKey string, userID int) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE batch_stages
		SET completed_at = NOW(), signed_by = $3, signed_at = NOW()
		WHERE batch_id = $1 AND stage_key = $2
		  AND completed_at IS NULL
	`, batchID, stageKey, userID)
	if err != nil {
		return fmt.Errorf("sign stage: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrStageAlreadySigned
	}
	return nil
}

func (r *ProcessRepo) GetBatchIDByCode(ctx context.Context, batchCode string) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx, `SELECT id FROM batches WHERE batch_code = $1`, batchCode).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domain.ErrBatchNotFound
		}
		return 0, fmt.Errorf("get batch id: %w", err)
	}
	return id, nil
}

func (r *ProcessRepo) StartProcess(ctx context.Context, batchCode string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE batches SET status = 'in_process', started_at = COALESCE(started_at, NOW())
		WHERE batch_code = $1 AND status = 'ready_for_process'
	`, batchCode)
	if err != nil {
		return fmt.Errorf("start process: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrInvalidBatchStatus
	}
	return nil
}
