package service

import (
	"context"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/repository"
)

type userRepo interface {
	Create(ctx context.Context, user *domain.User) error
	GetByUserName(ctx context.Context, userName string) (*domain.User, error)
	GetByID(ctx context.Context, id int) (*domain.User, error)
}

type batchRepo interface {
	Create(ctx context.Context, batch *domain.Batch) error
	GetByStatus(ctx context.Context, status string) ([]domain.Batch, error)
	GetWeighingLogByBatchCode(ctx context.Context, batchCode string) ([]domain.WeighingLogItem, error)
	StartWeighing(ctx context.Context, batchCode string, operatorID int) error
	ConfirmWeighingItem(ctx context.Context, batchCode string, itemID int, actualQty float64, operatorID int) (string, error)
}

type recipeRepo interface {
	GetByCode(ctx context.Context, code string) (*domain.Recipe, error)
	GetAll(ctx context.Context) ([]domain.Recipe, error)
	Create(ctx context.Context, recipe *domain.Recipe, ingredients []repository.RecipeIngredientInput) error
	Archive(ctx context.Context, code string) error
	GetIngredients(ctx context.Context) ([]repository.Ingredient, error)
}

type processRepo interface {
	CreateStage(ctx context.Context, stage *domain.BatchStage) error
	GetStagesByBatchID(ctx context.Context, batchID int) ([]domain.BatchStage, error)
	GetCurrentStageByBatchID(ctx context.Context, batchID int) (*domain.BatchStage, error)
	SignAndCompleteStage(ctx context.Context, batchID int, stageKey string, userID int, comment string) error
	GetBatchIDByCode(ctx context.Context, batchCode string) (int, error)
	StartProcess(ctx context.Context, batchCode string) error
	CompleteBatch(ctx context.Context, batchCode string) error
	CancelBatch(ctx context.Context, batchCode, reason string) error
	CheckProcessOperator(ctx context.Context, batchCode string, operatorID int) error
	BatchBelongsToUser(ctx context.Context, batchCode string, userID int) bool
}

type eventRepo interface {
	CreateEvent(ctx context.Context, event *domain.Event) error
	GetEventsByBatchID(ctx context.Context, batchID int) ([]domain.Event, error)
	ResolveEvent(ctx context.Context, eventID int, userID int, comment string) error
}
