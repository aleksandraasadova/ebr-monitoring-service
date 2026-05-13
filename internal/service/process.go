package service

import (
	"context"
	"fmt"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type ProcessService struct {
	processRepo processRepo
	eventRepo   eventRepo
	userRepo    userRepo
	telemetry   *TelemetryService
}

func NewProcessService(pr processRepo, er eventRepo, ur userRepo, ts *TelemetryService) *ProcessService {
	return &ProcessService{
		processRepo: pr,
		eventRepo:   er,
		userRepo:    ur,
		telemetry:   ts,
	}
}

// CreateEventRaw implements EventCreator so TelemetryService can fire events without circular imports.
func (s *ProcessService) CreateEventRaw(ctx context.Context, batchID int, stageKey, eventType, severity, description string) error {
	event := &domain.Event{
		BatchID:     batchID,
		StageKey:    stageKey,
		Type:        eventType,
		Severity:    severity,
		Description: description,
	}
	return s.eventRepo.CreateEvent(ctx, event)
}

// StartProcess checks equipment, verifies password, transitions batch to in_process, creates stage 1.
func (s *ProcessService) StartProcess(ctx context.Context, batchCode string, operatorID int, password string) error {
	if err := s.verifyPassword(ctx, operatorID, password); err != nil {
		return err
	}

	status, err := s.telemetry.GetEquipmentStatus(ctx, "VEH-001")
	if err != nil || !status.Ready {
		return domain.ErrEquipmentOffline
	}

	if err := s.processRepo.StartProcess(ctx, batchCode); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	batchID, err := s.processRepo.GetBatchIDByCode(ctx, batchCode)
	if err != nil {
		return fmt.Errorf("get batch id: %w", err)
	}

	s.telemetry.SetActiveBatch(&batchID)

	first := domain.AllStages[0]
	stage := &domain.BatchStage{
		BatchID:     batchID,
		StageNumber: first.Number,
		StageKey:    first.Key,
		StageName:   first.Name,
	}
	if err := s.processRepo.CreateStage(ctx, stage); err != nil {
		return err
	}
	s.telemetry.SetCurrentStage(first.Key)
	return nil
}

// SignStageTransition verifies password, checks conditions, signs current stage, opens next.
func (s *ProcessService) SignStageTransition(ctx context.Context, batchCode string, operatorID int, password, comment string) error {
	if err := s.verifyPassword(ctx, operatorID, password); err != nil {
		return err
	}

	batchID, err := s.processRepo.GetBatchIDByCode(ctx, batchCode)
	if err != nil {
		return err
	}

	if err := s.processRepo.CheckProcessOperator(ctx, batchCode, operatorID); err != nil {
		return err
	}

	current, err := s.processRepo.GetCurrentStageByBatchID(ctx, batchID)
	if err != nil {
		return err
	}

	if err := s.checkConditions(current.StageKey); err != nil {
		return err
	}

	if err := s.processRepo.SignAndCompleteStage(ctx, batchID, current.StageKey, operatorID, comment); err != nil {
		return err
	}

	nextNumber := current.StageNumber + 1
	if nextNumber > len(domain.AllStages) {
		if err := s.processRepo.CompleteBatch(ctx, batchCode); err != nil {
			return fmt.Errorf("complete batch: %w", err)
		}
		s.telemetry.SetActiveBatch(nil)
		return domain.ErrBatchCompleted
	}

	next := domain.AllStages[nextNumber-1]
	nextStage := &domain.BatchStage{
		BatchID:     batchID,
		StageNumber: next.Number,
		StageKey:    next.Key,
		StageName:   next.Name,
	}
	if err := s.processRepo.CreateStage(ctx, nextStage); err != nil {
		return err
	}
	s.telemetry.SetCurrentStage(next.Key)
	return nil
}

// checkConditions returns ErrConditionNotMet if any sensor condition is not satisfied.
func (s *ProcessService) checkConditions(stageKey string) error {
	stageDef, ok := domain.StageByKey(stageKey)
	if !ok || len(stageDef.Conditions) == 0 {
		return nil
	}
	for _, cond := range stageDef.Conditions {
		reading, err := s.telemetry.GetLatestBySensorCode(context.Background(), cond.SensorCode)
		if err != nil {
			return fmt.Errorf("%w: %s — нет данных с датчика", domain.ErrConditionNotMet, cond.Label)
		}
		if cond.MinValue != nil && reading.Value < *cond.MinValue {
			return fmt.Errorf("%w: %s (текущее: %.2f %s)", domain.ErrConditionNotMet, cond.Label, reading.Value, cond.Unit)
		}
		if cond.MaxValue != nil && reading.Value > *cond.MaxValue {
			return fmt.Errorf("%w: %s (текущее: %.2f %s)", domain.ErrConditionNotMet, cond.Label, reading.Value, cond.Unit)
		}
	}
	return nil
}

// GetStageConditions returns the real-time condition status for a given stage.
func (s *ProcessService) GetStageConditions(stageKey string) []domain.ConditionStatus {
	stageDef, ok := domain.StageByKey(stageKey)
	if !ok {
		return nil
	}
	result := make([]domain.ConditionStatus, 0, len(stageDef.Conditions))
	for _, cond := range stageDef.Conditions {
		cs := domain.ConditionStatus{
			SensorCode: cond.SensorCode,
			Label:      cond.Label,
			Unit:       cond.Unit,
		}
		reading, err := s.telemetry.GetLatestBySensorCode(context.Background(), cond.SensorCode)
		if err == nil {
			cs.Current = reading.Value
			cs.HasReading = true
			met := true
			if cond.MinValue != nil && reading.Value < *cond.MinValue {
				met = false
			}
			if cond.MaxValue != nil && reading.Value > *cond.MaxValue {
				met = false
			}
			cs.Met = met
		}
		result = append(result, cs)
	}
	return result
}

// CancelBatch cancels an in-process batch and logs a system event.
func (s *ProcessService) CancelBatch(ctx context.Context, batchCode string, operatorID int, reason string) error {
	batchID, err := s.processRepo.GetBatchIDByCode(ctx, batchCode)
	if err != nil {
		return err
	}
	if err := s.processRepo.CancelBatch(ctx, batchCode, reason); err != nil {
		return fmt.Errorf("cancel batch: %w", err)
	}
	s.telemetry.SetActiveBatch(nil)

	// Record cancellation as a system event
	_ = s.eventRepo.CreateEvent(ctx, &domain.Event{
		BatchID:     batchID,
		Type:        "system",
		Severity:    "critical",
		Description: "Партия отменена оператором. Причина: " + reason,
	})
	return nil
}

func (s *ProcessService) GetAllStages(ctx context.Context, batchCode string) ([]domain.BatchStage, error) {
	batchID, err := s.processRepo.GetBatchIDByCode(ctx, batchCode)
	if err != nil {
		return nil, err
	}
	return s.processRepo.GetStagesByBatchID(ctx, batchID)
}

func (s *ProcessService) GetCurrentStage(ctx context.Context, batchCode string) (*domain.BatchStage, error) {
	batchID, err := s.processRepo.GetBatchIDByCode(ctx, batchCode)
	if err != nil {
		return nil, err
	}
	return s.processRepo.GetCurrentStageByBatchID(ctx, batchID)
}

func (s *ProcessService) CreateEvent(ctx context.Context, batchCode string, eventType, severity, description string) (*domain.Event, error) {
	batchID, err := s.processRepo.GetBatchIDByCode(ctx, batchCode)
	if err != nil {
		return nil, err
	}
	var stageKey string
	if current, err := s.processRepo.GetCurrentStageByBatchID(ctx, batchID); err == nil {
		stageKey = current.StageKey
	}
	event := &domain.Event{
		BatchID:     batchID,
		StageKey:    stageKey,
		Type:        eventType,
		Severity:    severity,
		Description: description,
	}
	if err := s.eventRepo.CreateEvent(ctx, event); err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}
	return event, nil
}

func (s *ProcessService) GetEvents(ctx context.Context, batchCode string) ([]domain.Event, error) {
	batchID, err := s.processRepo.GetBatchIDByCode(ctx, batchCode)
	if err != nil {
		return nil, err
	}
	return s.eventRepo.GetEventsByBatchID(ctx, batchID)
}

func (s *ProcessService) ResolveEvent(ctx context.Context, eventID int, operatorID int, comment string) error {
	return s.eventRepo.ResolveEvent(ctx, eventID, operatorID, comment)
}

func (s *ProcessService) verifyPassword(ctx context.Context, userID int, password string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return domain.ErrNoUserFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return domain.ErrInvalidSignature
	}
	return nil
}
