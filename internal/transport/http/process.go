package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/middleware"
)

type processService interface {
	StartProcess(ctx context.Context, batchCode string, operatorID int, password string) error
	SignStageTransition(ctx context.Context, batchCode string, operatorID int, password string) error
	GetAllStages(ctx context.Context, batchCode string) ([]domain.BatchStage, error)
	GetCurrentStage(ctx context.Context, batchCode string) (*domain.BatchStage, error)
	CreateEvent(ctx context.Context, batchCode string, eventType, severity, description string) (*domain.Event, error)
	GetEvents(ctx context.Context, batchCode string) ([]domain.Event, error)
	ResolveEvent(ctx context.Context, eventID int, operatorID int, comment string) error
}

type ProcessHandler struct {
	svc processService
}

func NewProcessHandler(svc processService) *ProcessHandler {
	return &ProcessHandler{svc: svc}
}

func (h *ProcessHandler) StartProcess(w http.ResponseWriter, r *http.Request) {
	batchCode := r.PathValue("code")
	var req StartProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "password required", http.StatusBadRequest)
		return
	}

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.svc.StartProcess(r.Context(), batchCode, user.UserID, req.Password); err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidSignature):
			http.Error(w, "invalid signature", http.StatusForbidden)
		case errors.Is(err, domain.ErrEquipmentOffline):
			http.Error(w, "equipment is offline or not ready", http.StatusConflict)
		case errors.Is(err, domain.ErrInvalidBatchStatus):
			http.Error(w, "batch is not in ready_for_process status", http.StatusConflict)
		default:
			http.Error(w, "failed to start process", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProcessHandler) SignStage(w http.ResponseWriter, r *http.Request) {
	batchCode := r.PathValue("code")
	var req SignStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "password required", http.StatusBadRequest)
		return
	}

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.svc.SignStageTransition(r.Context(), batchCode, user.UserID, req.Password); err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidSignature):
			http.Error(w, "invalid signature", http.StatusForbidden)
		case errors.Is(err, domain.ErrStageAlreadySigned):
			http.Error(w, "stage already signed", http.StatusConflict)
		case errors.Is(err, domain.ErrStageNotFound):
			http.Error(w, "no active stage", http.StatusNotFound)
		default:
			http.Error(w, "failed to sign stage", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProcessHandler) GetStages(w http.ResponseWriter, r *http.Request) {
	batchCode := r.PathValue("code")
	stages, err := h.svc.GetAllStages(r.Context(), batchCode)
	if err != nil {
		http.Error(w, "failed to get stages", http.StatusInternalServerError)
		return
	}

	resp := make([]BatchStageResponse, len(stages))
	for i, s := range stages {
		instruction := ""
		for _, def := range domain.AllStages {
			if def.Key == s.StageKey {
				instruction = def.Instruction
				break
			}
		}
		resp[i] = BatchStageResponse{
			ID:          s.ID,
			StageNumber: s.StageNumber,
			StageKey:    s.StageKey,
			StageName:   s.StageName,
			Instruction: instruction,
			StartedAt:   s.StartedAt,
			CompletedAt: s.CompletedAt,
			SignedBy:    s.SignedBy,
			SignedAt:    s.SignedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ProcessHandler) GetCurrentStage(w http.ResponseWriter, r *http.Request) {
	batchCode := r.PathValue("code")
	stage, err := h.svc.GetCurrentStage(r.Context(), batchCode)
	if err != nil {
		if errors.Is(err, domain.ErrStageNotFound) {
			http.Error(w, "no active stage", http.StatusNotFound)
		} else {
			http.Error(w, "failed to get stage", http.StatusInternalServerError)
		}
		return
	}

	instruction := ""
	for _, def := range domain.AllStages {
		if def.Key == stage.StageKey {
			instruction = def.Instruction
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BatchStageResponse{
		ID:          stage.ID,
		StageNumber: stage.StageNumber,
		StageKey:    stage.StageKey,
		StageName:   stage.StageName,
		Instruction: instruction,
		StartedAt:   stage.StartedAt,
		CompletedAt: stage.CompletedAt,
		SignedBy:    stage.SignedBy,
		SignedAt:    stage.SignedAt,
	})
}

func (h *ProcessHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	batchCode := r.PathValue("code")
	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	event, err := h.svc.CreateEvent(r.Context(), batchCode, req.Type, req.Severity, req.Description)
	if err != nil {
		http.Error(w, "failed to create event", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(EventResponse{
		ID:          event.ID,
		StageKey:    event.StageKey,
		Type:        event.Type,
		Severity:    event.Severity,
		Description: event.Description,
		OccurredAt:  event.OccurredAt,
	})
}

func (h *ProcessHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	batchCode := r.PathValue("code")
	events, err := h.svc.GetEvents(r.Context(), batchCode)
	if err != nil {
		http.Error(w, "failed to get events", http.StatusInternalServerError)
		return
	}

	resp := make([]EventResponse, len(events))
	for i, e := range events {
		resp[i] = EventResponse{
			ID:          e.ID,
			StageKey:    e.StageKey,
			Type:        e.Type,
			Severity:    e.Severity,
			Description: e.Description,
			Comment:     e.Comment,
			OccurredAt:  e.OccurredAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ProcessHandler) ResolveEvent(w http.ResponseWriter, r *http.Request) {
	eventIDStr := r.PathValue("eventID")
	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil || eventID <= 0 {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req ResolveEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := h.svc.ResolveEvent(r.Context(), eventID, user.UserID, req.Comment); err != nil {
		if errors.Is(err, domain.ErrEventNotFound) {
			http.Error(w, "event not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to resolve event", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
