package domain

import (
	"context"
	"errors"
	"time"
)

type Batch struct {
	ID               int
	Code             string
	RecipeID         int
	RecipeCode       string
	TargetVolumeL    int
	Status           string
	RegisteredByID   int
	RegisteredByCode string
	CreatedAt        time.Time
}

var (
	ErrInvalidBatchVolume = errors.New("invalid batch volume")
)

type BatchRepo interface {
	Create(ctx context.Context, batch *Batch, recipeID int) error
	GetByStatus(ctx context.Context, status string) ([]Batch, error)
}
