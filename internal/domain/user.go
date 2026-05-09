package domain

import (
	"context"
	"errors"
)

type User struct {
	ID       int
	UserCode string
	UserName string //логин
	Password string
	Role     string
	FullName string
	IsActive bool
}

var (
	ErrInvalidRole      = errors.New("invalid role")
	ErrFullNameRequired = errors.New("full name is required")
	ErrUserExists       = errors.New("user-code or login is not unique")
	ErrNoUserFound      = errors.New("user not found")
	ErrWrongPassword    = errors.New("wrong password")
	ErrUserNotActive    = errors.New("user is not active")
)

type UserRole string

const (
	Admin    UserRole = "admin"
	Operator UserRole = "operator"
)

// отвечает только за доступ к данным, а не бизнес-операции
type UserRepo interface {
	//GetNextCode(ctx context.Context, role UserRole) (int, error)
	Create(ctx context.Context, user *User) error
	GetByUserName(ctx context.Context, userName string) (*User, error)
}
