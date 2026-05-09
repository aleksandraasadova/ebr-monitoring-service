package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo domain.UserRepo
}

func NewUserService(userRepo domain.UserRepo) *UserService {
	return &UserService{userRepo: userRepo}
}

func (us *UserService) Create(ctx context.Context, req domain.CreateUserRequest) (*domain.CreateUserResponse, error) {

	if req.Role != string(domain.Admin) && req.Role != string(domain.Operator) {
		return nil, domain.ErrInvalidRole
	}

	if req.Surname == "" || req.Name == "" || req.FatherName == "" {
		return nil, domain.ErrFullNameRequired
	}

	surname := strings.ReplaceAll(req.Surname, " ", "")
	name := strings.ReplaceAll(req.Name, " ", "")
	fatherName := strings.ReplaceAll(req.FatherName, " ", "")

	if len(surname) < 2 || len(name) < 2 || len(fatherName) < 2 {
		return nil, domain.ErrFullNameRequired
	}

	fullName := surname + " " + name + " " + fatherName

	userName := toTransliteratedLower(surname) + "." + toTransliteratedLower(name[:2]) + "." + toTransliteratedLower(fatherName[:2])

	password := userName // TODO: потом сделать возможность для юзера поменять свой пароль или генерить его

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to create hash from password: %w", err)
	}

	newUser := &domain.User{
		UserName: userName,
		Password: string(hash),
		Role:     req.Role,
		FullName: fullName,
		IsActive: true,
	}

	if err := us.userRepo.Create(ctx, newUser); err != nil {
		return nil, err
	}

	return &domain.CreateUserResponse{
		UserCode: newUser.UserCode,
		UserName: newUser.UserName,
	}, nil
}
