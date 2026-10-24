package repository

import (
	"context"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
)

//go:generate mockgen -destination=mocks/mock_user_repo.go -package=mocks . UserRepository
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id uint64) (*model.User, error)
	EmailExists(ctx context.Context, email string) (bool, error)
	UpdatePassword(ctx context.Context, userID uint64, passwordHash string) error
	UpdateDisplayName(ctx context.Context, userID uint64, displayName string) error
	UpdateDefaultGroup(ctx context.Context, userID, groupID uint64) error
	// SetDefaultGroupIfUnset sets default_group_id only if it still equals
	// expectedCurrent (NULL-safe). It reports whether the row was updated;
	// false means another writer changed it first (or the user is missing).
	SetDefaultGroupIfUnset(ctx context.Context, userID, groupID uint64, expectedCurrent *uint64) (bool, error)
	// UpdateAccount sets account_name and/or home_checklist_dismissed; a nil
	// argument leaves that column unchanged.
	UpdateAccount(ctx context.Context, userID uint64, accountName *string, dismissed *bool) error
	// UpdateEmail changes the user's email; ErrEmailExists when another user
	// already has it, ErrUserNotFound when the user is missing.
	UpdateEmail(ctx context.Context, userID uint64, email string) error
}

//go:generate mockgen -destination=mocks/mock_group_repo.go -package=mocks . GroupRepository
type GroupRepository interface {
	Create(ctx context.Context, group *model.Group) error
	GetByID(ctx context.Context, id uint64) (*model.Group, error)
	ListByUserID(ctx context.Context, userID uint64) ([]model.Group, error)
	// Update writes name and default_domain_id (nil clears it).
	Update(ctx context.Context, group *model.Group) error
	CountByUserID(ctx context.Context, userID uint64) (int, error)
	Delete(ctx context.Context, id uint64) error
}

//go:generate mockgen -destination=mocks/mock_api_token_repo.go -package=mocks . APITokenRepository
type APITokenRepository interface {
	Create(ctx context.Context, token *model.APIToken) error
	// GetByHash returns the token (revoked or not) with the given SHA-256 hex
	// hash, or ErrAPITokenNotFound.
	GetByHash(ctx context.Context, tokenHash string) (*model.APIToken, error)
	// ListActiveByUserID returns the user's non-revoked tokens, newest first.
	ListActiveByUserID(ctx context.Context, userID uint64) ([]model.APIToken, error)
	// Revoke marks a non-revoked token owned by userID as revoked; it returns
	// ErrAPITokenNotFound when no such token exists.
	Revoke(ctx context.Context, userID, id uint64) error
	UpdateLastUsedAt(ctx context.Context, id uint64, at time.Time) error
}

//go:generate mockgen -destination=mocks/mock_link_repo.go -package=mocks . LinkRepository
type LinkRepository interface {
	Create(ctx context.Context, link *model.Link) error
	GetByID(ctx context.Context, id uint64) (*model.Link, error)
	// GetByDomainAndShortCode finds a link by domain_id and short_code combination.
	// domainID nil means the default domain (domain_id IS NULL)
	GetByDomainAndShortCode(ctx context.Context, domainID *uint64, shortCode string) (*model.Link, error)
	ListByUserID(ctx context.Context, userID uint64, limit, offset int) ([]model.Link, error)
	CountByUserID(ctx context.Context, userID uint64) (int64, error)
	Update(ctx context.Context, link *model.Link) error
	Delete(ctx context.Context, id uint64) error
	// ShortCodeExistsInDomain checks if a short code exists within a specific domain.
	// domainID nil means the default domain (domain_id IS NULL)
	ShortCodeExistsInDomain(ctx context.Context, domainID *uint64, shortCode string) (bool, error)
	// GetByIDs returns the existing links among ids; missing ids are absent.
	GetByIDs(ctx context.Context, ids []uint64) ([]model.Link, error)
	// ListByGroup pages through the group's links matching f with their
	// total clicks (Tags left nil), plus the total match count. Links with
	// source <> 'link' are excluded unless f.IncludeAllSources.
	ListByGroup(ctx context.Context, groupID uint64, f LinkFilter, limit, offset int) ([]model.LinkWithStats, int64, error)
	// CountByGroup counts the group's links matching f (source='link' only
	// unless f.IncludeAllSources); used by global search.
	CountByGroup(ctx context.Context, groupID uint64, f LinkFilter) (int64, error)
	// SetArchived hides (true) or shows (false) the links; empty ids is a no-op.
	SetArchived(ctx context.Context, ids []uint64, archived bool) error
	// IncrementRedirectCount records a destination change; ErrLinkNotFound
	// when the link does not exist.
	IncrementRedirectCount(ctx context.Context, id uint64) error
	// CountByDomainID counts the links (of any group or source) on a custom domain.
	CountByDomainID(ctx context.Context, domainID uint64) (int64, error)
}

