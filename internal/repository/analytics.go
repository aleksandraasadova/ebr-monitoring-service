package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type AnalyticsRepo struct {
	db *sql.DB
}

func NewAnalyticsRepo(db *sql.DB) *AnalyticsRepo {
	return &AnalyticsRepo{db: db}
}

type BatchCountByDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type CycleTime struct {
	BatchCode    string  `json:"batch_code"`
	CreatedAt    string  `json:"created_at"`
	CompletedAt  string  `json:"completed_at"`
	CycleMinutes float64 `json:"cycle_minutes"`
}

type StatusBreakdown struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type EventsByStage struct {
	StageKey  string `json:"stage_key"`
	StageName string `json:"stage_name"`
	Count     int    `json:"count"`
}

type EventsPerBatch struct {
	BatchCode  string `json:"batch_code"`
	EventCount int    `json:"event_count"`
}

type AvgTempPerBatch struct {
	BatchCode string  `json:"batch_code"`
	AvgTemp   float64 `json:"avg_temp"`
	MinTemp   float64 `json:"min_temp"`
	MaxTemp   float64 `json:"max_temp"`
}

type AnalyticsSummary struct {
	TotalBatches     int      `json:"total_batches"`
	CompletedBatches int      `json:"completed_batches"`
	CancelledBatches int      `json:"cancelled_batches"`
	ActiveBatches    int      `json:"active_batches"`
	AvgCycleMinutes  *float64 `json:"avg_cycle_minutes"`
	TotalEvents      int      `json:"total_events"`
	Since            string   `json:"since"`
}

// userFilter returns an extra SQL condition and args when userID != 0.
// baseArgs is the number of args already in the query.
func userFilter(userID, baseArgs int) (string, []any) {
	if userID == 0 {
		return "", nil
	}
	return fmt.Sprintf(" AND (b.registered_by = $%d OR b.operator_id = $%d)", baseArgs+1, baseArgs+1), []any{userID}
}

func (r *AnalyticsRepo) Summary(ctx context.Context, since time.Time, userID int) (*AnalyticsSummary, error) {
	s := &AnalyticsSummary{Since: since.Format("2006-01-02")}

	userCond, userArgs := userFilter(userID, 1)
	args := append([]any{since}, userArgs...)

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'completed'),
			COUNT(*) FILTER (WHERE status = 'cancelled'),
			COUNT(*) FILTER (WHERE status IN ('waiting_weighing','weighing_in_progress','ready_for_process','in_process'))
		FROM batches b
		WHERE b.created_at >= $1`+userCond,
		args...,
	).Scan(&s.TotalBatches, &s.CompletedBatches, &s.CancelledBatches, &s.ActiveBatches)
	if err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}

	var avg sql.NullFloat64
	r.db.QueryRowContext(ctx, `
		SELECT ROUND(AVG(EXTRACT(EPOCH FROM (b.completed_at - b.created_at)) / 60.0)::NUMERIC, 1)
		FROM batches b
		WHERE b.status = 'completed' AND b.completed_at IS NOT NULL AND b.created_at >= $1`+userCond,
		args...,
	).Scan(&avg)
	if avg.Valid {
		s.AvgCycleMinutes = &avg.Float64
	}

	r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events e
		JOIN batches b ON b.id = e.batch_id
		WHERE e.type IN ('alarm','deviation') AND b.created_at >= $1`+userCond,
		args...,
	).Scan(&s.TotalEvents)

	return s, nil
}

