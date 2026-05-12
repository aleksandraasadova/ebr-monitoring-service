package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type TelemetryService struct {
	sensors   map[string]domain.SensorMeta
	mu        sync.RWMutex
	latest    map[string]domain.NormalizedTelemetry
	equipment map[string]domain.EquipmentStatus
}

func NewTelemetryService() *TelemetryService {
	return &TelemetryService{
		sensors: map[string]domain.SensorMeta{
			"ebr/sensor/weighing_scale_01": {
				Topic:         "ebr/sensor/weighing_scale_01",
				EquipmentCode: "SCALES-001",
				SensorCode:    "weighing_scale_01",
				ParameterType: "weight",
				Unit:          "g",
				Scale:         1,
				Offset:        0,
			},
		},
		latest:    make(map[string]domain.NormalizedTelemetry),
		equipment: make(map[string]domain.EquipmentStatus),
	}
}

func (s *TelemetryService) ProcessRawTelemetry(ctx context.Context, topic string, payload []byte) (*domain.NormalizedTelemetry, error) {
	_ = ctx

	meta, ok := s.sensors[topic]
	if !ok {
		return nil, domain.ErrUnknownTelemetryTopic
	}

	switch meta.ParameterType {
	case "weight":
		reading, err := normalizeNumericTelemetry(meta, payload)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.latest[reading.ParameterType] = *reading
		s.mu.Unlock()
		return reading, nil
	default:
		return nil, fmt.Errorf("unsupported telemetry parameter type %q", meta.ParameterType)
	}
}

func (s *TelemetryService) ProcessEquipmentStatus(ctx context.Context, topic string, payload []byte) (*domain.EquipmentStatus, error) {
	_ = ctx
	_ = topic

	var msg equipmentStatusMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidTelemetryValue, err)
	}
	if msg.EquipmentCode == "" {
		return nil, fmt.Errorf("%w: equipment_code is required", domain.ErrInvalidTelemetryValue)
	}

	lastSeenAt := time.Now().UTC()
	status := domain.EquipmentStatus{
		EquipmentCode: msg.EquipmentCode,
		PLCOnline:     msg.PLCOnline,
		LastSeenAt:    lastSeenAt,
		Sensors:       make([]domain.SensorStatus, len(msg.Sensors)),
	}

	ready := status.PLCOnline && len(msg.Sensors) > 0
	for i, sensor := range msg.Sensors {
		status.Sensors[i] = domain.SensorStatus{
			SensorCode: sensor.SensorCode,
			Online:     sensor.Online,
			LastSeenAt: lastSeenAt,
		}
		if !sensor.Online {
			ready = false
		}
	}
	status.Ready = ready

	s.mu.Lock()
	s.equipment[status.EquipmentCode] = status
	s.mu.Unlock()

	return &status, nil
}

func (s *TelemetryService) GetLatestTelemetry(ctx context.Context, parameterType string) (*domain.NormalizedTelemetry, error) {
	_ = ctx

	s.mu.RLock()
	reading, ok := s.latest[parameterType]
	s.mu.RUnlock()
	if !ok {
		return nil, domain.ErrTelemetryNotFound
	}
	return &reading, nil
}

func (s *TelemetryService) GetEquipmentStatus(ctx context.Context, equipmentCode string) (*domain.EquipmentStatus, error) {
	_ = ctx

	s.mu.RLock()
	status, ok := s.equipment[equipmentCode]
	s.mu.RUnlock()
	if !ok {
		return nil, domain.ErrEquipmentNotFound
	}
	return &status, nil
}

type equipmentStatusMessage struct {
	EquipmentCode string                `json:"equipment_code"`
	PLCOnline     bool                  `json:"plc_online"`
	Sensors       []sensorStatusMessage `json:"sensors"`
}

type sensorStatusMessage struct {
	SensorCode string `json:"sensor_code"`
	Online     bool   `json:"online"`
}

func normalizeNumericTelemetry(meta domain.SensorMeta, payload []byte) (*domain.NormalizedTelemetry, error) {
	raw := strings.TrimSpace(string(payload))
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", domain.ErrInvalidTelemetryValue, raw)
	}
	if value < 0 {
		return nil, fmt.Errorf("%w: negative value %v", domain.ErrInvalidTelemetryValue, value)
	}

	return &domain.NormalizedTelemetry{
		Topic:         meta.Topic,
		EquipmentCode: meta.EquipmentCode,
		SensorCode:    meta.SensorCode,
		ParameterType: meta.ParameterType,
		Value:         value*meta.Scale + meta.Offset,
		Unit:          meta.Unit,
		MeasuredAt:    time.Now().UTC(),
	}, nil
}
