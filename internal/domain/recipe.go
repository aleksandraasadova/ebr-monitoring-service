package domain

import (
	"errors"
	"time"
)

type Recipe struct {
	ID                    int
	RecipeCode            string
	Name                  string
	Version               string
	MinVolumeL            int
	MaxVolumeL            int
	Description           string
	RequiredEquipmentType string
	CreatedBy             int
	CreatedAt             time.Time
	IsActive              bool
}

var (
	ErrRecipeNotFound = errors.New("recipe not found")
	ErrRecipeArchived = errors.New("recipe archived")
)
