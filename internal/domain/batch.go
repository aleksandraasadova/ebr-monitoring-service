package domain

import (
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
