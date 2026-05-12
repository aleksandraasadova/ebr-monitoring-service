package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/repository"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/middleware"
)

type analyticsRepo interface {
	Summary(ctx context.Context, since time.Time, userID int) (*repository.AnalyticsSummary, error)
	BatchCountByPeriod(ctx context.Context, days, userID int) ([]repository.BatchCountByDay, error)
	CycleTimes(ctx context.Context, limit, userID int) ([]repository.CycleTime, error)
	StatusBreakdown(ctx context.Context, userID int) ([]repository.StatusBreakdown, error)
	EventsByStage(ctx context.Context, userID int) ([]repository.EventsByStage, error)
	EventsPerBatch(ctx context.Context, limit, userID int) ([]repository.EventsPerBatch, error)
	AvgHomogenizerTemp(ctx context.Context, limit, userID int) ([]repository.AvgTempPerBatch, error)
}

type AnalyticsHandler struct {
	repo analyticsRepo
}

func NewAnalyticsHandler(repo analyticsRepo) *AnalyticsHandler {
	return &AnalyticsHandler{repo: repo}
}

func (h *AnalyticsHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	daysStr := r.URL.Query().Get("days")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	user, _ := middleware.UserFromContext(r.Context())
	// Admin sees all batches; operator sees only their own
	userID := 0
	if user.Role == "operator" {
		userID = user.UserID
	}

	since := time.Now().UTC().AddDate(0, 0, -days)
	ctx := r.Context()

	summary, err := h.repo.Summary(ctx, since, userID)
	if err != nil {
		http.Error(w, "failed to load summary", http.StatusInternalServerError)
		return
	}

	batchByDay, _ := h.repo.BatchCountByPeriod(ctx, days, userID)
	cycleTimes, _ := h.repo.CycleTimes(ctx, 20, userID)
	statusBreakdown, _ := h.repo.StatusBreakdown(ctx, userID)
	eventsByStage, _ := h.repo.EventsByStage(ctx, userID)
	eventsPerBatch, _ := h.repo.EventsPerBatch(ctx, 10, userID)
	avgTemp, _ := h.repo.AvgHomogenizerTemp(ctx, 10, userID)

	resp := map[string]any{
		"summary":          summary,
		"batch_by_day":     batchByDay,
		"cycle_times":      cycleTimes,
		"status_breakdown": statusBreakdown,
		"events_by_stage":  eventsByStage,
		"events_per_batch": eventsPerBatch,
		"avg_homog_temp":   avgTemp,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
