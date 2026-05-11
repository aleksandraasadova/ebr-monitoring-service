package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type telemetryService interface {
	GetLatestTelemetry(ctx context.Context, parameterType string) (*domain.NormalizedTelemetry, error)
}

type TelemetryHandler struct {
	svc telemetryService
}

func NewTelemetryHandler(svc telemetryService) *TelemetryHandler {
	return &TelemetryHandler{svc: svc}
}

type CurrentTelemetryResponse struct {
	Topic         string    `json:"topic"`
	EquipmentCode string    `json:"equipment_code"`
	SensorCode    string    `json:"sensor_code"`
	ParameterType string    `json:"parameter_type"`
	Value         float64   `json:"value"`
	Unit          string    `json:"unit"`
	MeasuredAt    time.Time `json:"measured_at"`
}

func (h *TelemetryHandler) CurrentWeight(w http.ResponseWriter, r *http.Request) {
	reading, err := h.svc.GetLatestTelemetry(r.Context(), "weight")
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTelemetryNotFound):
			http.Error(w, "telemetry not found", http.StatusNotFound)
		default:
			http.Error(w, "failed to get current telemetry", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CurrentTelemetryResponse{
		Topic:         reading.Topic,
		EquipmentCode: reading.EquipmentCode,
		SensorCode:    reading.SensorCode,
		ParameterType: reading.ParameterType,
		Value:         reading.Value,
		Unit:          reading.Unit,
		MeasuredAt:    reading.MeasuredAt,
	})
}
