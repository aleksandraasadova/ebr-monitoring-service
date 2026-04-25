package service

import (
	"context"
	"os"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo domain.UserRepository
}

func NewAuthService(userRepo domain.UserRepository) *AuthService {
	return &AuthService{userRepo: userRepo}
}

func generateToken(userID int, role string) (string, error) {
	claims := jwt.MapClaims{ // payload
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func (as *AuthService) Login(ctx context.Context, req domain.LoginRequest) (*domain.LoginResponse, error) {

	user, err := as.userRepo.GetByUserName(ctx, req.Username)
	if err != nil {
		return nil, err // Ошибка БД
	}

	if user == nil {
		return nil, domain.ErrNoUserFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, domain.ErrWrongPassword
	}

	if !user.IsActive {
		return nil, domain.ErrUserNotActive
	}

	token, err := generateToken(user.ID, user.Role)
	if err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		Role:     user.Role,
		Token:    token,
		UserCode: user.UserCode,
		UserName: user.UserName,
		FullName: user.FullName,
		IsActive: user.IsActive,
	}, nil
}
