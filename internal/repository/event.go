package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type EventRepo struct {
	db *sql.DB
}

func NewEventRepo(db *sql.DB) *EventRepo {
	return &EventRepo{db: db}
}

func (r *EventRepo) CreateEvent(ctx context.Context, event *domain.Event) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO events (batch_id, stage_key, type, severity, description)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, occurred_at
	`, event.BatchID, event.StageKey, event.Type, event.Severity, event.Description).
		Scan(&event.ID, &event.OccurredAt)
}

func (r *EventRepo) GetEventsByBatchID(ctx context.Context, batchID int) ([]domain.Event, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, batch_id, stage_key, type, severity, description, comment, resolved_by, occurred_at
		FROM events
		WHERE batch_id = $1
		ORDER BY occurred_at
	`, batchID)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var e domain.Event
		var comment sql.NullString
		var resolvedBy sql.NullInt64

		if err := rows.Scan(
			&e.ID, &e.BatchID, &e.StageKey, &e.Type, &e.Severity,
			&e.Description, &comment, &resolvedBy, &e.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if comment.Valid {
			e.Comment = comment.String
		}
		if resolvedBy.Valid {
			v := int(resolvedBy.Int64)
			e.ResolvedBy = &v
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *EventRepo) ResolveEvent(ctx context.Context, eventID int, userID int, comment string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE events
		SET resolved_by = $2, comment = $3
		WHERE id = $1 AND resolved_by IS NULL
	`, eventID, userID, comment)
	if err != nil {
		return fmt.Errorf("resolve event: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrEventNotFound
	}
	return nil
}

func (r *EventRepo) GetEventsByBatchIDAndStage(ctx context.Context, batchID int, stageKey string) ([]domain.Event, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, batch_id, stage_key, type, severity, description, comment, resolved_by, occurred_at
		FROM events
		WHERE batch_id = $1 AND stage_key = $2
		ORDER BY occurred_at
	`, batchID, stageKey)
	if err != nil {
		return nil, fmt.Errorf("query events by stage: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var e domain.Event
		var comment sql.NullString
		var resolvedBy sql.NullInt64

		if err := rows.Scan(
			&e.ID, &e.BatchID, &e.StageKey, &e.Type, &e.Severity,
			&e.Description, &comment, &resolvedBy, &e.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if comment.Valid {
			e.Comment = comment.String
		}
		if resolvedBy.Valid {
			v := int(resolvedBy.Int64)
			e.ResolvedBy = &v
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *EventRepo) GetByID(ctx context.Context, eventID int) (*domain.Event, error) {
	var e domain.Event
	var comment sql.NullString
	var resolvedBy sql.NullInt64

	err := r.db.QueryRowContext(ctx, `
		SELECT id, batch_id, stage_key, type, severity, description, comment, resolved_by, occurred_at
		FROM events WHERE id = $1
	`, eventID).Scan(
		&e.ID, &e.BatchID, &e.StageKey, &e.Type, &e.Severity,
		&e.Description, &comment, &resolvedBy, &e.OccurredAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrEventNotFound
		}
		return nil, fmt.Errorf("get event: %w", err)
	}
	if comment.Valid {
		e.Comment = comment.String
	}
	if resolvedBy.Valid {
		v := int(resolvedBy.Int64)
		e.ResolvedBy = &v
	}
	return &e, nil
}
