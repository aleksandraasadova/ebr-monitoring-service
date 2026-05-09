package domain

import "errors"

type User struct {
	ID       int
	UserCode string
	UserName string
	Password string
	Role     UserRole
	FullName string
	IsActive bool
}

type UserRole string

const (
	Admin    UserRole = "admin"
	Operator UserRole = "operator"
)

var (
	ErrInvalidRole      = errors.New("invalid role")
	ErrFullNameRequired = errors.New("full name is required")
	ErrUserExists       = errors.New("user-code or login is not unique")
	ErrNoUserFound      = errors.New("user not found")
	ErrWrongPassword    = errors.New("wrong password")
	ErrUserNotActive    = errors.New("user is not active")
)
