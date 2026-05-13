package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aleksandraasadova/ebr-monitoring-service/cmd/plc-app/internal/plc"
	"github.com/aleksandraasadova/ebr-monitoring-service/cmd/plc-app/internal/simulations"
)

const (
	modbusAddr = "localhost:1502"
	mqttBroker = "tcp://localhost:1883"
	clientID   = "plc-simulator"
)

var registerMap = map[int]string{
	0: "ebr/sensor/weighing_scale_01",
}

/*
1. Запись в регистр ПЛК
2. Публикация в MQTT
*/
func main() {

	plcServer, err := plc.NewPLCServer(modbusAddr, mqttBroker, clientID, registerMap)
	if err != nil {
		slog.Error("failed to start PLC server", "err", err)
		return
	}
	defer plcServer.Mb.Close()
	slog.Info("PLC server started", "address", modbusAddr)
	go simulations.EquipmentHeartbeat(plcServer)

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\n> Enter command: ")
		if !scanner.Scan() {
			break
		}
		cmd := strings.TrimSpace(scanner.Text())

		switch cmd {
		case "1":
			fmt.Println("Starting weighing...")
			go simulations.Weighing(plcServer) // запускаем в фоне, сервер продолжает ждать ввод
		case "2":
			fmt.Println("Publishing equipment ready heartbeat...")
			if err := simulations.PublishEquipmentReady(plcServer); err != nil {
				slog.Error("failed to publish equipment heartbeat", "err", err)
				continue
			}
			fmt.Println("Equipment heartbeat sent: VEH-001 online")
		case "3":
			fmt.Println("Starting emulsification process simulation (18 stages × 5 min)...")
			go simulations.Emulsification(plcServer)
		case "4":
			fmt.Println("Simulating oil pot SENSOR FAILURE (OP-TEMP-02 → 92°C for 2 min)...")
			go simulations.SimulateOilOverheat(plcServer)
		case "5":
			fmt.Println("Simulating oil pot SENSOR RECOVERY (back to 80°C)...")
			go simulations.SimulateOilRecovery(plcServer)
		case "q", "quit", "exit":
			fmt.Println("Shutting down...")
			return
		default:
			if cmd != "" {
				fmt.Println("Unknown command. Use 1 (weighing), 2 (heartbeat), 3 (emulsification), 4 (overheat test), 5 (recovery), q (quit)")
			}
		}
	}
}
