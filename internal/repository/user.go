package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create сохраняет пользователя и возвращает сгенерированный user_code
func (ur *UserRepo) Create(ctx context.Context, user *domain.User) error {
	// QueryRowContext выполняет запрос и ждёт ОДНУ строку результата
	// Возвращает *sql.Row, из которой можно забрать значения через Scan()
	err := ur.db.QueryRowContext(ctx, `
		INSERT INTO users (
			username,
			password_hash,
			role,
			full_name,
			is_active
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING user_code
	`,
		user.UserName, // $1
		user.Password, // $2
		user.Role,     // $3
		user.FullName, // $4
		user.IsActive, // $5
	).Scan(&user.UserCode)
	fmt.Printf("user code: %s", user.UserCode)
	if err != nil {
		// Обработка ошибки уникальности (дубль user_name или user_code)
		// PostgreSQL возвращает ошибку с кодом "23505" или текстом "duplicate key"
		if strings.Contains(err.Error(), "duplicate key") ||
			errors.Is(err, sql.ErrNoRows) {
			return domain.ErrUserExists
		}
		return err
	}

	return nil
}

func (ur *UserRepo) GetByUserName(ctx context.Context, userName string) (*domain.User, error) {
	var user domain.User

	err := ur.db.QueryRowContext(ctx, `
		SELECT id, user_code, username, password_hash, role, full_name, is_active
		FROM users
		WHERE username = $1
	`, userName).Scan(
		&user.ID,
		&user.UserCode,
		&user.UserName,
		&user.Password,
		&user.Role,
		&user.FullName,
		&user.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNoUserFound
		}
		return nil, err
	}

	return &user, nil
}
