package transport

import "time"

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Role     string `json:"role"`
	UserCode string `json:"user_code"`
	UserName string `json:"user_name"`
	FullName string `json:"full_name"`
	IsActive bool   `json:"is_active"`
}

type CreateUserRequest struct {
	Role       string `json:"role"`
	Surname    string `json:"surname"`
	Name       string `json:"name"`
	FatherName string `json:"father_name"`
}

type CreateUserResponse struct {
	UserCode string `json:"user_code"`
	UserName string `json:"user_name"`
}

type GetRecipeByCodeResponse struct {
	Name                  string `json:"name"`
	Version               string `json:"version"`
	MinVolumeL            int    `json:"min_volume_l"`
	MaxVolumeL            int    `json:"max_volume_l"`
	Description           string `json:"description"`
	RequiredEquipmentType string `json:"required_equipment_type"`
}

type CreateBatchRequest struct {
	RecipeCode    string `json:"recipe_code"`
	TargetVolumeL int    `json:"target_volume_l"`
}

type CreateBatchResponse struct {
	BatchCode    string    `json:"batch_code"`
	BatchStatus  string    `json:"batch_status"`
	CreatedAt    time.Time `json:"created_at"`
	RegisteredBy int       `json:"registered_by"`
}

type GetBatchesByStatusResponse struct {
	ID            int       `json:"batch_id"`
	BatchCode     string    `json:"batch_code"`
	RecipeCode    string    `json:"recipe_code"`
	TargetVolumeL int       `json:"target_volume_l"`
	BatchStatus   string    `json:"batch_status"`
	RegisteredBy  string    `json:"registered_by"`
	CreatedAt     time.Time `json:"created_at"`
}

type WeighingLogItemResponse struct {
	ID            int        `json:"id"`
	BatchCode     string     `json:"batch_code"`
	BatchStatus   string     `json:"batch_status"`
	IngredientID  int        `json:"ingredient_id"`
	Ingredient    string     `json:"ingredient"`
	StageKey      string     `json:"stage_key"`
	RequiredQty   float64    `json:"required_qty"`
	ActualQty     *float64   `json:"actual_qty"`
	ContainerCode string     `json:"container_code"`
	WeighedBy     string     `json:"weighed_by"`
	WeighedAt     *time.Time `json:"weighed_at"`
}

type ConfirmWeighingItemRequest struct {
	ActualQty         float64 `json:"actual_qty"`
	SignaturePassword string  `json:"signature_password"`
}

type ConfirmWeighingItemResponse struct {
	BatchStatus string `json:"batch_status"`
}

// Process

type StartProcessRequest struct {
	Password string `json:"password"`
}

type SignStageRequest struct {
	Password string `json:"password"`
}

type BatchStageResponse struct {
	ID          int        `json:"id"`
	StageNumber int        `json:"stage_number"`
	StageKey    string     `json:"stage_key"`
	StageName   string     `json:"stage_name"`
	Instruction string     `json:"instruction"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	SignedBy    *int       `json:"signed_by"`
	SignedAt    *time.Time `json:"signed_at"`
}

// Events

type CreateEventRequest struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

type ResolveEventRequest struct {
	Comment string `json:"comment"`
}

type EventResponse struct {
	ID          int       `json:"id"`
	StageKey    string    `json:"stage_key"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Comment     string    `json:"comment"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// Reports

type ReportMetaResponse struct {
	ID          int       `json:"id"`
	BatchCode   string    `json:"batch_code"`
	BatchStatus string    `json:"batch_status"`
	GeneratedAt time.Time `json:"generated_at"`
}
