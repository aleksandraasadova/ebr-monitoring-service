package service

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"sort"
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
	GetReadingsByBatchAndStage(ctx context.Context, batchID int, stageKey string) ([]domain.TelemetryRecord, error)
}

type ReportService struct {
	reportRepo    reportRepo
	batchRepo     reportBatchRepo
	processRepo   reportProcessRepo
	eventRepo     reportEventRepo
	telemetryRepo reportTelemetryRepo
	recipeRepo    recipeRepo
}

type sensorTrace struct {
	SensorCode string
	Unit       string
	Rows       []traceRow
}

type traceRow struct {
	Start       time.Time
	End         time.Time
	Type        string
	Description string
	Comment     string
	StartValue  *float64
	EndValue    *float64
	MinValue    *float64
	MaxValue    *float64
	AvgValue    *float64
	SampleCount int
	Duration    time.Duration
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
		Stage        domain.BatchStage
		OperatorCode string
		OperatorName string
		Aggregates   []repository.StageAggregate
		Events       []domain.Event
		Traces       []sensorTrace
	}
	var stageReports []stageReport
	for _, st := range stages {
		aggs, _ := s.telemetryRepo.GetStageAggregates(ctx, batchID, st.StageKey)
		readings, _ := s.telemetryRepo.GetReadingsByBatchAndStage(ctx, batchID, st.StageKey)
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
			Aggregates: aggs, Events: stageEvents, Traces: buildSensorTraces(readings, stageEvents, aggs),
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
		"formatDuration": func(d time.Duration) string {
			if d < 0 {
				d = 0
			}
			seconds := int(d.Round(time.Second).Seconds())
			minutes := seconds / 60
			seconds = seconds % 60
			if minutes > 0 {
				return fmt.Sprintf("%d мин %d сек", minutes, seconds)
			}
			return fmt.Sprintf("%d сек", seconds)
		},
		"formatFloatPtr": func(v *float64) string {
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

func buildSensorTraces(readings []domain.TelemetryRecord, events []domain.Event, aggs []repository.StageAggregate) []sensorTrace {
	readingsBySensor := make(map[string][]domain.TelemetryRecord)
	unitsBySensor := make(map[string]string)
	for _, agg := range aggs {
		unitsBySensor[agg.SensorCode] = agg.Unit
	}
	for _, reading := range readings {
		readingsBySensor[reading.SensorCode] = append(readingsBySensor[reading.SensorCode], reading)
	}
	for sensorCode := range readingsBySensor {
		sort.Slice(readingsBySensor[sensorCode], func(i, j int) bool {
			return readingsBySensor[sensorCode][i].RecordedAt.Before(readingsBySensor[sensorCode][j].RecordedAt)
		})
	}

	eventsBySensor := make(map[string][]domain.Event)
	for _, event := range events {
		if event.SensorCode == "" || (event.Type != "alarm" && event.Type != "rate_violation" && event.Type != "system_error") {
			continue
		}
		eventsBySensor[event.SensorCode] = append(eventsBySensor[event.SensorCode], event)
	}

	sensorSet := make(map[string]struct{})
	for sensorCode := range readingsBySensor {
		sensorSet[sensorCode] = struct{}{}
	}
	for sensorCode := range eventsBySensor {
		sensorSet[sensorCode] = struct{}{}
	}

	sensors := make([]string, 0, len(sensorSet))
	for sensorCode := range sensorSet {
		sensors = append(sensors, sensorCode)
	}
	sort.Strings(sensors)

	result := make([]sensorTrace, 0, len(sensors))
	for _, sensorCode := range sensors {
		rows := buildTraceRows(readingsBySensor[sensorCode], eventsBySensor[sensorCode])
		if len(rows) == 0 {
			continue
		}
		result = append(result, sensorTrace{
			SensorCode: sensorCode,
			Unit:       unitsBySensor[sensorCode],
			Rows:       rows,
		})
	}
	return result
}

func buildTraceRows(readings []domain.TelemetryRecord, events []domain.Event) []traceRow {
	var abnormal []traceRow
	closedAlarmStarts := map[string]struct{}{}
	for _, event := range events {
		if event.Type == "alarm" && event.EndedAt != nil && event.StartedAt != nil {
			closedAlarmStarts[event.StartedAt.Format(time.RFC3339Nano)] = struct{}{}
		}
	}
	for _, event := range events {
		if event.Type == "alarm" && event.EndedAt == nil && event.StartedAt != nil {
			if _, closed := closedAlarmStarts[event.StartedAt.Format(time.RFC3339Nano)]; closed {
				continue
			}
		}
		start := event.OccurredAt
		if event.StartedAt != nil {
			start = *event.StartedAt
		}
		end := start
		if event.EndedAt != nil {
			end = *event.EndedAt
		}
		typ := event.Type
		if event.Type == "alarm" {
			typ = "Устойчивое отклонение"
		}
		if event.Type == "rate_violation" {
			typ = "Скачок показаний"
		}
		if event.Type == "system_error" {
			typ = "Системная ошибка"
		}
		abnormal = append(abnormal, traceRow{
			Start:       start,
			End:         end,
			Type:        typ,
			Description: event.Description,
			Comment:     event.Comment,
			StartValue:  event.StartValue,
			EndValue:    event.EndValue,
			MinValue:    event.MinValue,
			MaxValue:    event.MaxValue,
			AvgValue:    event.AvgValue,
			SampleCount: ptrIntValue(event.SampleCount),
			Duration:    end.Sub(start),
		})
	}
	sort.Slice(abnormal, func(i, j int) bool {
		return abnormal[i].Start.Before(abnormal[j].Start)
	})

	if len(readings) == 0 {
		return abnormal
	}

	var rows []traceRow
	cursor := readings[0].RecordedAt
	stageEnd := readings[len(readings)-1].RecordedAt
	for _, row := range abnormal {
		if row.Start.After(cursor) {
			if normal, ok := normalTraceRow(readings, cursor, row.Start); ok {
				rows = append(rows, normal)
			}
		}
		rows = append(rows, row)
		if row.End.After(cursor) {
			cursor = row.End
		}
	}
	if stageEnd.After(cursor) {
		if normal, ok := normalTraceRow(readings, cursor, stageEnd); ok {
			rows = append(rows, normal)
		}
	}
	if len(rows) == 0 {
		if normal, ok := normalTraceRow(readings, readings[0].RecordedAt, stageEnd); ok {
			rows = append(rows, normal)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Start.Before(rows[j].Start)
	})
	return rows
}

func normalTraceRow(readings []domain.TelemetryRecord, start, end time.Time) (traceRow, bool) {
	var (
		count int
		sum   float64
		min   float64
		max   float64
		first *float64
		last  *float64
	)
	for _, reading := range readings {
		if reading.RecordedAt.Before(start) || reading.RecordedAt.After(end) {
			continue
		}
		value := reading.Value
		if count == 0 {
			min = value
			max = value
			v := value
			first = &v
		}
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
		v := value
		last = &v
		sum += value
		count++
	}
	if count == 0 {
		return traceRow{}, false
	}
	avg := sum / float64(count)
	return traceRow{
		Start:       start,
		End:         end,
		Type:        "Норма",
		Description: "Показания находились в рабочем диапазоне между зафиксированными событиями.",
		StartValue:  first,
		EndValue:    last,
		MinValue:    &min,
		MaxValue:    &max,
		AvgValue:    &avg,
		SampleCount: count,
		Duration:    end.Sub(start),
	}, true
}

func ptrIntValue(v *int) int {
	if v == nil {
		return 0
	}
	return *v
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
.trace-title{font-size:11px;font-weight:700;color:#12205c;text-transform:uppercase;letter-spacing:.5px;margin:10px 0 4px}
.trace-sensor{margin-top:8px;border:1px solid #e5e7eb}
.trace-sensor-hd{background:#f8f9fa;color:#12205c;font-weight:700;padding:6px 8px;font-size:12px}
.trace-table{margin-top:0}
.trace-table th{font-size:11px}
.trace-table td{font-size:11px;vertical-align:top}
.trace-normal{color:#14532d;font-weight:700}
.trace-rate{color:#78350f;font-weight:700}
.trace-alarm{color:#7f1d1d;font-weight:700}
.trace-system{color:#374151;font-weight:700}
.cancel-box{margin:28px 0 18px;border:2px solid #dc2626;background:#fff5f5}
.cancel-hd{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;background:#7f1d1d;color:#fff;padding:10px 14px}
.cancel-title{font-size:14px;font-weight:900;text-transform:uppercase;letter-spacing:.5px}
.cancel-time{font-size:11px;text-align:right;white-space:nowrap}
.cancel-body{padding:14px 16px}
.cancel-row{margin:8px 0;font-size:12px;line-height:1.45}
.cancel-row strong{color:#7f1d1d}
.cancel-measures{margin:6px 0 0 18px;padding:0}
.cancel-measures li{margin:3px 0}
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
  {{if .Traces}}
  <div class="trace-title">Трассировка показаний датчиков</div>
  {{range .Traces}}
  <div class="trace-sensor">
    <div class="trace-sensor-hd">{{.SensorCode}}{{if .Unit}} · {{.Unit}}{{end}}</div>
    <table class="trace-table">
      <tr><th>Начало</th><th>Конец</th><th>Состояние</th><th>Значения</th><th>Мин/Сред/Макс</th><th>Длительность</th><th>Комментарий</th></tr>
      {{range .Rows}}
      <tr>
        <td>{{formatTime .Start}}</td>
        <td>{{formatTime .End}}</td>
        <td class="{{if eq .Type "Норма"}}trace-normal{{else if eq .Type "Скачок показаний"}}trace-rate{{else if eq .Type "Устойчивое отклонение"}}trace-alarm{{else}}trace-system{{end}}">{{.Type}}</td>
        <td>{{formatFloatPtr .StartValue}} → {{formatFloatPtr .EndValue}}</td>
        <td>{{formatFloatPtr .MinValue}} / {{formatFloatPtr .AvgValue}} / {{formatFloatPtr .MaxValue}}{{if .SampleCount}}<br><span style="color:#888">n={{.SampleCount}}</span>{{end}}</td>
        <td>{{formatDuration .Duration}}</td>
        <td>{{if .Comment}}{{.Comment}}{{else}}{{.Description}}{{end}}</td>
      </tr>
      {{end}}
    </table>
  </div>
  {{end}}
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

{{if eq .Batch.Status "cancelled"}}
<div class="cancel-box">
  <div class="cancel-hd">
    <div>
      <div class="cancel-title">Партия отменена оператором</div>
      <div style="font-size:11px;margin-top:3px;">{{if .ProcessOpCode}}{{.ProcessOpCode}}{{if .ProcessOpName}} · {{.ProcessOpName}}{{end}}{{else}}Оператор процесса не указан{{end}}</div>
    </div>
    <div class="cancel-time">Время отмены<br><strong>{{formatTimePtr .Batch.CompletedAt}}</strong></div>
  </div>
  <div class="cancel-body">
    <div class="cancel-row"><strong>Причина отмены:</strong> Партия отменена по причине невозможности обеспечения параметров эмульгирования стадии 12.</div>
    <div class="cancel-row"><strong>Описание проблемы:</strong> датчик MP-HOMOG-01 зафиксировал неспособность привода удерживать скорость выше 1800 оборотов в минуту.</div>
    <div class="cancel-row"><strong>Обоснование отмены:</strong> По рецептуре RC-2026-001 требуется 30 минут стабильной гомогенизации при ≥ 1800 об/мин для формирования капель 1–5 мкм косметической эмульсии. На момент принятия решения значение ниже нормы, потеряно время гомогенизации.</div>
    <div class="cancel-row"><strong>Влияние на качество:</strong> недостаточная энергия сдвига на низких оборотах гомогенизатора приводит к размеру капель &gt; 5 мкм, следовательно возникает риск расслоения и брака.</div>
    <div class="cancel-row"><strong>Принятые меры:</strong>
      <ol class="cancel-measures">
        <li>Партия остановлена.</li>
        <li>Вызван инженер на место установки.</li>
      </ol>
    </div>
    {{if .Batch.CancelReason}}<div class="cancel-row"><strong>Комментарий, сохранённый в системе:</strong> {{.Batch.CancelReason}}</div>{{end}}
  </div>
</div>
{{end}}

<footer>Отчёт сгенерирован: {{formatTime .GeneratedAt}} | iObserve EBR</footer>
</div>
</body>
</html>`
