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
	GetLatestBySensorCode(ctx context.Context, sensorCode string) (*domain.NormalizedTelemetry, error)
	GetEquipmentStatus(ctx context.Context, equipmentCode string) (*domain.EquipmentStatus, error)
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

type EquipmentStatusResponse struct {
	EquipmentCode string                 `json:"equipment_code"`
	PLCOnline     bool                   `json:"plc_online"`
	Ready         bool                   `json:"ready"`
	LastSeenAt    time.Time              `json:"last_seen_at"`
	Sensors       []SensorStatusResponse `json:"sensors"`
}

type SensorStatusResponse struct {
	SensorCode string    `json:"sensor_code"`
	Online     bool      `json:"online"`
	LastSeenAt time.Time `json:"last_seen_at"`
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

func (h *TelemetryHandler) CurrentSensor(w http.ResponseWriter, r *http.Request) {
	sensorCode := r.PathValue("code")
	reading, err := h.svc.GetLatestBySensorCode(r.Context(), sensorCode)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTelemetryNotFound):
			http.Error(w, "telemetry not found", http.StatusNotFound)
		default:
			http.Error(w, "failed to get sensor telemetry", http.StatusInternalServerError)
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

func (h *TelemetryHandler) EquipmentStatus(w http.ResponseWriter, r *http.Request) {
	equipmentCode := r.PathValue("code")

	status, err := h.svc.GetEquipmentStatus(r.Context(), equipmentCode)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEquipmentNotFound):
			http.Error(w, "equipment status not found", http.StatusNotFound)
		default:
			http.Error(w, "failed to get equipment status", http.StatusInternalServerError)
		}
		return
	}

	resp := EquipmentStatusResponse{
		EquipmentCode: status.EquipmentCode,
		PLCOnline:     status.PLCOnline,
		Ready:         status.Ready,
		LastSeenAt:    status.LastSeenAt,
		Sensors:       make([]SensorStatusResponse, len(status.Sensors)),
	}
	for i, sensor := range status.Sensors {
		resp.Sensors[i] = SensorStatusResponse{
			SensorCode: sensor.SensorCode,
			Online:     sensor.Online,
			LastSeenAt: sensor.LastSeenAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
