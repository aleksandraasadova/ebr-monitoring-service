package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/repository"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/middleware"
)

type reportService interface {
	GenerateAndSave(ctx context.Context, batchCode string, generatedBy int) (string, error)
	GetReport(ctx context.Context, batchCode string) (string, error)
	ListReports(ctx context.Context) ([]repository.BatchReportMeta, error)
	ListReportsByOperator(ctx context.Context, operatorID int) ([]repository.BatchReportMeta, error)
	CanAccessBatch(ctx context.Context, batchCode string, userID int) bool
}

type ReportHandler struct {
	svc reportService
}

func NewReportHandler(svc reportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

func (h *ReportHandler) GetOrGenerate(w http.ResponseWriter, r *http.Request) {
	batchCode := r.PathValue("code")

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if user.Role != "admin" && !h.svc.CanAccessBatch(r.Context(), batchCode, user.UserID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	html, err := h.svc.GetReport(r.Context(), batchCode)
	if err != nil {
		// Not yet generated — generate now
		html, err = h.svc.GenerateAndSave(r.Context(), batchCode, user.UserID)
		if err != nil {
			http.Error(w, "failed to generate report", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

func (h *ReportHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())

	var (
		reports []repository.BatchReportMeta
		err     error
	)
	if user.Role == "admin" {
		reports, err = h.svc.ListReports(r.Context())
	} else {
		reports, err = h.svc.ListReportsByOperator(r.Context(), user.UserID)
	}
	if err != nil {
		http.Error(w, "failed to list reports", http.StatusInternalServerError)
		return
	}

	resp := make([]ReportMetaResponse, len(reports))
	for i, rep := range reports {
		resp[i] = ReportMetaResponse{
			ID:          rep.ID,
			BatchCode:   rep.BatchCode,
			BatchStatus: rep.BatchStatus,
			GeneratedAt: rep.GeneratedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
