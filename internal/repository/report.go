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

func (r *ReportRepo) ListReports(ctx context.Context) ([]BatchReportMeta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT br.id, br.batch_id, b.batch_code, br.generated_by, br.generated_at
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
		if err := rows.Scan(&m.ID, &m.BatchID, &m.BatchCode, &m.GeneratedBy, &m.GeneratedAt); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		reports = append(reports, m)
	}
	return reports, rows.Err()
}