//go:generate mockgen -destination=mocks/mock_passkey_repo.go -package=mocks . PasskeyRepository
type PasskeyRepository interface {
	Create(ctx context.Context, passkey *model.Passkey) error
	GetByCredentialID(ctx context.Context, credentialID []byte) (*model.Passkey, error)
	ListByUserID(ctx context.Context, userID uint64) ([]model.Passkey, error)
	UpdateCounter(ctx context.Context, id uint64, counter uint32) error
	UpdateLastUsedAt(ctx context.Context, id uint64) error
	UpdateName(ctx context.Context, id uint64, name string) error
	Delete(ctx context.Context, id uint64) error
	CountByUserID(ctx context.Context, userID uint64) (int, error)
}

//go:generate mockgen -destination=mocks/mock_domain_repo.go -package=mocks . DomainRepository
type DomainRepository interface {
	// Create inserts user_id, group_id, domain, verification_token and
	// verification_status (empty means pending) and sets domain.ID.
	Create(ctx context.Context, domain *model.Domain) error
	GetByID(ctx context.Context, id uint64) (*model.Domain, error)
	GetByDomain(ctx context.Context, domain string) (*model.Domain, error)
	ListByUserID(ctx context.Context, userID uint64) ([]*model.Domain, error)
	// Update writes group_id, verification_status, verification_error,
	// verified_at, last_checked_at, root_redirect and not_found_redirect (nil
	// clears); ErrDomainNotFound when the domain is missing.
	Update(ctx context.Context, domain *model.Domain) error
	// UpdateVerification writes only verification_status, verification_error,
	// verified_at and last_checked_at, so a concurrent settings edit is kept;
	// ErrDomainNotFound when the domain is missing.
	UpdateVerification(ctx context.Context, id uint64, status string, errMsg *string,
		verifiedAt, lastCheckedAt model.NullTime) error
	// Delete removes the domain; ErrDomainHasLinks when links still use it.
	Delete(ctx context.Context, id uint64) error
}

//go:generate mockgen -destination=mocks/mock_routing_rule_repo.go -package=mocks . RoutingRuleRepository
type RoutingRuleRepository interface {
	// ListByLinkID returns the link's dynamic routing rules ordered by position.
	ListByLinkID(ctx context.Context, linkID uint64) ([]model.RoutingRule, error)
	// ReplaceForLink atomically replaces the link's rules with rules, assigning
	// Position from the slice index (nil/empty clears them).
	ReplaceForLink(ctx context.Context, linkID uint64, rules []model.RoutingRule) error
}

// TagRepository manages link tags. Tags are trimmed, truncated to 100
// characters, de-duplicated (case-sensitively) and blank ones dropped.
//
//go:generate mockgen -destination=mocks/mock_tag_repo.go -package=mocks . TagRepository
type TagRepository interface {
	// SetLinkTags replaces the link's tags (nil/empty clears them).
	SetLinkTags(ctx context.Context, linkID uint64, tags []string) error
	// GetLinkTags returns the link's tags sorted; never nil.
	GetLinkTags(ctx context.Context, linkID uint64) ([]string, error)
	// GetTagsForLinks returns tags keyed by link ID; links without tags are absent.
	GetTagsForLinks(ctx context.Context, linkIDs []uint64) (map[uint64][]string, error)
	// ListByGroup returns the distinct tags used by the group's links, sorted.
	ListByGroup(ctx context.Context, groupID uint64) ([]string, error)
	// AddTagsToLinks adds tags to every link; existing tags are left alone.
	AddTagsToLinks(ctx context.Context, linkIDs []uint64, tags []string) error
	// RemoveTagsFromLinks removes tags from every link.
	RemoveTagsFromLinks(ctx context.Context, linkIDs []uint64, tags []string) error
}

