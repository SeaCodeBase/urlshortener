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
	query := `SELECT id, email, display_name, password_hash, default_group_id, account_name, home_checklist_dismissed, created_at, updated_at FROM users WHERE email = ?`
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
	query := `SELECT id, email, display_name, password_hash, default_group_id, account_name, home_checklist_dismissed, created_at, updated_at FROM users WHERE id = ?`
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

func (r *UserRepositoryImpl) UpdateDefaultGroup(ctx context.Context, userID, groupID uint64) error {
	query := `UPDATE users SET default_group_id = ?, updated_at = NOW() WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, groupID, userID)
	if err != nil {
		logger.Error(ctx, "user-repo: failed to update default group",
			zap.Uint64("user_id", userID),
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return err
	}
	return r.requireUpdated(ctx, result, userID)
}

func (r *UserRepositoryImpl) UpdateAccount(ctx context.Context, userID uint64, accountName *string, dismissed *bool) error {
	sets := []string{}
	args := []interface{}{}
	if accountName != nil {
		sets = append(sets, "account_name = ?")
		args = append(args, *accountName)
	}
	if dismissed != nil {
		sets = append(sets, "home_checklist_dismissed = ?")
		args = append(args, *dismissed)
	}
	if len(sets) == 0 {
		return r.requireExists(ctx, userID)
	}
	query := `UPDATE users SET ` + strings.Join(sets, ", ") + `, updated_at = NOW() WHERE id = ?`
	args = append(args, userID)
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		logger.Error(ctx, "user-repo: failed to update account",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	return r.requireUpdated(ctx, result, userID)
}

// requireUpdated maps "0 rows affected" to ErrUserNotFound. MariaDB reports
// 0 affected rows when an UPDATE matches a row but changes nothing (e.g. the
// same values within the same second), so existence is re-checked first.
func (r *UserRepositoryImpl) requireUpdated(ctx context.Context, result sql.Result, userID uint64) error {
	rows, err := result.RowsAffected()
	if err != nil {
		logger.Error(ctx, "user-repo: failed to get rows affected",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	if rows > 0 {
		return nil
	}
	return r.requireExists(ctx, userID)
}

func (r *UserRepositoryImpl) requireExists(ctx context.Context, userID uint64) error {
	var count int
	if err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM users WHERE id = ?`, userID); err != nil {
		logger.Error(ctx, "user-repo: failed to check user existence",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	if count == 0 {
		return ErrUserNotFound
	}
	return nil
}
