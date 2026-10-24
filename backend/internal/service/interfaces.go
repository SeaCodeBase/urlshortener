package service

import (
	"context"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/go-webauthn/webauthn/protocol"
)

//go:generate mockgen -destination=mocks/mock_auth_service.go -package=mocks . AuthService
type AuthService interface {
	Register(ctx context.Context, input RegisterInput) (*AuthResponse, error)
	Login(ctx context.Context, input LoginInput) (*AuthResponse, error)
	GetUserByID(ctx context.Context, userID uint64) (*model.User, error)
	ValidateToken(tokenString string) (uint64, error)
	ChangePassword(ctx context.Context, userID uint64, input ChangePasswordInput) error
	UpdateDisplayName(ctx context.Context, userID uint64, displayName string) error
	GenerateToken(userID uint64) (string, error)
}

//go:generate mockgen -destination=mocks/mock_group_service.go -package=mocks . GroupService
type GroupService interface {
	Create(ctx context.Context, userID uint64, name string) (*model.Group, error)
	List(ctx context.Context, userID uint64) ([]model.Group, error)
	Get(ctx context.Context, userID, groupID uint64) (*model.Group, error)
	Update(ctx context.Context, userID, groupID uint64, input UpdateGroupInput) (*model.Group, error)
	// ResolveActive returns the group a request acts on: the group named by
	// the X-Group-ID header when given (must be owned by userID), otherwise
	// the user's default group, created lazily when missing.
	ResolveActive(ctx context.Context, userID uint64, headerGroupID *uint64) (*model.Group, error)
}

//go:generate mockgen -destination=mocks/mock_token_service.go -package=mocks . TokenService
type TokenService interface {
	// Create issues a new API token after re-checking the user's password.
	// The raw token is returned only here; only its hash is stored.
	Create(ctx context.Context, userID uint64, name, password string) (string, *model.APIToken, error)
	List(ctx context.Context, userID uint64) ([]model.APIToken, error)
	Revoke(ctx context.Context, userID, id uint64) error
	// Validate resolves a raw "us_" token to its user ID. Unknown or revoked
	// tokens return ErrInvalidAPIToken.
	Validate(ctx context.Context, raw string) (uint64, error)
}

//go:generate mockgen -destination=mocks/mock_link_service.go -package=mocks . LinkService
type LinkService interface {
	// Create creates a link in input.GroupID; when input.Title is empty the
	// destination's title is fetched.
	Create(ctx context.Context, userID uint64, input CreateLinkInput) (*model.Link, error)
	GetByID(ctx context.Context, userID, linkID uint64) (*model.Link, error)
	// GetDetails returns the link with tags, clicks, routing rules and its
	// back-half history.
	GetDetails(ctx context.Context, userID, linkID uint64) (*LinkDetails, error)
	// List pages through the group's links matching params.Filter.
	List(ctx context.Context, groupID uint64, params ListLinksParams) (*ListLinksResult, error)
	// Update edits the link in place; a destination change increments
	// redirect_count. It also returns the link as it was before the update.
	Update(ctx context.Context, userID, linkID uint64, input UpdateLinkInput) (*model.Link, *model.Link, error)
	// SetTags replaces the link's tags and returns the normalized set.
	SetTags(ctx context.Context, userID, linkID uint64, tags []string) ([]string, error)
	// SetArchived hides/shows links; every id must belong to groupID or
	// ErrLinkNotFound is returned and nothing changes.
	SetArchived(ctx context.Context, userID, groupID uint64, ids []uint64, archived bool) ([]model.Link, error)
	// BulkTags adds/removes tags; every id must belong to groupID or
	// ErrLinkNotFound is returned and nothing changes.
	BulkTags(ctx context.Context, groupID uint64, ids []uint64, add, remove []string) error
	// EditBackHalf creates a new link with the back-half, copying the old
	// one; domainID nil keeps the old domain, 0 means the default domain.
	EditBackHalf(ctx context.Context, userID, linkID uint64, domainID *uint64, backHalf string) (*model.Link, error)
	// Delete removes the link and returns it; ErrLinkNotDeletable when it was
	// redirected or has a custom back-half.
	Delete(ctx context.Context, userID, linkID uint64) (*model.Link, error)
	// Export returns CSV rows (see LinkExportHeader) for links matching f.
	Export(ctx context.Context, groupID uint64, f repository.LinkFilter) ([][]string, error)
}

//go:generate mockgen -destination=mocks/mock_stats_service.go -package=mocks . StatsService
type StatsService interface {
	// GetLinkStats returns the link's analytics for the query window; it
	// returns a *StatsQueryError for invalid queries and ErrNotLinkOwner when
	// the user does not own the link.
	GetLinkStats(ctx context.Context, userID, linkID uint64, q StatsQuery) (*LinkStatsV2, error)
	// GroupEngagements counts all clicks on the group's links in the
	// inclusive UTC date range.
	GroupEngagements(ctx context.Context, groupID uint64, from, to time.Time) (int64, error)
}

//go:generate mockgen -destination=mocks/mock_shortcode_service.go -package=mocks . ShortCodeService
type ShortCodeService interface {
	// Generate creates a unique short code within the given domain.
	// domainID nil means the default domain.
	Generate(ctx context.Context, domainID *uint64) (string, error)
	IsValid(code string) bool
	// IsAvailable checks if a short code is available within the given domain.
	// domainID nil means the default domain.
	IsAvailable(ctx context.Context, domainID *uint64, code string) (bool, error)
}

//go:generate mockgen -destination=mocks/mock_tag_service.go -package=mocks . TagService
type TagService interface {
	// ListByGroup returns the group's distinct tags for an entity type:
	// "link" (or "") for links, "qr" for QR codes; other types return
	// ErrInvalidTagType.
	ListByGroup(ctx context.Context, groupID uint64, typ string) ([]string, error)
}

//go:generate mockgen -destination=mocks/mock_activity_service.go -package=mocks . ActivityService
type ActivityService interface {
	// Log records an activity entry. It never fails: errors are logged and
	// swallowed so callers' primary actions are unaffected. meta may be nil.
	Log(ctx context.Context, userID, groupID uint64, action, entityType string, entityID uint64, meta map[string]any)
	// List pages through the group's activity, newest first. from is
	// inclusive, to exclusive (nil = unbounded). page < 1 becomes 1; limit
	// defaults to 20 and is capped at 100.
	List(ctx context.Context, groupID uint64, from, to *time.Time, page, limit int) (*ActivityListResult, error)
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
