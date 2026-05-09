package service

import (
	"context"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type userRepo interface {
	Create(ctx context.Context, user *domain.User) error
	GetByUserName(ctx context.Context, userName string) (*domain.User, error)
}

type batchRepo interface {
	Create(ctx context.Context, batch *domain.Batch) error
	GetByStatus(ctx context.Context, status string) ([]domain.Batch, error)
}

type recipeRepo interface {
	GetByCode(ctx context.Context, code string) (*domain.Recipe, error)
}