func (r *AnalyticsRepo) BatchCountByPeriod(ctx context.Context, days, userID int) ([]BatchCountByDay, error) {
	userCond, userArgs := userFilter(userID, 1)
	args := append([]any{days}, userArgs...)

	rows, err := r.db.QueryContext(ctx, `
		SELECT TO_CHAR(b.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day, COUNT(*) AS cnt
		FROM batches b
		WHERE b.created_at >= NOW() - ($1 || ' days')::INTERVAL`+userCond+`
		GROUP BY day ORDER BY day`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("batch count by period: %w", err)
	}
	defer rows.Close()
	var result []BatchCountByDay
	for rows.Next() {
		var row BatchCountByDay
		if err := rows.Scan(&row.Date, &row.Count); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *AnalyticsRepo) CycleTimes(ctx context.Context, limit, userID int) ([]CycleTime, error) {
	userCond, userArgs := userFilter(userID, 1)
	args := append([]any{limit}, userArgs...)

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			b.batch_code,
			TO_CHAR(b.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI'),
			TO_CHAR(b.completed_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI'),
			ROUND(EXTRACT(EPOCH FROM (b.completed_at - b.created_at)) / 60.0, 1)
		FROM batches b
		WHERE b.status = 'completed' AND b.completed_at IS NOT NULL`+userCond+`
		ORDER BY b.completed_at DESC LIMIT $1`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("cycle times: %w", err)
	}
	defer rows.Close()
	var result []CycleTime
	for rows.Next() {
		var ct CycleTime
		if err := rows.Scan(&ct.BatchCode, &ct.CreatedAt, &ct.CompletedAt, &ct.CycleMinutes); err != nil {
			return nil, err
		}
		result = append(result, ct)
	}
	return result, rows.Err()
}

func (r *AnalyticsRepo) StatusBreakdown(ctx context.Context, userID int) ([]StatusBreakdown, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if userID == 0 {
		rows, err = r.db.QueryContext(ctx, `
			SELECT status, COUNT(*) AS cnt FROM batches b
			GROUP BY status ORDER BY cnt DESC`)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT status, COUNT(*) AS cnt FROM batches b
			WHERE b.registered_by = $1 OR b.operator_id = $1
			GROUP BY status ORDER BY cnt DESC`, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("status breakdown: %w", err)
	}
	defer rows.Close()
	var result []StatusBreakdown
	for rows.Next() {
		var s StatusBreakdown
		if err := rows.Scan(&s.Status, &s.Count); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *AnalyticsRepo) EventsByStage(ctx context.Context, userID int) ([]EventsByStage, error) {
	var (
		rows *sql.Rows
		err  error
	)
	q := `
		SELECT
			COALESCE(e.stage_key, 'unknown'),
			COALESCE(bs.stage_name, e.stage_key, 'Неизвестно'),
			COUNT(*) AS cnt
		FROM events e
		JOIN batches b ON b.id = e.batch_id
		LEFT JOIN batch_stages bs ON bs.batch_id = e.batch_id AND bs.stage_key = e.stage_key
		WHERE e.type IN ('alarm', 'deviation')`
	if userID == 0 {
		rows, err = r.db.QueryContext(ctx, q+` GROUP BY e.stage_key, bs.stage_name ORDER BY cnt DESC LIMIT 10`)
	} else {
		rows, err = r.db.QueryContext(ctx, q+` AND (b.registered_by = $1 OR b.operator_id = $1)
			GROUP BY e.stage_key, bs.stage_name ORDER BY cnt DESC LIMIT 10`, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("events by stage: %w", err)
	}
	defer rows.Close()
	var result []EventsByStage
	for rows.Next() {
		var s EventsByStage
		if err := rows.Scan(&s.StageKey, &s.StageName, &s.Count); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *AnalyticsRepo) EventsPerBatch(ctx context.Context, limit, userID int) ([]EventsPerBatch, error) {
	var (
		rows *sql.Rows
		err  error
	)
	base := `
		SELECT b.batch_code, COUNT(e.id) AS event_count
		FROM batches b
		LEFT JOIN events e ON e.batch_id = b.id AND e.type IN ('alarm', 'deviation')`
	if userID == 0 {
		rows, err = r.db.QueryContext(ctx, base+`
			GROUP BY b.batch_code ORDER BY event_count DESC, b.created_at DESC LIMIT $1`, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, base+`
			WHERE b.registered_by = $2 OR b.operator_id = $2
			GROUP BY b.batch_code ORDER BY event_count DESC, b.created_at DESC LIMIT $1`, limit, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("events per batch: %w", err)
	}
	defer rows.Close()
	var result []EventsPerBatch
	for rows.Next() {
		var ep EventsPerBatch
		if err := rows.Scan(&ep.BatchCode, &ep.EventCount); err != nil {
			return nil, err
		}
		result = append(result, ep)
	}
	return result, rows.Err()
}

func (r *AnalyticsRepo) AvgHomogenizerTemp(ctx context.Context, limit, userID int) ([]AvgTempPerBatch, error) {
	var (
		rows *sql.Rows
		err  error
	)
	base := `
		SELECT b.batch_code,
			ROUND(AVG(t.value)::NUMERIC, 1),
			ROUND(MIN(t.value)::NUMERIC, 1),
			ROUND(MAX(t.value)::NUMERIC, 1)
		FROM telemetry t
		JOIN sensors s ON s.id = t.sensor_id AND s.sensor_code = 'MP-TEMP-03'
		JOIN batches b ON b.id = t.batch_id
		WHERE t.stage_key IN ('emulsifying_speed_2', 'emulsifying_speed_3')`
	if userID == 0 {
		rows, err = r.db.QueryContext(ctx, base+`
			GROUP BY b.batch_code ORDER BY b.created_at DESC LIMIT $1`, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, base+`
			AND (b.registered_by = $2 OR b.operator_id = $2)
			GROUP BY b.batch_code ORDER BY b.created_at DESC LIMIT $1`, limit, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("avg homogenizer temp: %w", err)
	}
	defer rows.Close()
	var result []AvgTempPerBatch
	for rows.Next() {
		var a AvgTempPerBatch
		if err := rows.Scan(&a.BatchCode, &a.AvgTemp, &a.MinTemp, &a.MaxTemp); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
