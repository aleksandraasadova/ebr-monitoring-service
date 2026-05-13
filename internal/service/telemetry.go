package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type telemetryRepo interface {
	SaveReading(ctx context.Context, record *domain.TelemetryRecord) error
	GetSensorIDByCode(ctx context.Context, sensorCode string) (int, error)
}

// TelemetryBroadcaster broadcasts normalized telemetry to connected WebSocket clients.
type TelemetryBroadcaster interface {
	Broadcast(data []byte)
}

// EventCreator is implemented by ProcessService to allow TelemetryService to create events
// without a direct import cycle.
type EventCreator interface {
	CreateEventRaw(ctx context.Context, batchID int, stageKey, eventType, severity, description string) error
}

// TelemetryThreshold defines acceptable range for a sensor on given stages.
type TelemetryThreshold struct {
	SensorCode string
	StageKeys  []string
	Min        *float64
	Max        *float64
	Severity   string
	Label      string
}

var thresholds = func() []TelemetryThreshold {
	f := func(v float64) *float64 { return &v }
	return []TelemetryThreshold{
		{
			SensorCode: "WP-TEMP-01", StageKeys: []string{"water_pot_heating", "oil_pot_heating"},
			Min: f(75), Max: f(85), Severity: "critical",
			Label: "T водного котла вне диапазона 75–85 °C",
		},
		{
			SensorCode: "OP-TEMP-02", StageKeys: []string{"oil_pot_heating", "main_pot_oil_feeding"},
			Min: f(75), Max: f(85), Severity: "critical",
			Label: "T масляного котла вне диапазона 75–85 °C",
		},
		{
			SensorCode: "MP-TEMP-03", StageKeys: []string{"emulsifying_speed_2", "emulsifying_speed_3"},
			Min: f(75), Max: f(85), Severity: "critical",
			Label: "T основного котла вне диапазона 75–85 °C при эмульгировании",
		},
		{
			SensorCode: "MP-TEMP-03", StageKeys: []string{"additive_feeding"},
			Min: nil, Max: f(40), Severity: "critical",
			Label: "T основного котла > 40 °C при внесении добавок",
		},
		{
			SensorCode: "MP-HOMOG-01", StageKeys: []string{"emulsifying_speed_2", "emulsifying_speed_3"},
			Min: f(1800), Max: nil, Severity: "warning",
			Label: "Скорость гомогенизатора < 1800 об/мин при эмульгировании",
		},
	}
}()

// deviationState tracks when a threshold violation started for a specific sensor on a stage.
type deviationState struct {
	StartedAt  time.Time
	EventFired bool // true after first event is created
}

type TelemetryService struct {
	sensors      map[string]domain.SensorMeta
	sensorIDs    map[string]int
	mu           sync.RWMutex
	latest       map[string]domain.NormalizedTelemetry // key: sensorCode
	equipment    map[string]domain.EquipmentStatus
	repo         telemetryRepo
	broadcaster  TelemetryBroadcaster
	eventCreator EventCreator
	activeBatch  *int
	currentStage string
	lastSaved    map[string]time.Time
	deviations   map[string]deviationState // key: "sensorCode:stageKey"
}

func NewTelemetryService(repo telemetryRepo) *TelemetryService {
	return &TelemetryService{
		sensors:    buildSensorMap(),
		sensorIDs:  make(map[string]int),
		latest:     make(map[string]domain.NormalizedTelemetry),
		equipment:  make(map[string]domain.EquipmentStatus),
		repo:       repo,
		lastSaved:  make(map[string]time.Time),
		deviations: make(map[string]deviationState),
	}
}

func (s *TelemetryService) SetBroadcaster(b TelemetryBroadcaster) {
	s.mu.Lock()
	s.broadcaster = b
	s.mu.Unlock()
}

func (s *TelemetryService) SetEventCreator(ec EventCreator) {
	s.mu.Lock()
	s.eventCreator = ec
	s.mu.Unlock()
}

