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

var (
	ErrUnknownTelemetryTopic = errors.New("unknown telemetry topic")
	ErrInvalidTelemetryValue = errors.New("invalid telemetry value")
	ErrTelemetryNotFound     = errors.New("telemetry not found")
)
