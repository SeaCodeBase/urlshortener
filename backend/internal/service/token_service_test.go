package service_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"regexp"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

var fixedNow = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func newTokenService(t *testing.T) (*service.TokenServiceImpl, *mocks.MockAPITokenRepository, *mocks.MockUserRepository) {
	ctrl := gomock.NewController(t)
	tokenRepo := mocks.NewMockAPITokenRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewTokenServiceWithClock(tokenRepo, userRepo, func() time.Time { return fixedNow })
	return svc, tokenRepo, userRepo
}

func userWithPassword(t *testing.T, id uint64, password string) *model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return &model.User{ID: id, Email: "u@example.com", PasswordHash: string(hash)}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestTokenService_Create_WrongPassword(t *testing.T) {
	svc, _, userRepo := newTokenService(t)
	userRepo.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(userWithPassword(t, 1, "right-pass"), nil)

	raw, tok, err := svc.Create(context.Background(), 1, "cli", "wrong-pass")
	assert.ErrorIs(t, err, service.ErrInvalidCredentials)
	assert.Empty(t, raw)
	assert.Nil(t, tok)
}

func TestTokenService_Create_Success(t *testing.T) {
	svc, tokenRepo, userRepo := newTokenService(t)
	userRepo.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(userWithPassword(t, 1, "right-pass"), nil)

	var stored *model.APIToken
	tokenRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, tok *model.APIToken) error {
		stored = tok
		tok.ID = 42
		tok.CreatedAt = fixedNow
		return nil
	})

	raw, tok, err := svc.Create(context.Background(), 1, "  cli  ", "right-pass")
	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^us_[A-Za-z0-9]{40}$`), raw)
	require.NotNil(t, stored)
	assert.Same(t, stored, tok)
	assert.Equal(t, uint64(1), tok.UserID)
	assert.Equal(t, "cli", tok.Name)
	assert.Equal(t, sha256Hex(raw), tok.TokenHash)
	assert.Equal(t, raw[3:11], tok.TokenPrefix)
	assert.Equal(t, uint64(42), tok.ID)
}

func TestTokenService_Create_EmptyName(t *testing.T) {
	svc, _, _ := newTokenService(t)
	_, _, err := svc.Create(context.Background(), 1, "   ", "pw")
	assert.ErrorIs(t, err, service.ErrInvalidTokenName)
}

func TestTokenService_Create_UniqueTokens(t *testing.T) {
	svc, tokenRepo, userRepo := newTokenService(t)
	userRepo.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(userWithPassword(t, 1, "pw"), nil).Times(2)
	tokenRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	a, _, err := svc.Create(context.Background(), 1, "a", "pw")
	require.NoError(t, err)
	b, _, err := svc.Create(context.Background(), 1, "b", "pw")
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

func TestTokenService_Validate_Unknown401(t *testing.T) {
	svc, tokenRepo, _ := newTokenService(t)
	raw := "us_" + "A123456789012345678901234567890123456789"
	tokenRepo.EXPECT().GetByHash(gomock.Any(), sha256Hex(raw)).Return(nil, repository.ErrAPITokenNotFound)

	_, err := svc.Validate(context.Background(), raw)
	assert.ErrorIs(t, err, service.ErrInvalidAPIToken)
}

func TestTokenService_Validate_Revoked(t *testing.T) {
	svc, tokenRepo, _ := newTokenService(t)
	raw := "us_revoked"
	tokenRepo.EXPECT().GetByHash(gomock.Any(), sha256Hex(raw)).Return(&model.APIToken{
		ID: 3, UserID: 1,
		RevokedAt: model.NullTime{NullTime: sql.NullTime{Time: fixedNow.Add(-time.Hour), Valid: true}},
	}, nil)

	_, err := svc.Validate(context.Background(), raw)
	assert.ErrorIs(t, err, service.ErrInvalidAPIToken)
}

func TestTokenService_Validate_UpdatesLastUsedWhenNeverUsed(t *testing.T) {
	svc, tokenRepo, _ := newTokenService(t)
	raw := "us_fresh"
	tokenRepo.EXPECT().GetByHash(gomock.Any(), sha256Hex(raw)).Return(&model.APIToken{ID: 3, UserID: 9}, nil)
	tokenRepo.EXPECT().UpdateLastUsedAt(gomock.Any(), uint64(3), fixedNow).Return(nil)

	uid, err := svc.Validate(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, uint64(9), uid)
}

func TestTokenService_Validate_UpdatesLastUsedWhenStale(t *testing.T) {
	svc, tokenRepo, _ := newTokenService(t)
	raw := "us_stale"
	tokenRepo.EXPECT().GetByHash(gomock.Any(), sha256Hex(raw)).Return(&model.APIToken{
		ID: 3, UserID: 9,
		LastUsedAt: model.NullTime{NullTime: sql.NullTime{Time: fixedNow.Add(-61 * time.Second), Valid: true}},
	}, nil)
	tokenRepo.EXPECT().UpdateLastUsedAt(gomock.Any(), uint64(3), fixedNow).Return(nil)

	_, err := svc.Validate(context.Background(), raw)
	require.NoError(t, err)
}

func TestTokenService_Validate_SkipsLastUsedWithinMinute(t *testing.T) {
	svc, tokenRepo, _ := newTokenService(t)
	raw := "us_recent"
	tokenRepo.EXPECT().GetByHash(gomock.Any(), sha256Hex(raw)).Return(&model.APIToken{
		ID: 3, UserID: 9,
		LastUsedAt: model.NullTime{NullTime: sql.NullTime{Time: fixedNow.Add(-30 * time.Second), Valid: true}},
	}, nil)
	// No UpdateLastUsedAt expected.

	uid, err := svc.Validate(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, uint64(9), uid)
}

func TestTokenService_Revoke_NotFound(t *testing.T) {
	svc, tokenRepo, _ := newTokenService(t)
	tokenRepo.EXPECT().Revoke(gomock.Any(), uint64(1), uint64(5)).Return(repository.ErrAPITokenNotFound)

	err := svc.Revoke(context.Background(), 1, 5)
	assert.ErrorIs(t, err, service.ErrAPITokenNotFound)
}

func TestTokenService_List(t *testing.T) {
	svc, tokenRepo, _ := newTokenService(t)
	want := []model.APIToken{{ID: 1, UserID: 1, Name: "a"}}
	tokenRepo.EXPECT().ListActiveByUserID(gomock.Any(), uint64(1)).Return(want, nil)

	got, err := svc.List(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