func (s *TelemetryService) SetActiveBatch(batchID *int) {
	s.mu.Lock()
	s.activeBatch = batchID
	if batchID == nil {
		s.currentStage = ""
		s.deviations = make(map[string]deviationState)
	}
	s.mu.Unlock()
}

func (s *TelemetryService) SetCurrentStage(stageKey string) {
	s.mu.Lock()
	s.currentStage = stageKey
	// Clear deviation state for the new stage so we start fresh
	for k := range s.deviations {
		if strings.HasSuffix(k, ":"+stageKey) {
			delete(s.deviations, k)
		}
	}
	s.mu.Unlock()
}

func (s *TelemetryService) ProcessRawTelemetry(ctx context.Context, topic string, payload []byte) (*domain.NormalizedTelemetry, error) {
	meta, ok := s.sensors[topic]
	if !ok {
		return nil, domain.ErrUnknownTelemetryTopic
	}

	reading, err := normalizeNumericTelemetry(meta, payload)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.latest[meta.SensorCode] = *reading
	batchID := s.activeBatch
	stageKey := s.currentStage
	lastSaved := s.lastSaved[meta.SensorCode]
	broadcaster := s.broadcaster
	s.mu.Unlock()

	// Broadcast via WebSocket
	if broadcaster != nil {
		if data, err := json.Marshal(reading); err == nil {
			broadcaster.Broadcast(data)
		}
	}

	// Persist to DB every 5 seconds if a batch is active
	if batchID != nil && time.Since(lastSaved) >= 5*time.Second {
		sensorID, err := s.getSensorID(ctx, meta.SensorCode)
		if err == nil {
			rec := &domain.TelemetryRecord{
				BatchID:    *batchID,
				SensorID:   sensorID,
				SensorCode: meta.SensorCode,
				StageKey:   stageKey,
				Value:      reading.Value,
			}
			if saveErr := s.repo.SaveReading(ctx, rec); saveErr != nil {
				slog.Warn("failed to persist telemetry", "sensor", meta.SensorCode, "err", saveErr)
			} else {
				s.mu.Lock()
				s.lastSaved[meta.SensorCode] = time.Now()
				s.mu.Unlock()
			}
		}
	}

	// Check thresholds and auto-create deviation events
	if batchID != nil && stageKey != "" {
		s.checkDeviations(ctx, reading, *batchID, stageKey)
	}

	return reading, nil
}

// checkDeviations inspects a reading against thresholds and fires events for sustained violations.
func (s *TelemetryService) checkDeviations(ctx context.Context, reading *domain.NormalizedTelemetry, batchID int, stageKey string) {
	violations := s.CheckThresholds(reading, stageKey)
	key := reading.SensorCode + ":" + stageKey

	s.mu.Lock()
	if len(violations) == 0 {
		delete(s.deviations, key)
		s.mu.Unlock()
		return
	}

	state, exists := s.deviations[key]
	if !exists {
		state = deviationState{StartedAt: time.Now()}
		s.deviations[key] = state
	}

	// Fire one event after 30 seconds of sustained deviation
	if !state.EventFired && time.Since(state.StartedAt) >= 30*time.Second {
		state.EventFired = true
		s.deviations[key] = state
		s.mu.Unlock()

		ec := s.eventCreator
		if ec == nil {
			return
		}

		// Find threshold label and severity
		severity := "warning"
		label := violations[0]
		for _, t := range thresholds {
			if t.SensorCode == reading.SensorCode {
				for _, sk := range t.StageKeys {
					if sk == stageKey {
						severity = t.Severity
						label = fmt.Sprintf("%s: текущее значение %.2f %s (стадия %s)", t.Label, reading.Value, reading.Unit, stageKey)
						break
					}
				}
			}
		}

		if err := ec.CreateEventRaw(ctx, batchID, stageKey, "alarm", severity, label); err != nil {
			slog.Warn("failed to create deviation event", "sensor", reading.SensorCode, "err", err)
		}
		return
	}
	s.mu.Unlock()
}

