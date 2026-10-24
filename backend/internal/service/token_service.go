package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidAPIToken  = errors.New("invalid api token")
	ErrAPITokenNotFound = errors.New("api token not found")
	ErrInvalidTokenName = errors.New("token name must not be empty")
)

const (
	// APITokenPrefix marks a bearer token as a personal access token rather
	// than a JWT.
	APITokenPrefix      = "us_"
	apiTokenBodyLen     = 40
	apiTokenDisplayLen  = 8
	apiTokenAlphabet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	maxTokenNameLen     = 255
	lastUsedWriteWindow = time.Minute
)

// Compile-time check: TokenServiceImpl implements TokenService
var _ TokenService = (*TokenServiceImpl)(nil)

type TokenServiceImpl struct {
	tokenRepo repository.APITokenRepository
	userRepo  repository.UserRepository
	now       func() time.Time
}

func NewTokenService(tokenRepo repository.APITokenRepository, userRepo repository.UserRepository) *TokenServiceImpl {
	return NewTokenServiceWithClock(tokenRepo, userRepo, time.Now)
}

// NewTokenServiceWithClock is NewTokenService with an injectable clock, used
// by tests of the last_used_at throttling.
func NewTokenServiceWithClock(tokenRepo repository.APITokenRepository, userRepo repository.UserRepository, now func() time.Time) *TokenServiceImpl {
	return &TokenServiceImpl{tokenRepo: tokenRepo, userRepo: userRepo, now: now}
}

func (s *TokenServiceImpl) Create(ctx context.Context, userID uint64, name, password string) (string, *model.APIToken, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil, ErrInvalidTokenName
	}
	name = truncateRunes(name, maxTokenNameLen)

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "token-service: failed to load user",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return "", nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		logger.Warn(ctx, "token-service: password mismatch on token creation",
			zap.Uint64("user_id", userID),
		)
		return "", nil, ErrInvalidCredentials
	}

	body, err := randomBase62(apiTokenBodyLen)
	if err != nil {
		logger.Error(ctx, "token-service: failed to generate token",
			zap.Error(err),
		)
		return "", nil, err
	}
	raw := APITokenPrefix + body
	token := &model.APIToken{
		UserID:      userID,
		Name:        name,
		TokenHash:   hashAPIToken(raw),
		TokenPrefix: body[:apiTokenDisplayLen],
	}
	if err := s.tokenRepo.Create(ctx, token); err != nil {
		logger.Error(ctx, "token-service: failed to store token",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return "", nil, err
	}
	return raw, token, nil
}

func (s *TokenServiceImpl) List(ctx context.Context, userID uint64) ([]model.APIToken, error) {
	tokens, err := s.tokenRepo.ListActiveByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "token-service: failed to list tokens",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	return tokens, nil
}

func (s *TokenServiceImpl) Revoke(ctx context.Context, userID, id uint64) error {
	err := s.tokenRepo.Revoke(ctx, userID, id)
	if errors.Is(err, repository.ErrAPITokenNotFound) {
		return ErrAPITokenNotFound
	}
	if err != nil {
		logger.Error(ctx, "token-service: failed to revoke token",
			zap.Uint64("user_id", userID),
			zap.Uint64("token_id", id),
			zap.Error(err),
		)
		return err
	}
	return nil
}

func (s *TokenServiceImpl) Validate(ctx context.Context, raw string) (uint64, error) {
	token, err := s.tokenRepo.GetByHash(ctx, hashAPIToken(raw))
	if errors.Is(err, repository.ErrAPITokenNotFound) {
		return 0, ErrInvalidAPIToken
	}
	if err != nil {
		logger.Error(ctx, "token-service: failed to look up token",
			zap.Error(err),
		)
		return 0, err
	}
	if token.RevokedAt.Valid {
		return 0, ErrInvalidAPIToken
	}

	// Throttle last_used_at writes to at most one per minute per token. A
	// failed write must not fail the request.
	now := s.now()
	if !token.LastUsedAt.Valid || now.Sub(token.LastUsedAt.Time) >= lastUsedWriteWindow {
		if err := s.tokenRepo.UpdateLastUsedAt(ctx, token.ID, now); err != nil {
			logger.Warn(ctx, "token-service: failed to update last_used_at",
				zap.Uint64("token_id", token.ID),
				zap.Error(err),
			)
		}
	}
	return token.UserID, nil
}

func hashAPIToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// randomBase62 returns n characters drawn uniformly from apiTokenAlphabet
// using crypto/rand.
func randomBase62(n int) (string, error) {
	max := big.NewInt(int64(len(apiTokenAlphabet)))
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = apiTokenAlphabet[idx.Int64()]
	}
	return string(b), nil
}
