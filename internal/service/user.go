package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo userRepo
}

func NewUserService(r userRepo) *UserService {
	return &UserService{userRepo: r}
}

func (us *UserService) Create(ctx context.Context, role domain.UserRole, surname, name, fatherName string) (*domain.User, error) {
	if role != domain.Admin && role != domain.Operator {
		return nil, domain.ErrInvalidRole
	}

	surname = strings.ReplaceAll(surname, " ", "")
	name = strings.ReplaceAll(name, " ", "")
	fatherName = strings.ReplaceAll(fatherName, " ", "")

	if len(surname) < 2 || len(name) < 2 || len(fatherName) < 2 {
		return nil, domain.ErrFullNameRequired
	}

	fullName := surname + " " + name + " " + fatherName
	userName := toTransliteratedLower(surname) + "." + toTransliteratedLower(name[:2]) + "." + toTransliteratedLower(fatherName[:2])

	hash, err := bcrypt.GenerateFromPassword([]byte(userName), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		UserName: userName,
		Password: string(hash),
		Role:     role,
		FullName: fullName,
		IsActive: true,
	}

	if err := us.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}
