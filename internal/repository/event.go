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
		INSERT INTO events (
			batch_id, stage_key, type, severity, description,
			sensor_code, started_at, ended_at, start_value, end_value,
			min_value, max_value, avg_value, sample_count
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, occurred_at
	`,
		event.BatchID, event.StageKey, event.Type, event.Severity, event.Description,
		nullString(event.SensorCode), event.StartedAt, event.EndedAt, event.StartValue, event.EndValue,
		event.MinValue, event.MaxValue, event.AvgValue, event.SampleCount,
	).
		Scan(&event.ID, &event.OccurredAt)
}

func (r *EventRepo) GetEventsByBatchID(ctx context.Context, batchID int) ([]domain.Event, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, batch_id, stage_key, type, severity, description, comment, resolved_by, occurred_at,
		       sensor_code, started_at, ended_at, start_value, end_value, min_value, max_value, avg_value, sample_count
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
		var comment, sensorCode sql.NullString
		var resolvedBy sql.NullInt64
		var startedAt, endedAt sql.NullTime
		var startValue, endValue, minValue, maxValue, avgValue sql.NullFloat64
		var sampleCount sql.NullInt64

		if err := rows.Scan(
			&e.ID, &e.BatchID, &e.StageKey, &e.Type, &e.Severity,
			&e.Description, &comment, &resolvedBy, &e.OccurredAt,
			&sensorCode, &startedAt, &endedAt, &startValue, &endValue, &minValue, &maxValue, &avgValue, &sampleCount,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		applyEventNulls(&e, comment, resolvedBy, sensorCode, startedAt, endedAt, startValue, endValue, minValue, maxValue, avgValue, sampleCount)
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
		SELECT id, batch_id, stage_key, type, severity, description, comment, resolved_by, occurred_at,
		       sensor_code, started_at, ended_at, start_value, end_value, min_value, max_value, avg_value, sample_count
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
		var comment, sensorCode sql.NullString
		var resolvedBy sql.NullInt64
		var startedAt, endedAt sql.NullTime
		var startValue, endValue, minValue, maxValue, avgValue sql.NullFloat64
		var sampleCount sql.NullInt64

		if err := rows.Scan(
			&e.ID, &e.BatchID, &e.StageKey, &e.Type, &e.Severity,
			&e.Description, &comment, &resolvedBy, &e.OccurredAt,
			&sensorCode, &startedAt, &endedAt, &startValue, &endValue, &minValue, &maxValue, &avgValue, &sampleCount,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		applyEventNulls(&e, comment, resolvedBy, sensorCode, startedAt, endedAt, startValue, endValue, minValue, maxValue, avgValue, sampleCount)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *EventRepo) GetByID(ctx context.Context, eventID int) (*domain.Event, error) {
	var e domain.Event
	var comment, sensorCode sql.NullString
	var resolvedBy sql.NullInt64
	var startedAt, endedAt sql.NullTime
	var startValue, endValue, minValue, maxValue, avgValue sql.NullFloat64
	var sampleCount sql.NullInt64

	err := r.db.QueryRowContext(ctx, `
		SELECT id, batch_id, stage_key, type, severity, description, comment, resolved_by, occurred_at,
		       sensor_code, started_at, ended_at, start_value, end_value, min_value, max_value, avg_value, sample_count
		FROM events WHERE id = $1
	`, eventID).Scan(
		&e.ID, &e.BatchID, &e.StageKey, &e.Type, &e.Severity,
		&e.Description, &comment, &resolvedBy, &e.OccurredAt,
		&sensorCode, &startedAt, &endedAt, &startValue, &endValue, &minValue, &maxValue, &avgValue, &sampleCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrEventNotFound
		}
		return nil, fmt.Errorf("get event: %w", err)
	}
	applyEventNulls(&e, comment, resolvedBy, sensorCode, startedAt, endedAt, startValue, endValue, minValue, maxValue, avgValue, sampleCount)
	return &e, nil
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func applyEventNulls(
	e *domain.Event,
	comment sql.NullString,
	resolvedBy sql.NullInt64,
	sensorCode sql.NullString,
	startedAt sql.NullTime,
	endedAt sql.NullTime,
	startValue sql.NullFloat64,
	endValue sql.NullFloat64,
	minValue sql.NullFloat64,
	maxValue sql.NullFloat64,
	avgValue sql.NullFloat64,
	sampleCount sql.NullInt64,
) {
	if comment.Valid {
		e.Comment = comment.String
	}
	if resolvedBy.Valid {
		v := int(resolvedBy.Int64)
		e.ResolvedBy = &v
	}
	if sensorCode.Valid {
		e.SensorCode = sensorCode.String
	}
	if startedAt.Valid {
		t := startedAt.Time
		e.StartedAt = &t
	}
	if endedAt.Valid {
		t := endedAt.Time
		e.EndedAt = &t
	}
	if startValue.Valid {
		v := startValue.Float64
		e.StartValue = &v
	}
	if endValue.Valid {
		v := endValue.Float64
		e.EndValue = &v
	}
	if minValue.Valid {
		v := minValue.Float64
		e.MinValue = &v
	}
	if maxValue.Valid {
		v := maxValue.Float64
		e.MaxValue = &v
	}
	if avgValue.Valid {
		v := avgValue.Float64
		e.AvgValue = &v
	}
	if sampleCount.Valid {
		v := int(sampleCount.Int64)
		e.SampleCount = &v
	}
}
