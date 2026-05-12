package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type TelemetryRepo struct {
	db *sql.DB
}

func NewTelemetryRepo(db *sql.DB) *TelemetryRepo {
	return &TelemetryRepo{db: db}
}

func (r *TelemetryRepo) SaveReading(ctx context.Context, record *domain.TelemetryRecord) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO telemetry (batch_id, sensor_id, stage_key, value)
		VALUES ($1, $2, $3, $4)
		RETURNING id, recorded_at
	`, record.BatchID, record.SensorID, record.StageKey, record.Value).
		Scan(&record.ID, &record.RecordedAt)
}

func (r *TelemetryRepo) GetSensorIDByCode(ctx context.Context, sensorCode string) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx, `SELECT id FROM sensors WHERE sensor_code = $1`, sensorCode).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("sensor not found: %s", sensorCode)
		}
		return 0, fmt.Errorf("get sensor id: %w", err)
	}
	return id, nil
}

func (r *TelemetryRepo) GetReadingsByBatchAndStage(ctx context.Context, batchID int, stageKey string) ([]domain.TelemetryRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.batch_id, t.sensor_id, s.sensor_code, t.stage_key, t.value, t.recorded_at
		FROM telemetry t
		JOIN sensors s ON s.id = t.sensor_id
		WHERE t.batch_id = $1 AND t.stage_key = $2
		ORDER BY t.recorded_at
	`, batchID, stageKey)
	if err != nil {
		return nil, fmt.Errorf("query telemetry: %w", err)
	}
	defer rows.Close()

	var records []domain.TelemetryRecord
	for rows.Next() {
		var rec domain.TelemetryRecord
		if err := rows.Scan(
			&rec.ID, &rec.BatchID, &rec.SensorID, &rec.SensorCode,
			&rec.StageKey, &rec.Value, &rec.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan telemetry: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// GetStageAggregates returns avg/min/max per sensor for a batch stage, used in reports.
func (r *TelemetryRepo) GetStageAggregates(ctx context.Context, batchID int, stageKey string) ([]StageAggregate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.sensor_code, s.name, s.unit,
		       AVG(t.value)::DECIMAL(10,2) AS avg_val,
		       MIN(t.value)::DECIMAL(10,2) AS min_val,
		       MAX(t.value)::DECIMAL(10,2) AS max_val,
		       COUNT(*) AS reading_count
		FROM telemetry t
		JOIN sensors s ON s.id = t.sensor_id
		WHERE t.batch_id = $1 AND t.stage_key = $2
		GROUP BY s.sensor_code, s.name, s.unit
		ORDER BY s.sensor_code
	`, batchID, stageKey)
	if err != nil {
		return nil, fmt.Errorf("query aggregates: %w", err)
	}
	defer rows.Close()

	var aggs []StageAggregate
	for rows.Next() {
		var a StageAggregate
		if err := rows.Scan(&a.SensorCode, &a.SensorName, &a.Unit, &a.Avg, &a.Min, &a.Max, &a.Count); err != nil {
			return nil, fmt.Errorf("scan aggregate: %w", err)
		}
		aggs = append(aggs, a)
	}
	return aggs, rows.Err()
}

type StageAggregate struct {
	SensorCode string
	SensorName string
	Unit       string
	Avg        float64
	Min        float64
	Max        float64
	Count      int
}