// QRRepository manages QR codes, each owning exactly one link, and their
// tags (normalised like TagRepository's).
//
//go:generate mockgen -destination=mocks/mock_qr_repo.go -package=mocks . QRRepository
type QRRepository interface {
	// Create inserts the QR code and sets ID and timestamps;
	// ErrQRCodeExistsForLink when the link already has one.
	Create(ctx context.Context, qr *model.QRCode) error
	// GetByID returns the QR code with its link fields; ErrQRCodeNotFound.
	GetByID(ctx context.Context, id uint64) (*QRCodeRow, error)
	// GetByLinkID returns the link's QR code; ErrQRCodeNotFound.
	GetByLinkID(ctx context.Context, linkID uint64) (*QRCodeRow, error)
	// GetByIDs returns the existing QR codes among ids; missing ids are absent.
	GetByIDs(ctx context.Context, ids []uint64) ([]QRCodeRow, error)
	// ListByGroup pages through the group's QR codes matching f, plus the
	// total match count.
	ListByGroup(ctx context.Context, groupID uint64, f QRFilter, limit, offset int) ([]QRCodeRow, int64, error)
	// CountByGroup counts the group's QR codes matching f.
	CountByGroup(ctx context.Context, groupID uint64, f QRFilter) (int64, error)
	// Update writes title and render_customizations; ErrQRCodeNotFound.
	Update(ctx context.Context, qr *model.QRCode) error
	// SetArchived hides (true) or shows (false) the QR codes; empty ids is a no-op.
	SetArchived(ctx context.Context, ids []uint64, archived bool) error
	// Delete removes the QR code (not its link); ErrQRCodeNotFound.
	Delete(ctx context.Context, id uint64) error
	// QRCodeIDsForLinks maps link ID to QR code ID; links without one are absent.
	QRCodeIDsForLinks(ctx context.Context, linkIDs []uint64) (map[uint64]uint64, error)
	// SetTags replaces the QR code's tags (nil/empty clears them).
	SetTags(ctx context.Context, qrID uint64, tags []string) error
	// GetTagsFor returns tags keyed by QR code ID; untagged ones are absent.
	GetTagsFor(ctx context.Context, ids []uint64) (map[uint64][]string, error)
	// ListTagsByGroup returns the distinct tags of the group's QR codes, sorted.
	ListTagsByGroup(ctx context.Context, groupID uint64) ([]string, error)
	// AddTags adds tags to every QR code; existing tags are left alone.
	AddTags(ctx context.Context, ids []uint64, tags []string) error
	// RemoveTags removes tags from every QR code.
	RemoveTags(ctx context.Context, ids []uint64, tags []string) error
}

//go:generate mockgen -destination=mocks/mock_activity_repo.go -package=mocks . ActivityRepository
type ActivityRepository interface {
	// Insert stores the entry and fills ID and CreatedAt.
	Insert(ctx context.Context, entry *model.ActivityEntry) error
	// List returns the group's entries newest first with the total count.
	// from is inclusive and to exclusive; nil means unbounded.
	List(ctx context.Context, groupID uint64, from, to *time.Time, limit, offset int) ([]model.ActivityEntry, int64, error)
}

// Range methods take UTC dates: from and to are both inclusive days, i.e. the
// clicks in [from 00:00, to+1d 00:00). source "" or "all" means every source,
// otherwise clicks.source must equal it. An empty linkIDs set matches nothing.
//
//go:generate mockgen -destination=mocks/mock_click_repo.go -package=mocks . ClickRepository
type ClickRepository interface {
	BatchInsert(ctx context.Context, clicks []model.Click) error
	GetTotalByLinkID(ctx context.Context, linkID uint64) (int64, error)
	// CountRange counts the links' clicks in the date range.
	CountRange(ctx context.Context, linkIDs []uint64, from, to time.Time, source string) (int64, error)
	// CountRangeByGroup counts clicks on every link of the group in the range.
	CountRangeByGroup(ctx context.Context, groupID uint64, from, to time.Time, source string) (int64, error)
	// Series returns zero-filled buckets for the range; unit "day" keys are
	// YYYY-MM-DD, unit "hour" keys are YYYY-MM-DDTHH:00.
	Series(ctx context.Context, linkIDs []uint64, from, to time.Time, unit, source string) ([]Bucket, error)
	// Breakdown groups the range's clicks by facet (country, city, referrer,
	// device, browser, os), most clicks first, at most limit rows. Empty values
	// are grouped as "Unknown" ("Direct" for referrer); city rows carry the
	// country code in Key2.
	Breakdown(ctx context.Context, linkIDs []uint64, from, to time.Time, facet string, limit int, source string) ([]Row, error)
	// TotalAllTime counts every click of the links.
	TotalAllTime(ctx context.Context, linkIDs []uint64, source string) (int64, error)
}

// Bucket is one point of a click time series.
type Bucket struct {
	Date  string `db:"bucket"`
	Count int64  `db:"cnt"`
}

// Row is one breakdown entry.
type Row struct {
	Key   string `db:"k"`
	Key2  string `db:"k2"`
	Count int64  `db:"cnt"`
}
