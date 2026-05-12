package plc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/tbrandon/mbserver"
)

type PLCServer struct {
	Mb          *mbserver.Server
	mqtt        mqtt.Client
	registerMap map[int]string
}

func NewPLCServer(modbusAddr, mqttBroker, clientID string, registerMap map[int]string) (*PLCServer, error) {
	mb := mbserver.NewServer()
	if err := mb.ListenTCP(modbusAddr); err != nil {
		return nil, fmt.Errorf("failed to start Modbus server: %w", err)
	}
	slog.Info("PLC server started", "address", modbusAddr)

	opts := mqtt.NewClientOptions() // pointer
	opts.AddBroker(mqttBroker)
	opts.SetAutoReconnect(true)
	opts.SetClientID(clientID)
	opts.SetConnectRetry(true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}
	slog.Info("Connected to MQTT broker", "broker", mqttBroker)

	return &PLCServer{
		Mb:          mb,
		mqtt:        client,
		registerMap: registerMap,
	}, nil
}

func (p *PLCServer) WriteRegister(addr int, rawValue uint16) error {
	p.Mb.InputRegisters[addr] = rawValue // read-only

	topic, exists := p.registerMap[addr]
	if !exists {
		return nil
	}

	payload := strconv.FormatUint(uint64(rawValue), 10)
	token := p.mqtt.Publish(topic, 0, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		slog.Warn("MQTT publish timeout", "topic", topic, "value", rawValue)
		return fmt.Errorf("publish timeout")
	}
	if err := token.Error(); err != nil {
		slog.Error("MQTT publish failed", "topic", topic, "value", rawValue, "err", err)
		return fmt.Errorf("publish failed: %w", err)
	}
	slog.Debug("MQTT published", "topic", topic, "value", rawValue)
	return nil
}

func (p *PLCServer) PublishRaw(topic string, payload []byte) error {
	token := p.mqtt.Publish(topic, 0, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	return token.Error()
}

func (p *PLCServer) PublishJSON(topic string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	token := p.mqtt.Publish(topic, 0, false, data)
	if !token.WaitTimeout(5 * time.Second) {
		slog.Warn("MQTT publish timeout", "topic", topic)
		return fmt.Errorf("publish timeout")
	}
	if err := token.Error(); err != nil {
		slog.Error("MQTT publish failed", "topic", topic, "err", err)
		return fmt.Errorf("publish failed: %w", err)
	}
	slog.Debug("MQTT JSON published", "topic", topic, "payload", string(data))
	return nil
}
