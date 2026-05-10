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
			fmt.Println("Starting process (multi-goroutine)...")
			//go runProcess(plcServer)
		case "q", "quit", "exit":
			fmt.Println("Shutting down...")
			return
		default:
			if cmd != "" {
				fmt.Println("Unknown command. Use 1, 2 or q")
			}
		}
	}
}
