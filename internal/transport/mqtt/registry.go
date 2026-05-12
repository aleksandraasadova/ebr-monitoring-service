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
	ProcessEquipmentStatus(ctx context.Context, topic string, payload []byte) (*domain.EquipmentStatus, error)
}

var sensorTopics = []string{
	"ebr/sensor/weighing_scale_01",
	"ebr/equipment/VEH-001/sensor/water_pot_weight",
	"ebr/equipment/VEH-001/sensor/water_pot_temp",
	"ebr/equipment/VEH-001/sensor/water_pot_mixer_rpm",
	"ebr/equipment/VEH-001/sensor/oil_pot_weight",
	"ebr/equipment/VEH-001/sensor/oil_pot_temp",
	"ebr/equipment/VEH-001/sensor/oil_pot_mixer_rpm",
	"ebr/equipment/VEH-001/sensor/main_pot_vacuum",
	"ebr/equipment/VEH-001/sensor/main_pot_temp",
	"ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm",
	"ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm",
	"ebr/equipment/VEH-001/sensor/main_pot_weight",
}

func NewTopicRegistry(processor TelemetryProcessor) []Subscription {
	subs := make([]Subscription, 0, len(sensorTopics)+1)
	for _, topic := range sensorTopics {
		subs = append(subs, Subscription{Topic: topic, Handler: handleTelemetry(processor)})
	}
	subs = append(subs, Subscription{
		Topic:   "ebr/equipment/VEH-001/status",
		Handler: handleEquipmentStatus(processor),
	})
	return subs
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

func handleEquipmentStatus(processor TelemetryProcessor) func(topic string, payload []byte) {
	return func(topic string, payload []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		status, err := processor.ProcessEquipmentStatus(ctx, topic, payload)
		if err != nil {
			slog.Warn("equipment status processing failed", "topic", topic, "payload", string(payload), "err", err)
			return
		}

		slog.Info("equipment status updated",
			"topic", topic,
			"equipment", status.EquipmentCode,
			"plc_online", status.PLCOnline,
			"ready", status.Ready,
			"sensors", len(status.Sensors),
		)
	}
}
