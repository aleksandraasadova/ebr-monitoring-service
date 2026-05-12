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
	return s.processRepo.CreateStage(ctx, stage)
}

// SignStageTransition verifies password, completes current stage, opens next.
func (s *ProcessService) SignStageTransition(ctx context.Context, batchCode string, operatorID int, password string) error {
	if err := s.verifyPassword(ctx, operatorID, password); err != nil {
		return err
	}

	batchID, err := s.processRepo.GetBatchIDByCode(ctx, batchCode)
	if err != nil {
		return err
	}

	current, err := s.processRepo.GetCurrentStageByBatchID(ctx, batchID)
	if err != nil {
		return err
	}

	if err := s.processRepo.SignAndCompleteStage(ctx, batchID, current.StageKey, operatorID); err != nil {
		return err
	}

	nextNumber := current.StageNumber + 1
	if nextNumber > len(domain.AllStages) {
		// all stages done
		return nil
	}

	next := domain.AllStages[nextNumber-1]
	nextStage := &domain.BatchStage{
		BatchID:     batchID,
		StageNumber: next.Number,
		StageKey:    next.Key,
		StageName:   next.Name,
	}
	return s.processRepo.CreateStage(ctx, nextStage)
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
