package domain

import (
	"errors"
	"time"
)

type SensorMeta struct {
	Topic         string
	EquipmentCode string
	SensorCode    string
	ParameterType string
	Unit          string
	Scale         float64
	Offset        float64
}

type NormalizedTelemetry struct {
	Topic         string
	EquipmentCode string
	SensorCode    string
	ParameterType string
	Value         float64
	Unit          string
	MeasuredAt    time.Time
}

type EquipmentStatus struct {
	EquipmentCode string
	PLCOnline     bool
	Ready         bool
	LastSeenAt    time.Time
	Sensors       []SensorStatus
}

type SensorStatus struct {
	SensorCode string
	Online     bool
	LastSeenAt time.Time
}

type TelemetryRecord struct {
	ID         int64
	BatchID    int
	SensorID   int
	SensorCode string
	StageKey   string
	Value      float64
	RecordedAt time.Time
}

var (
	ErrUnknownTelemetryTopic = errors.New("unknown telemetry topic")
	ErrInvalidTelemetryValue = errors.New("invalid telemetry value")
	ErrTelemetryNotFound     = errors.New("telemetry not found")
	ErrEquipmentNotFound     = errors.New("equipment not found")
)
