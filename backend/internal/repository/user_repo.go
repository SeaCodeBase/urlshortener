package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var ErrUserNotFound = errors.New("user not found")
var ErrEmailExists = errors.New("email already exists")

// Compile-time check: UserRepositoryImpl implements UserRepository
var _ UserRepository = (*UserRepositoryImpl)(nil)

type UserRepositoryImpl struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepositoryImpl {
	return &UserRepositoryImpl{db: db}
}

func (r *UserRepositoryImpl) Create(ctx context.Context, user *model.User) error {
	query := `INSERT INTO users (email, password_hash) VALUES (?, ?)`
	result, err := r.db.ExecContext(ctx, query, user.Email, user.PasswordHash)
	if err != nil {
		// Check for MySQL duplicate entry error (error code 1062)
		if strings.Contains(err.Error(), "Duplicate entry") {
			return ErrEmailExists
		}
		logger.Error(ctx, "user-repo: failed to create user",
			zap.String("email", user.Email),
			zap.Error(err),
		)
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		logger.Error(ctx, "user-repo: failed to get last insert ID",
			zap.Error(err),
		)
		return err
	}
	user.ID = uint64(id)
	return nil
}

func (r *UserRepositoryImpl) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	query := `SELECT id, email, display_name, password_hash, created_at, updated_at FROM users WHERE email = ?`
	err := r.db.GetContext(ctx, &user, query, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		logger.Error(ctx, "user-repo: failed to get user by email",
			zap.String("email", email),
			zap.Error(err),
		)
		return nil, err
	}
	return &user, nil
}

func (r *UserRepositoryImpl) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	query := `SELECT id, email, display_name, password_hash, created_at, updated_at FROM users WHERE id = ?`
	err := r.db.GetContext(ctx, &user, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		logger.Error(ctx, "user-repo: failed to get user by ID",
			zap.Uint64("user_id", id),
			zap.Error(err),
		)
		return nil, err
	}
	return &user, nil
}

func (r *UserRepositoryImpl) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE email = ?`
	err := r.db.GetContext(ctx, &count, query, email)
	if err != nil {
		logger.Error(ctx, "user-repo: failed to check email existence",
			zap.String("email", email),
			zap.Error(err),
		)
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepositoryImpl) UpdatePassword(ctx context.Context, userID uint64, passwordHash string) error {
	query := `UPDATE users SET password_hash = ?, updated_at = NOW() WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		logger.Error(ctx, "user-repo: failed to update password",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		logger.Error(ctx, "user-repo: failed to get rows affected",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *UserRepositoryImpl) UpdateDisplayName(ctx context.Context, userID uint64, displayName string) error {
	query := `UPDATE users SET display_name = ?, updated_at = NOW() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, displayName, userID)
	if err != nil {
		logger.Error(ctx, "user-repo: failed to update display name",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	return nil
}
