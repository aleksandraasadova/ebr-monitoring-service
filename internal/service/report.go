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
	ListReportsByOperator(ctx context.Context, operatorID int) ([]repository.BatchReportMeta, error)
	GetBatchEquipment(ctx context.Context, batchCode string) (*repository.BatchEquipment, error)
	GetUsersByIDs(ctx context.Context, ids []int) (map[int]repository.UserShort, error)
	GetBatchParticipants(ctx context.Context, batchCode string) (*repository.BatchParticipants, error)
}

type reportBatchRepo interface {
	GetByStatus(ctx context.Context, status string) ([]domain.Batch, error)
	GetWeighingLogByBatchCode(ctx context.Context, batchCode string) ([]domain.WeighingLogItem, error)
}

type reportProcessRepo interface {
	GetStagesByBatchID(ctx context.Context, batchID int) ([]domain.BatchStage, error)
	GetBatchIDByCode(ctx context.Context, batchCode string) (int, error)
	BatchBelongsToUser(ctx context.Context, batchCode string, userID int) bool
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
	equipment, _ := s.reportRepo.GetBatchEquipment(ctx, batchCode)
	participants, _ := s.reportRepo.GetBatchParticipants(ctx, batchCode)

	// Collect all user IDs from stages to resolve names
	userIDSet := map[int]struct{}{}
	for _, st := range stages {
		if st.SignedBy != nil {
			userIDSet[*st.SignedBy] = struct{}{}
		}
	}
	userIDs := make([]int, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}
	users, _ := s.reportRepo.GetUsersByIDs(ctx, userIDs)

	type stageReport struct {
		Stage         domain.BatchStage
		OperatorCode  string
		OperatorName  string
		Aggregates    []repository.StageAggregate
		Events        []domain.Event
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
		var opCode, opName string
		if st.SignedBy != nil {
			if u, ok := users[*st.SignedBy]; ok {
				opCode = u.UserCode
				opName = u.FullName
			}
		}
		stageReports = append(stageReports, stageReport{
			Stage: st, OperatorCode: opCode, OperatorName: opName,
			Aggregates: aggs, Events: stageEvents,
		})
	}

	var equipCode, equipName string
	if equipment != nil {
		equipCode = equipment.EquipmentCode
		equipName = equipment.EquipmentName
	}

	var regCode, regName, procCode, procName string
	if participants != nil {
		regCode = participants.RegisteredByCode
		regName = participants.RegisteredByName
		procCode = participants.ProcessOpCode
		procName = participants.ProcessOpName
	}

	data := map[string]any{
		"Batch":            batch,
		"WeighingItems":    weighingItems,
		"StageReports":     stageReports,
		"GeneratedAt":      time.Now().UTC(),
		"BatchCode":        batchCode,
		"EquipmentCode":    equipCode,
		"EquipmentName":    equipName,
		"RegisteredByCode": regCode,
		"RegisteredByName": regName,
		"ProcessOpCode":    procCode,
		"ProcessOpName":    procName,
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

func (s *ReportService) ListReportsByOperator(ctx context.Context, operatorID int) ([]repository.BatchReportMeta, error) {
	return s.reportRepo.ListReportsByOperator(ctx, operatorID)
}

func (s *ReportService) CanAccessBatch(ctx context.Context, batchCode string, userID int) bool {
	return s.processRepo.BatchBelongsToUser(ctx, batchCode, userID)
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
		"derefF": func(v *float64) float64 {
			if v == nil {
				return 0
			}
			return *v
		},
		"sub": func(a, b float64) string {
			diff := a - b
			if diff > 0 {
				return fmt.Sprintf("+%.2f", diff)
			}
			return fmt.Sprintf("%.2f", diff)
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
<script src="https://cdnjs.cloudflare.com/ajax/libs/html2pdf.js/0.10.1/html2pdf.bundle.min.js"></script>
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
.participants{margin:16px 0 24px;padding:12px 16px;background:#f8f9fa;border:1px solid #e0e4ed}
.part-title{font-size:12px;font-weight:700;color:#12205c;text-transform:uppercase;letter-spacing:.5px;margin-bottom:8px}
.stage-block{margin:20px 0;border-left:3px solid #12205c;padding-left:16px}
.stage-header{font-weight:bold;color:#12205c;font-size:14px}
.stage-time{font-size:11px;color:#888;margin-top:3px}
.event-block{margin:6px 0;padding:8px 12px;border-radius:3px;border-left:3px solid #ddd}
.event-crit{background:#fff0f0;border-color:#dc2626}
.event-warn{background:#fffbf0;border-color:#d97706}
.event-info{background:#f0f4ff;border-color:#2563eb}
.event-hd{display:flex;align-items:center;gap:8px;margin-bottom:4px}
.event-desc{font-size:12px;color:#1a1a2e}
.event-comment{font-size:12px;margin-top:4px;padding:6px 8px;background:rgba(0,0,0,.04);border-left:2px solid #12205c;color:#1c2b3a}
footer{margin-top:40px;font-size:11px;color:#aaa;border-top:1px solid #eee;padding-top:10px}
.toolbar{display:flex;gap:10px;margin-bottom:24px;align-items:center}
.btn-pdf{background:#12205c;color:#fff;border:none;padding:9px 20px;font-size:13px;font-weight:700;cursor:pointer;letter-spacing:.3px}
.btn-pdf:hover{opacity:.87}
.btn-pdf:disabled{opacity:.5;cursor:not-allowed}
.btn-print{background:#fff;color:#12205c;border:1px solid #12205c;padding:9px 20px;font-size:13px;font-weight:700;cursor:pointer}
.btn-close{background:#fff;color:#888;border:1px solid #ddd;padding:9px 20px;font-size:13px;font-weight:700;cursor:pointer}
.badge{display:inline-block;padding:2px 8px;font-size:11px;font-weight:700;border-radius:2px}
.badge-ok{background:#dcfce7;color:#14532d}
.badge-warn{background:#ffedd5;color:#78350f}
.badge-crit{background:#fee2e2;color:#7f1d1d}
@media print{
  .toolbar{display:none!important}
  body{margin:20px}
  .stage-block{break-inside:avoid}
  table{break-inside:avoid}
  h2{break-after:avoid}
}
</style>
</head>
<body>
<div class="toolbar">
  <button class="btn-pdf" id="btn-pdf" onclick="savePDF()">⬇ Скачать PDF</button>
  <button class="btn-print" onclick="window.print()">Распечатать</button>
  <button class="btn-close" onclick="window.close()">Закрыть</button>
</div>
<script>
function savePDF() {
  const btn = document.getElementById('btn-pdf');
  btn.disabled = true;
  btn.textContent = 'Генерация…';
  const el = document.getElementById('report-content');
  html2pdf().set({
    margin: [10, 10, 10, 10],
    filename: 'report-{{.BatchCode}}.pdf',
    image: { type: 'jpeg', quality: 0.98 },
    html2canvas: { scale: 2, useCORS: true, logging: false },
    jsPDF: { unit: 'mm', format: 'a4', orientation: 'portrait' },
    pagebreak: { mode: ['avoid-all', 'css', 'legacy'] }
  }).from(el).save().then(() => {
    btn.disabled = false;
    btn.textContent = '⬇ Скачать PDF';
  });
}
</script>
<div id="report-content">
<h1>Протокол технологического процесса</h1>

<div class="meta">
  <div class="meta-item"><div class="meta-label">Код партии</div><div class="meta-value">{{.BatchCode}}</div></div>
  <div class="meta-item"><div class="meta-label">Статус</div><div class="meta-value">{{.Batch.Status}}</div></div>
  <div class="meta-item"><div class="meta-label">Код рецептуры</div><div class="meta-value">{{.Batch.RecipeCode}}</div></div>
  <div class="meta-item"><div class="meta-label">Объём партии</div><div class="meta-value">{{.Batch.TargetVolumeL}} л</div></div>
  {{if .EquipmentCode}}
  <div class="meta-item"><div class="meta-label">Оборудование (код)</div><div class="meta-value">{{.EquipmentCode}}</div></div>
  <div class="meta-item"><div class="meta-label">Наименование оборудования</div><div class="meta-value">{{.EquipmentName}}</div></div>
  {{end}}
</div>

<div class="participants">
  <div class="part-title">Участники производственного процесса</div>
  <table>
    <tr><th>Роль</th><th>Код оператора</th><th>ФИО</th></tr>
    <tr><td>Регистрация партии — {{formatTime .Batch.CreatedAt}}</td><td class="mono">{{.RegisteredByCode}}</td><td>{{.RegisteredByName}}</td></tr>
    {{if .ProcessOpCode}}
    <tr><td>Ведение процесса (подписание стадий)</td><td class="mono">{{.ProcessOpCode}}</td><td>{{.ProcessOpName}}</td></tr>
    {{end}}
  </table>
</div>

<h2>Ингредиенты (журнал взвешивания)</h2>
<table>
  <tr><th>Ингредиент</th><th>Фаза</th><th>Норма (г)</th><th>Факт (г)</th><th>Откл.</th><th>Контейнер</th><th>Взвесил</th><th>Время</th></tr>
  {{range .WeighingItems}}
  <tr>
    <td>{{.Ingredient}}</td>
    <td>{{.StageKey}}</td>
    <td>{{printf "%.2f" .RequiredQty}}</td>
    <td>{{deref .ActualQty}}</td>
    <td>{{if .ActualQty}}{{printf "%.2f" (sub (derefF .ActualQty) .RequiredQty)}} г{{else}}—{{end}}</td>
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
    {{if .Stage.CompletedAt}} &nbsp;|&nbsp; Завершение: {{formatTimePtr .Stage.CompletedAt}}{{end}}
    {{if .OperatorCode}} &nbsp;|&nbsp; Подписал: <strong>{{.OperatorCode}}</strong>{{if .OperatorName}} ({{.OperatorName}}){{end}} в {{formatTimePtr .Stage.SignedAt}}{{end}}
  </div>
  {{if .Aggregates}}
  <table style="margin-top:8px;">
    <tr><th>Датчик</th><th>Ед.</th><th>Среднее</th><th>Мин</th><th>Макс</th></tr>
    {{range .Aggregates}}
    <tr><td>{{.SensorName}}</td><td>{{.Unit}}</td><td>{{printf "%.2f" .Avg}}</td><td>{{printf "%.2f" .Min}}</td><td>{{printf "%.2f" .Max}}</td></tr>
    {{end}}
  </table>
  {{end}}
  <div style="margin-top:8px;">
  <div style="font-size:11px;font-weight:700;color:#888;text-transform:uppercase;letter-spacing:.5px;margin-bottom:4px;">События стадии</div>
  {{if .Events}}
  {{range .Events}}
  <div class="event-block {{if eq .Severity "critical"}}event-crit{{else if eq .Severity "warning"}}event-warn{{else}}event-info{{end}}">
    <div class="event-hd">
      <span class="badge {{if eq .Severity "critical"}}badge-crit{{else if eq .Severity "warning"}}badge-warn{{else}}badge-ok{{end}}">{{.Severity}}</span>
      <span style="font-weight:700;">{{.Type}}</span>
      <span style="color:#888;font-size:11px;margin-left:auto;">{{formatTime .OccurredAt}}</span>
    </div>
    <div class="event-desc">{{.Description}}</div>
    {{if .Comment}}<div class="event-comment"><strong>Комментарий оператора:</strong> {{.Comment}}</div>{{end}}
  </div>
  {{end}}
  {{else}}
  <div style="font-size:12px;color:#aaa;padding:4px 0;">Событий не зафиксировано</div>
  {{end}}
  </div>
</div>
{{end}}

<footer>Отчёт сгенерирован: {{formatTime .GeneratedAt}} | iObserve EBR</footer>
</div>
</body>
</html>`
