package simulations

import (
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/cmd/plc-app/internal/plc"
	"github.com/aleksandraasadova/ebr-monitoring-service/cmd/plc-app/internal/sensor"
)

var weighingTargets = []uint16{200, 200, 200, 50, 20, 10, 10, 500, 500, 500, 400, 300, 200, 300, 50, 6560}

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
