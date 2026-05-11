package service

import (
	"context"
	"fmt"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type BatchService struct {
	batchRepo  batchRepo
	recipeRepo recipeRepo
}

func NewBatchService(br batchRepo, rr recipeRepo) *BatchService {
	return &BatchService{
		batchRepo:  br,
		recipeRepo: rr,
	}
}

func (bs *BatchService) CreateBatch(ctx context.Context, recipeCode string, targetVolumeL int, registeredByID int) (*domain.Batch, error) {
	recipe, err := bs.recipeRepo.GetByCode(ctx, recipeCode)
	if err != nil {
		return nil, err
	}

	if targetVolumeL < recipe.MinVolumeL || targetVolumeL > recipe.MaxVolumeL {
		return nil, domain.ErrInvalidBatchVolume
	}

	batch := &domain.Batch{
		RecipeID:       recipe.ID,
		TargetVolumeL:  targetVolumeL,
		RegisteredByID: registeredByID,
	}

	if err := bs.batchRepo.Create(ctx, batch); err != nil {
		return nil, fmt.Errorf("create batch: %w", err)
	}

	return batch, nil
}

func (bs *BatchService) GetByStatus(ctx context.Context, status string) ([]domain.Batch, error) {
	batches, err := bs.batchRepo.GetByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("get batches by status: %w", err)
	}
	return batches, nil
}

func (bs *BatchService) GetWeighingLogByBatchCode(ctx context.Context, batchCode string) ([]domain.WeighingLogItem, error) {
	items, err := bs.batchRepo.GetWeighingLogByBatchCode(ctx, batchCode)
	if err != nil {
		return nil, fmt.Errorf("get weighing log by batch code: %w", err)
	}
	return items, nil
}

func (bs *BatchService) StartWeighing(ctx context.Context, batchCode string, operatorID int) error {
	if err := bs.batchRepo.StartWeighing(ctx, batchCode, operatorID); err != nil {
		return fmt.Errorf("start weighing: %w", err)
	}
	return nil
}

func (bs *BatchService) ConfirmWeighingItem(ctx context.Context, batchCode string, itemID int, actualQty float64, operatorID int) (string, error) {
	if actualQty < 0 {
		return "", domain.ErrInvalidTelemetryValue
	}

	status, err := bs.batchRepo.ConfirmWeighingItem(ctx, batchCode, itemID, actualQty, operatorID)
	if err != nil {
		return "", fmt.Errorf("confirm weighing item: %w", err)
	}
	return status, nil
}
