package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/jose/urlshortener/internal/model"
)

var ErrUserNotFound = errors.New("user not found")
var ErrEmailExists = errors.New("email already exists")

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	query := `INSERT INTO users (email, password_hash) VALUES (?, ?)`
	result, err := r.db.ExecContext(ctx, query, user.Email, user.PasswordHash)
	if err != nil {
		// Check for MySQL duplicate entry error (error code 1062)
		if strings.Contains(err.Error(), "Duplicate entry") {
			return ErrEmailExists
		}
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = uint64(id)
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	query := `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = ?`
	err := r.db.GetContext(ctx, &user, query, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	query := `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = ?`
	err := r.db.GetContext(ctx, &user, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE email = ?`
	err := r.db.GetContext(ctx, &count, query, email)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uint64, passwordHash string) error {
	query := `UPDATE users SET password_hash = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}
