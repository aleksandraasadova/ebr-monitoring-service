package service

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/repository"
)

type reportRepo interface {
	SaveReport(ctx context.Context, batchID int, generatedBy int, html string) error
	GetReport(ctx context.Context, batchID int) (string, error)
	ListReports(ctx context.Context) ([]repository.BatchReportMeta, error)
}

type reportBatchRepo interface {
	GetByStatus(ctx context.Context, status string) ([]domain.Batch, error)
	GetWeighingLogByBatchCode(ctx context.Context, batchCode string) ([]domain.WeighingLogItem, error)
}

type reportProcessRepo interface {
	GetStagesByBatchID(ctx context.Context, batchID int) ([]domain.BatchStage, error)
	GetBatchIDByCode(ctx context.Context, batchCode string) (int, error)
}

type reportEventRepo interface {
	GetEventsByBatchID(ctx context.Context, batchID int) ([]domain.Event, error)
}

type reportTelemetryRepo interface {
	GetStageAggregates(ctx context.Context, batchID int, stageKey string) ([]repository.StageAggregate, error)
}

type ReportService struct {
	reportRepo    reportRepo
	batchRepo     reportBatchRepo
	processRepo   reportProcessRepo
	eventRepo     reportEventRepo
	telemetryRepo reportTelemetryRepo
	recipeRepo    recipeRepo
}

func NewReportService(
	rr reportRepo,
	br reportBatchRepo,
	pr reportProcessRepo,
	er reportEventRepo,
	tr reportTelemetryRepo,
	recr recipeRepo,
) *ReportService {
	return &ReportService{
		reportRepo:    rr,
		batchRepo:     br,
		processRepo:   pr,
		eventRepo:     er,
		telemetryRepo: tr,
		recipeRepo:    recr,
	}
}

func (s *ReportService) GenerateAndSave(ctx context.Context, batchCode string, generatedBy int) (string, error) {
	batchID, err := s.processRepo.GetBatchIDByCode(ctx, batchCode)
	if err != nil {
		return "", fmt.Errorf("get batch: %w", err)
	}

	// Collect all data
	batches, _ := s.batchRepo.GetByStatus(ctx, "completed")
	var batch domain.Batch
	for _, b := range batches {
		if b.Code == batchCode {
			batch = b
			break
		}
	}
	// Try other statuses if not found in completed
	if batch.ID == 0 {
		for _, status := range []string{"in_process", "cancelled"} {
			bs, _ := s.batchRepo.GetByStatus(ctx, status)
			for _, b := range bs {
				if b.Code == batchCode {
					batch = b
					break
				}
			}
			if batch.ID != 0 {
				break
			}
		}
	}

	weighingItems, _ := s.batchRepo.GetWeighingLogByBatchCode(ctx, batchCode)
	stages, _ := s.processRepo.GetStagesByBatchID(ctx, batchID)
	events, _ := s.eventRepo.GetEventsByBatchID(ctx, batchID)

	type stageReport struct {
		Stage      domain.BatchStage
		Aggregates []repository.StageAggregate
		Events     []domain.Event
	}
	var stageReports []stageReport
	for _, st := range stages {
		aggs, _ := s.telemetryRepo.GetStageAggregates(ctx, batchID, st.StageKey)
		var stageEvents []domain.Event
		for _, e := range events {
			if e.StageKey == st.StageKey {
				stageEvents = append(stageEvents, e)
			}
		}
		stageReports = append(stageReports, stageReport{Stage: st, Aggregates: aggs, Events: stageEvents})
	}

	data := map[string]any{
		"Batch":         batch,
		"WeighingItems": weighingItems,
		"StageReports":  stageReports,
		"GeneratedAt":   time.Now().UTC(),
		"BatchCode":     batchCode,
	}

	html, err := renderReportHTML(data)
	if err != nil {
		return "", fmt.Errorf("render report: %w", err)
	}

	if err := s.reportRepo.SaveReport(ctx, batchID, generatedBy, html); err != nil {
		return "", fmt.Errorf("save report: %w", err)
	}
	return html, nil
}

func (s *ReportService) GetReport(ctx context.Context, batchCode string) (string, error) {
	batchID, err := s.processRepo.GetBatchIDByCode(ctx, batchCode)
	if err != nil {
		return "", err
	}
	return s.reportRepo.GetReport(ctx, batchID)
}

func (s *ReportService) ListReports(ctx context.Context) ([]repository.BatchReportMeta, error) {
	return s.reportRepo.ListReports(ctx)
}

