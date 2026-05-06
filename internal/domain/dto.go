package domain

import "time"

// DTO для логина и пароля
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DTO для ответа на авторизацию
type LoginResponse struct {
	Role     string `json:"role"`
	Token    string `json:"token"`
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
	// UserCode - генерирует система
	// UserName - генерирует система
	// IsActive - ставит система
}

type CreateUserResponse struct {
	UserCode string `json:"user_code"`
	UserName string `json:"user_name"`
}

type GetRecipeByCodeRequest struct {
	Code string // приходит из path param
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
	// registered_by берется из токена
}

type CreateBatchResponse struct {
	BatchCode    string    `json:"batch_code"`
	BatchStatus  string    `json:"batch_status"`
	CreatedAt    time.Time `json:"created_at"`
	RegisteredBy int       `json:"registered_by"`
}