// CheckThresholds returns human-readable violation strings for a reading.
func (s *TelemetryService) CheckThresholds(reading *domain.NormalizedTelemetry, stageKey string) []string {
	var violations []string
	for _, t := range thresholds {
		if t.SensorCode != reading.SensorCode {
			continue
		}
		inStage := false
		for _, sk := range t.StageKeys {
			if sk == stageKey {
				inStage = true
				break
			}
		}
		if !inStage {
			continue
		}
		if t.Min != nil && reading.Value < *t.Min {
			violations = append(violations, fmt.Sprintf("%s: %.2f %s < min %.0f (%s)", reading.SensorCode, reading.Value, reading.Unit, *t.Min, t.Severity))
		}
		if t.Max != nil && reading.Value > *t.Max {
			violations = append(violations, fmt.Sprintf("%s: %.2f %s > max %.0f (%s)", reading.SensorCode, reading.Value, reading.Unit, *t.Max, t.Severity))
		}
	}
	return violations
}

func (s *TelemetryService) ProcessEquipmentStatus(ctx context.Context, topic string, payload []byte) (*domain.EquipmentStatus, error) {
	_ = topic

	var msg equipmentStatusMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidTelemetryValue, err)
	}
	if msg.EquipmentCode == "" {
		return nil, fmt.Errorf("%w: equipment_code is required", domain.ErrInvalidTelemetryValue)
	}

	lastSeenAt := time.Now().UTC()
	status := domain.EquipmentStatus{
		EquipmentCode: msg.EquipmentCode,
		PLCOnline:     msg.PLCOnline,
		LastSeenAt:    lastSeenAt,
		Sensors:       make([]domain.SensorStatus, len(msg.Sensors)),
	}

	ready := status.PLCOnline && len(msg.Sensors) > 0
	for i, sensor := range msg.Sensors {
		status.Sensors[i] = domain.SensorStatus{
			SensorCode: sensor.SensorCode,
			Online:     sensor.Online,
			LastSeenAt: lastSeenAt,
		}
		if !sensor.Online {
			ready = false
		}
	}
	status.Ready = ready

	s.mu.Lock()
	s.equipment[status.EquipmentCode] = status
	s.mu.Unlock()

	return &status, nil
}

func (s *TelemetryService) GetLatestTelemetry(ctx context.Context, parameterType string) (*domain.NormalizedTelemetry, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, reading := range s.latest {
		if reading.ParameterType == parameterType {
			r := reading
			return &r, nil
		}
	}
	return nil, domain.ErrTelemetryNotFound
}

func (s *TelemetryService) GetLatestBySensorCode(ctx context.Context, sensorCode string) (*domain.NormalizedTelemetry, error) {
	_ = ctx
	s.mu.RLock()
	reading, ok := s.latest[sensorCode]
	s.mu.RUnlock()
	if !ok {
		return nil, domain.ErrTelemetryNotFound
	}
	return &reading, nil
}

func (s *TelemetryService) GetLatestAll() map[string]domain.NormalizedTelemetry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]domain.NormalizedTelemetry, len(s.latest))
	for k, v := range s.latest {
		result[k] = v
	}
	return result
}

func (s *TelemetryService) GetEquipmentStatus(ctx context.Context, equipmentCode string) (*domain.EquipmentStatus, error) {
	_ = ctx
	s.mu.RLock()
	status, ok := s.equipment[equipmentCode]
	s.mu.RUnlock()
	if !ok {
		return nil, domain.ErrEquipmentNotFound
	}
	return &status, nil
}

func (s *TelemetryService) getSensorID(ctx context.Context, sensorCode string) (int, error) {
	s.mu.RLock()
	id, ok := s.sensorIDs[sensorCode]
	s.mu.RUnlock()
	if ok {
		return id, nil
	}
	id, err := s.repo.GetSensorIDByCode(ctx, sensorCode)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.sensorIDs[sensorCode] = id
	s.mu.Unlock()
	return id, nil
}

