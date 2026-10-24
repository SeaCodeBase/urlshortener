package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var ErrAPITokenNotFound = errors.New("api token not found")

// Compile-time check: APITokenRepositoryImpl implements APITokenRepository
var _ APITokenRepository = (*APITokenRepositoryImpl)(nil)

const apiTokenColumns = "id, user_id, name, token_hash, token_prefix, last_used_at, created_at, revoked_at"

type APITokenRepositoryImpl struct {
	db *sqlx.DB
}

func NewAPITokenRepository(db *sqlx.DB) *APITokenRepositoryImpl {
	return &APITokenRepositoryImpl{db: db}
}

func (r *APITokenRepositoryImpl) Create(ctx context.Context, token *model.APIToken) error {
	query := "INSERT INTO api_tokens (user_id, name, token_hash, token_prefix) VALUES (?, ?, ?, ?)"
	result, err := r.db.ExecContext(ctx, query, token.UserID, token.Name, token.TokenHash, token.TokenPrefix)
	if err != nil {
		logger.Error(ctx, "api-token-repo: failed to create token",
			zap.Uint64("user_id", token.UserID),
			zap.Error(err),
		)
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		logger.Error(ctx, "api-token-repo: failed to get last insert ID",
			zap.Error(err),
		)
		return err
	}
	token.ID = uint64(id)

	// Fetch the created record to get the DB-generated created_at.
	var created model.APIToken
	if err := r.db.GetContext(ctx, &created, "SELECT "+apiTokenColumns+" FROM api_tokens WHERE id = ?", token.ID); err != nil {
		logger.Error(ctx, "api-token-repo: failed to reload created token",
			zap.Uint64("id", token.ID),
			zap.Error(err),
		)
		return err
	}
	token.CreatedAt = created.CreatedAt
	return nil
}

func (r *APITokenRepositoryImpl) GetByHash(ctx context.Context, tokenHash string) (*model.APIToken, error) {
	var token model.APIToken
	query := "SELECT " + apiTokenColumns + " FROM api_tokens WHERE token_hash = ?"
	if err := r.db.GetContext(ctx, &token, query, tokenHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAPITokenNotFound
		}
		logger.Error(ctx, "api-token-repo: failed to get token by hash",
			zap.Error(err),
		)
		return nil, err
	}
	return &token, nil
}

func (r *APITokenRepositoryImpl) ListActiveByUserID(ctx context.Context, userID uint64) ([]model.APIToken, error) {
	tokens := []model.APIToken{}
	query := "SELECT " + apiTokenColumns + " FROM api_tokens WHERE user_id = ? AND revoked_at IS NULL ORDER BY created_at DESC, id DESC"
	if err := r.db.SelectContext(ctx, &tokens, query, userID); err != nil {
		logger.Error(ctx, "api-token-repo: failed to list tokens",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	return tokens, nil
}

func (r *APITokenRepositoryImpl) Revoke(ctx context.Context, userID, id uint64) error {
	query := "UPDATE api_tokens SET revoked_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ? AND revoked_at IS NULL"
	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		logger.Error(ctx, "api-token-repo: failed to revoke token",
			zap.Uint64("id", id),
			zap.Error(err),
		)
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAPITokenNotFound
	}
	return nil
}

func (r *APITokenRepositoryImpl) UpdateLastUsedAt(ctx context.Context, id uint64, at time.Time) error {
	if _, err := r.db.ExecContext(ctx, "UPDATE api_tokens SET last_used_at = ? WHERE id = ?", at.UTC(), id); err != nil {
		logger.Error(ctx, "api-token-repo: failed to update last_used_at",
			zap.Uint64("id", id),
			zap.Error(err),
		)
		return err
	}
	return nil
}
