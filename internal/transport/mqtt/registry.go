package mqtt

import (
	"context"
	"log/slog"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type Subscription struct {
	Topic   string
	Handler func(topic string, payload []byte)
}

type TelemetryProcessor interface {
	ProcessRawTelemetry(ctx context.Context, topic string, payload []byte) (*domain.NormalizedTelemetry, error)
}

func NewTopicRegistry(processor TelemetryProcessor) []Subscription {
	return []Subscription{
		{
			Topic:   "ebr/sensor/weighing_scale_01",
			Handler: handleTelemetry(processor),
		},
	}
}

func handleTelemetry(processor TelemetryProcessor) func(topic string, payload []byte) {
	return func(topic string, payload []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		reading, err := processor.ProcessRawTelemetry(ctx, topic, payload)
		if err != nil {
			slog.Warn("telemetry processing failed", "topic", topic, "payload", string(payload), "err", err)
			return
		}

		slog.Info("telemetry normalized",
			"topic", reading.Topic,
			"equipment", reading.EquipmentCode,
			"sensor", reading.SensorCode,
			"type", reading.ParameterType,
			"value", reading.Value,
			"unit", reading.Unit,
		)
	}
}
