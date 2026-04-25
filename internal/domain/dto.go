package domain

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
