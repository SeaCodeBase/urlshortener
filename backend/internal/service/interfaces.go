package service

import (
	"context"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/go-webauthn/webauthn/protocol"
)

//go:generate mockgen -destination=mocks/mock_auth_service.go -package=mocks . AuthService
type AuthService interface {
	Register(ctx context.Context, input RegisterInput) (*AuthResponse, error)
	Login(ctx context.Context, input LoginInput) (*AuthResponse, error)
	GetUserByID(ctx context.Context, userID uint64) (*model.User, error)
	ValidateToken(tokenString string) (uint64, error)
	ChangePassword(ctx context.Context, userID uint64, input ChangePasswordInput) error
}

//go:generate mockgen -destination=mocks/mock_link_service.go -package=mocks . LinkService
type LinkService interface {
	Create(ctx context.Context, userID uint64, input CreateLinkInput) (*model.Link, error)
	GetByID(ctx context.Context, userID, linkID uint64) (*model.Link, error)
	List(ctx context.Context, userID uint64, params ListLinksParams) (*ListLinksResult, error)
	Update(ctx context.Context, userID, linkID uint64, input UpdateLinkInput) (*model.Link, error)
	Delete(ctx context.Context, userID, linkID uint64) error
}

//go:generate mockgen -destination=mocks/mock_stats_service.go -package=mocks . StatsService
type StatsService interface {
	GetLinkStats(ctx context.Context, userID, linkID uint64) (*LinkStatsResponse, error)
}

//go:generate mockgen -destination=mocks/mock_shortcode_service.go -package=mocks . ShortCodeService
type ShortCodeService interface {
	Generate(ctx context.Context) (string, error)
	IsValid(code string) bool
	IsAvailable(ctx context.Context, code string) (bool, error)
}

//go:generate mockgen -destination=mocks/mock_passkey_service.go -package=mocks . PasskeyService
type PasskeyService interface {
	BeginRegistration(ctx context.Context, userID uint64) (*protocol.CredentialCreation, string, error)
	FinishRegistration(ctx context.Context, userID uint64, sessionData string, credentialJSON []byte, name string) (*model.Passkey, error)
	BeginLogin(ctx context.Context, userID uint64) (*protocol.CredentialAssertion, string, error)
	FinishLogin(ctx context.Context, userID uint64, sessionData string, credentialJSON []byte) error
	List(ctx context.Context, userID uint64) ([]model.Passkey, error)
	Rename(ctx context.Context, userID, passkeyID uint64, name string) error
	Delete(ctx context.Context, userID, passkeyID uint64) error
	HasPasskeys(ctx context.Context, userID uint64) (bool, error)
}
