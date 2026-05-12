package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ReportRepo struct {
	db *sql.DB
}

func NewReportRepo(db *sql.DB) *ReportRepo {
	return &ReportRepo{db: db}
}

type BatchReportMeta struct {
	ID          int
	BatchID     int
	BatchCode   string
	BatchStatus string
	GeneratedBy int
	GeneratedAt time.Time
}

func (r *ReportRepo) SaveReport(ctx context.Context, batchID int, generatedBy int, html string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO batch_reports (batch_id, generated_by, html_content)
		VALUES ($1, $2, $3)
		ON CONFLICT (batch_id) DO UPDATE
		SET html_content = EXCLUDED.html_content,
		    generated_by = EXCLUDED.generated_by,
		    generated_at = NOW()
	`, batchID, generatedBy, html)
	return err
}

func (r *ReportRepo) GetReport(ctx context.Context, batchID int) (string, error) {
	var html string
	err := r.db.QueryRowContext(ctx, `
		SELECT html_content FROM batch_reports WHERE batch_id = $1
	`, batchID).Scan(&html)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("report not found")
		}
		return "", fmt.Errorf("get report: %w", err)
	}
	return html, nil
}

type BatchParticipants struct {
	RegisteredByCode string
	RegisteredByName string
	ProcessOpCode    string
	ProcessOpName    string
}

func (r *ReportRepo) GetBatchParticipants(ctx context.Context, batchCode string) (*BatchParticipants, error) {
	var p BatchParticipants
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(ru.user_code, '—'),  COALESCE(ru.full_name, '—'),
			COALESCE(ou.user_code, '—'),  COALESCE(ou.full_name, '—')
		FROM batches b
		LEFT JOIN users ru ON ru.id = b.registered_by
		LEFT JOIN users ou ON ou.id = b.operator_id
		WHERE b.batch_code = $1
	`, batchCode).Scan(
		&p.RegisteredByCode, &p.RegisteredByName,
		&p.ProcessOpCode, &p.ProcessOpName,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

type BatchEquipment struct {
	EquipmentCode string
	EquipmentName string
}

func (r *ReportRepo) GetBatchEquipment(ctx context.Context, batchCode string) (*BatchEquipment, error) {
	var eq BatchEquipment
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(e.equipment_code,'—'), COALESCE(e.name,'—')
		FROM batches b
		LEFT JOIN equipment e ON e.id = b.equipment_id
		WHERE b.batch_code = $1
	`, batchCode).Scan(&eq.EquipmentCode, &eq.EquipmentName)
	if err != nil {
		return nil, err
	}
	return &eq, nil
}

type UserShort struct {
	UserCode string
	FullName string
}

func (r *ReportRepo) GetUsersByIDs(ctx context.Context, ids []int) (map[int]UserShort, error) {
	result := make(map[int]UserShort)
	if len(ids) == 0 {
		return result, nil
	}
	// Build $1,$2,... placeholders
	args := make([]any, len(ids))
	placeholders := ""
	for i, id := range ids {
		args[i] = id
		if i > 0 {
			placeholders += ","
		}
		placeholders += fmt.Sprintf("$%d", i+1)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_code, full_name FROM users WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var u UserShort
		if err := rows.Scan(&id, &u.UserCode, &u.FullName); err != nil {
			continue
		}
		result[id] = u
	}
	return result, rows.Err()
}

func (r *ReportRepo) ListReports(ctx context.Context) ([]BatchReportMeta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT br.id, br.batch_id, b.batch_code, b.status, br.generated_by, br.generated_at
		FROM batch_reports br
		JOIN batches b ON b.id = br.batch_id
		ORDER BY br.generated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	var reports []BatchReportMeta
	for rows.Next() {
		var m BatchReportMeta
		if err := rows.Scan(&m.ID, &m.BatchID, &m.BatchCode, &m.BatchStatus, &m.GeneratedBy, &m.GeneratedAt); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		reports = append(reports, m)
	}
	return reports, rows.Err()
}

// ListReportsByOperator returns reports for batches registered by a specific operator.
func (r *ReportRepo) ListReportsByOperator(ctx context.Context, operatorID int) ([]BatchReportMeta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT br.id, br.batch_id, b.batch_code, b.status, br.generated_by, br.generated_at
		FROM batch_reports br
		JOIN batches b ON b.id = br.batch_id
		WHERE b.registered_by = $1 OR b.operator_id = $1
		ORDER BY br.generated_at DESC
	`, operatorID)
	if err != nil {
		return nil, fmt.Errorf("list operator reports: %w", err)
	}
	defer rows.Close()

	var reports []BatchReportMeta
	for rows.Next() {
		var m BatchReportMeta
		if err := rows.Scan(&m.ID, &m.BatchID, &m.BatchCode, &m.BatchStatus, &m.GeneratedBy, &m.GeneratedAt); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		reports = append(reports, m)
	}
	return reports, rows.Err()
}