type equipmentStatusMessage struct {
	EquipmentCode string                `json:"equipment_code"`
	PLCOnline     bool                  `json:"plc_online"`
	Sensors       []sensorStatusMessage `json:"sensors"`
}

type sensorStatusMessage struct {
	SensorCode string `json:"sensor_code"`
	Online     bool   `json:"online"`
}

// normalizeNumericTelemetry parses a raw MQTT payload into a NormalizedTelemetry.
// Negative values are allowed (e.g. vacuum pressure in MPa).
func normalizeNumericTelemetry(meta domain.SensorMeta, payload []byte) (*domain.NormalizedTelemetry, error) {
	raw := strings.TrimSpace(string(payload))
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", domain.ErrInvalidTelemetryValue, raw)
	}
	return &domain.NormalizedTelemetry{
		Topic:         meta.Topic,
		EquipmentCode: meta.EquipmentCode,
		SensorCode:    meta.SensorCode,
		ParameterType: meta.ParameterType,
		Value:         value*meta.Scale + meta.Offset,
		Unit:          meta.Unit,
		MeasuredAt:    time.Now().UTC(),
	}, nil
}

func buildSensorMap() map[string]domain.SensorMeta {
	mk := func(topic, equipment, code, paramType, unit string) domain.SensorMeta {
		return domain.SensorMeta{Topic: topic, EquipmentCode: equipment, SensorCode: code, ParameterType: paramType, Unit: unit, Scale: 1, Offset: 0}
	}
	return map[string]domain.SensorMeta{
		"ebr/sensor/weighing_scale_01":                          mk("ebr/sensor/weighing_scale_01", "SCALES-001", "SCALE-WEIGHT-01", "weight", "g"),
		"ebr/equipment/VEH-001/sensor/water_pot_weight":         mk("ebr/equipment/VEH-001/sensor/water_pot_weight", "VEH-001", "WP-WEIGHT-01", "weight", "g"),
		"ebr/equipment/VEH-001/sensor/water_pot_temp":           mk("ebr/equipment/VEH-001/sensor/water_pot_temp", "VEH-001", "WP-TEMP-01", "temperature", "C"),
		"ebr/equipment/VEH-001/sensor/water_pot_mixer_rpm":      mk("ebr/equipment/VEH-001/sensor/water_pot_mixer_rpm", "VEH-001", "WP-MIXER-01", "mixer_rpm", "rpm"),
		"ebr/equipment/VEH-001/sensor/oil_pot_weight":           mk("ebr/equipment/VEH-001/sensor/oil_pot_weight", "VEH-001", "OP-WEIGHT-02", "weight", "g"),
		"ebr/equipment/VEH-001/sensor/oil_pot_temp":             mk("ebr/equipment/VEH-001/sensor/oil_pot_temp", "VEH-001", "OP-TEMP-02", "temperature", "C"),
		"ebr/equipment/VEH-001/sensor/oil_pot_mixer_rpm":        mk("ebr/equipment/VEH-001/sensor/oil_pot_mixer_rpm", "VEH-001", "OP-MIXER-02", "mixer_rpm", "rpm"),
		"ebr/equipment/VEH-001/sensor/main_pot_vacuum":          mk("ebr/equipment/VEH-001/sensor/main_pot_vacuum", "VEH-001", "MP-VACUUM-01", "vacuum", "MPa"),
		"ebr/equipment/VEH-001/sensor/main_pot_temp":            mk("ebr/equipment/VEH-001/sensor/main_pot_temp", "VEH-001", "MP-TEMP-03", "temperature", "C"),
		"ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm": mk("ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", "VEH-001", "MP-HOMOG-01", "homogenizer_rpm", "rpm"),
		"ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm":     mk("ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", "VEH-001", "MP-SCRAPER-01", "mixer_rpm", "rpm"),
		"ebr/equipment/VEH-001/sensor/main_pot_weight":          mk("ebr/equipment/VEH-001/sensor/main_pot_weight", "VEH-001", "MP-WEIGHT-03", "weight", "g"),
	}
}
