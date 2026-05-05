package domain

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Batch struct {
	ID            int
	Code          string
	RecipeID      int
	TargetVolumeL int
	Status        string
	RegisteredBy  int
	CreatedAt     time.Time
}

var (
	ErrInvalidBatchVolume = errors.New("invalid batch volume")
)

type BatchRepo interface {
	Create(ctx context.Context, db *sql.Tx, batch *Batch) error
}
