package service

import (
	"context"
	"fmt"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type BatchService struct {
	batchRepo  domain.BatchRepo
	recipeRepo domain.RecipeRepo
}

func NewBatchService(batchRepo domain.BatchRepo, recipeRepo domain.RecipeRepo) *BatchService {
	return &BatchService{
		batchRepo:  batchRepo,
		recipeRepo: recipeRepo,
	}
}

func (bs *BatchService) CreateBatch(ctx context.Context, req domain.CreateBatchRequest, registeredByID int) (*domain.CreateBatchResponse, error) {
	recipe, err := bs.recipeRepo.GetByCode(ctx, req.RecipeCode)
	if err != nil {
		return nil, err
	}

	if req.TargetVolumeL < recipe.MinVolumeL || req.TargetVolumeL > recipe.MaxVolumeL {
		return nil, domain.ErrInvalidBatchVolume
	}

	batch := &domain.Batch{
		RecipeID:       recipe.ID,
		TargetVolumeL:  req.TargetVolumeL,
		RegisteredByID: registeredByID,
	}

	if err := bs.batchRepo.Create(ctx, batch, recipe.ID); err != nil {
		return nil, fmt.Errorf("create batch: %w", err)
	}

	return &domain.CreateBatchResponse{
		BatchCode:    batch.Code,
		BatchStatus:  batch.Status,
		CreatedAt:    batch.CreatedAt,
		RegisteredBy: batch.RegisteredByID,
	}, nil
}

func (bs *BatchService) GetByStatus(ctx context.Context, status string) ([]domain.Batch, error) {
	batches, err := bs.batchRepo.GetByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("get batches by status: %w", err)
	}
	return batches, nil
}
