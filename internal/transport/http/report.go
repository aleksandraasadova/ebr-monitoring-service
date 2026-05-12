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
	w.Header().Set("Content-Disposition", "attachment; filename=\"report-"+batchCode+".html\"")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

func (h *ReportHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	reports, err := h.svc.ListReports(r.Context())
	if err != nil {
		http.Error(w, "failed to list reports", http.StatusInternalServerError)
		return
	}

	resp := make([]ReportMetaResponse, len(reports))
	for i, r := range reports {
		resp[i] = ReportMetaResponse{
			ID:          r.ID,
			BatchCode:   r.BatchCode,
			GeneratedAt: r.GeneratedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
