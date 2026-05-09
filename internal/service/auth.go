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
	userRepo userRepo
}

func NewAuthService(r userRepo) *AuthService {
	return &AuthService{userRepo: r}
}

func generateToken(userID int, role domain.UserRole) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    string(role),
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func (as *AuthService) Login(ctx context.Context, username, password string) (*domain.User, string, error) {
	user, err := as.userRepo.GetByUserName(ctx, username)
	if err != nil {
		return nil, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, "", domain.ErrWrongPassword
	}

	if !user.IsActive {
		return nil, "", domain.ErrUserNotActive
	}

	token, err := generateToken(user.ID, user.Role)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}
