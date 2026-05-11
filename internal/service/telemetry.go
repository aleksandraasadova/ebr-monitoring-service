package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type TelemetryService struct {
	sensors map[string]domain.SensorMeta
	mu      sync.RWMutex
	latest  map[string]domain.NormalizedTelemetry
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
		latest: make(map[string]domain.NormalizedTelemetry),
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
