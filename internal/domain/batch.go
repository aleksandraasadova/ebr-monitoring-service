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

type WeighingLogItem struct {
	ID            int
	BatchCode     string
	BatchStatus   string
	IngredientID  int
	Ingredient    string
	StageKey      string
	RequiredQty   float64
	ActualQty     *float64
	ContainerCode string
	WeighedByCode string
	WeighedAt     *time.Time
}

var (
	ErrInvalidBatchVolume = errors.New("invalid batch volume")
	ErrBatchNotFound      = errors.New("batch not found")
	ErrInvalidBatchStatus = errors.New("invalid batch status")
	ErrWeighingNotFound   = errors.New("weighing item not found")
)
