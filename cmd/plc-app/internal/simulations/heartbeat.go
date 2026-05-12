package simulations

import (
	"log/slog"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/cmd/plc-app/internal/plc"
)

const equipmentStatusTopic = "ebr/equipment/VEH-001/status"

type EquipmentStatusPayload struct {
	EquipmentCode string                `json:"equipment_code"`
	PLCOnline     bool                  `json:"plc_online"`
	Sensors       []SensorStatusPayload `json:"sensors"`
}

type SensorStatusPayload struct {
	SensorCode string `json:"sensor_code"`
	Online     bool   `json:"online"`
}

func EquipmentHeartbeat(plcServer *plc.PLCServer) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		if err := PublishEquipmentReady(plcServer); err != nil {
			slog.Warn("failed to publish equipment heartbeat", "err", err)
		}
		<-ticker.C
	}
}

func PublishEquipmentReady(plcServer *plc.PLCServer) error {
	payload := EquipmentStatusPayload{
		EquipmentCode: "VEH-001",
		PLCOnline:     true,
		Sensors: []SensorStatusPayload{
			{SensorCode: "WP-WEIGHT-01", Online: true},
			{SensorCode: "WP-TEMP-01", Online: true},
			{SensorCode: "WP-MIXER-01", Online: true},
			{SensorCode: "OP-WEIGHT-01", Online: true},
			{SensorCode: "OP-TEMP-01", Online: true},
			{SensorCode: "OP-MIXER-01", Online: true},
			{SensorCode: "MP-VACUUM-01", Online: true},
			{SensorCode: "MP-TEMP-01", Online: true},
			{SensorCode: "MP-HOMOG-01", Online: true},
			{SensorCode: "MP-SCRAPER-01", Online: true},
			{SensorCode: "MP-WEIGHT-01", Online: true},
		},
	}

	return plcServer.PublishJSON(equipmentStatusTopic, payload)
}
