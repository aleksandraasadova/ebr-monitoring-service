package simulations

import (
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/cmd/plc-app/internal/plc"
	"github.com/aleksandraasadova/ebr-monitoring-service/cmd/plc-app/internal/sensor"
)

var weighingTargets = []uint16{8000, 8000, 300, 400, 500}

const weighingRegisterAddr = 0

func Weighing(plcServer *plc.PLCServer) {
	scale := sensor.Scale{}
	for _, target := range weighingTargets {
		scale.Reset()

		for {
			current := scale.SimulateStep(target)
			plcServer.WriteRegister(weighingRegisterAddr, current)

			if current == target {
				break
			}
			time.Sleep(3 * time.Second)
		}
		time.Sleep(10 * time.Second)
	}
}