func renderReportHTML(data map[string]any) (string, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			return t.Format("02.01.2006 15:04:05")
		},
		"formatTimePtr": func(t *time.Time) string {
			if t == nil || t.IsZero() {
				return "—"
			}
			return t.Format("02.01.2006 15:04:05")
		},
		"deref": func(v *float64) string {
			if v == nil {
				return "—"
			}
			return fmt.Sprintf("%.2f", *v)
		},
	}).Parse(reportHTMLTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const reportHTMLTemplate = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<title>Протокол партии {{.BatchCode}}</title>
<style>
body{font-family:Arial,sans-serif;margin:40px;color:#1a1a2e;font-size:13px}
h1{color:#12205c;border-bottom:2px solid #f5c04f;padding-bottom:8px}
h2{color:#1c2b3a;margin-top:30px;font-size:15px}
table{width:100%;border-collapse:collapse;margin-top:10px}
th{background:#12205c;color:#fff;padding:8px;text-align:left;font-size:12px}
td{padding:6px 8px;border-bottom:1px solid #ddd}
tr:nth-child(even){background:#f8f9fa}
.ok{color:#27ae60;font-weight:bold}
.warn{color:#e67e22;font-weight:bold}
.crit{color:#e74c3c;font-weight:bold}
.meta{display:grid;grid-template-columns:1fr 1fr;gap:10px;margin:16px 0}
.meta-item{background:#f0f4ff;padding:10px;border-radius:4px}
.meta-label{font-size:11px;color:#666;text-transform:uppercase}
.meta-value{font-size:14px;font-weight:bold;color:#12205c}
.stage-block{margin:20px 0;border-left:3px solid #12205c;padding-left:16px}
.stage-header{font-weight:bold;color:#12205c}
.stage-time{font-size:11px;color:#888}
footer{margin-top:40px;font-size:11px;color:#aaa;border-top:1px solid #eee;padding-top:10px}
</style>
</head>
<body>
<h1>Протокол технологического процесса</h1>

<div class="meta">
  <div class="meta-item"><div class="meta-label">Код партии</div><div class="meta-value">{{.BatchCode}}</div></div>
  <div class="meta-item"><div class="meta-label">Статус</div><div class="meta-value">{{.Batch.Status}}</div></div>
  <div class="meta-item"><div class="meta-label">Код рецептуры</div><div class="meta-value">{{.Batch.RecipeCode}}</div></div>
  <div class="meta-item"><div class="meta-label">Объём партии</div><div class="meta-value">{{.Batch.TargetVolumeL}} л</div></div>
  <div class="meta-item"><div class="meta-label">Зарегистрирована</div><div class="meta-value">{{formatTime .Batch.CreatedAt}}</div></div>
  <div class="meta-item"><div class="meta-label">Зарегистрировал</div><div class="meta-value">{{.Batch.RegisteredByCode}}</div></div>
</div>

<h2>Ингредиенты (журнал взвешивания)</h2>
<table>
  <tr><th>Ингредиент</th><th>Фаза</th><th>Норма (г)</th><th>Факт (г)</th><th>Контейнер</th><th>Взвесил</th><th>Время</th></tr>
  {{range .WeighingItems}}
  <tr>
    <td>{{.Ingredient}}</td>
    <td>{{.StageKey}}</td>
    <td>{{printf "%.2f" .RequiredQty}}</td>
    <td>{{deref .ActualQty}}</td>
    <td>{{.ContainerCode}}</td>
    <td>{{.WeighedByCode}}</td>
    <td>{{formatTimePtr .WeighedAt}}</td>
  </tr>
  {{end}}
</table>

<h2>Стадии технологического процесса</h2>
{{range .StageReports}}
<div class="stage-block">
  <div class="stage-header">{{.Stage.StageNumber}}. {{.Stage.StageName}}</div>
  <div class="stage-time">
    Начало: {{formatTime .Stage.StartedAt}}
    {{if .Stage.CompletedAt}} | Завершение: {{formatTimePtr .Stage.CompletedAt}}{{end}}
    {{if .Stage.SignedBy}} | Подписал оператор ID: {{.Stage.SignedBy}}{{end}}
  </div>
  {{if .Aggregates}}
  <table>
    <tr><th>Датчик</th><th>Ед.</th><th>Среднее</th><th>Мин</th><th>Макс</th></tr>
    {{range .Aggregates}}
    <tr><td>{{.SensorName}}</td><td>{{.Unit}}</td><td>{{printf "%.2f" .Avg}}</td><td>{{printf "%.2f" .Min}}</td><td>{{printf "%.2f" .Max}}</td></tr>
    {{end}}
  </table>
  {{end}}
  {{if .Events}}
  <table>
    <tr><th>Время</th><th>Тип</th><th>Серьёзность</th><th>Описание</th><th>Комментарий оператора</th></tr>
    {{range .Events}}
    <tr>
      <td>{{formatTime .OccurredAt}}</td>
      <td>{{.Type}}</td>
      <td class="{{if eq .Severity "critical"}}crit{{else if eq .Severity "warning"}}warn{{else}}ok{{end}}">{{.Severity}}</td>
      <td>{{.Description}}</td>
      <td>{{.Comment}}</td>
    </tr>
    {{end}}
  </table>
  {{end}}
</div>
{{end}}

<footer>Отчёт сгенерирован: {{formatTime .GeneratedAt}} | iObserve EBR</footer>
</body>
</html>`
