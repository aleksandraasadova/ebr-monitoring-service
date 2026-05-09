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
