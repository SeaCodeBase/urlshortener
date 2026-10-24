package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

var (
	ErrLinkNotFound     = errors.New("link not found")
	ErrNotLinkOwner     = errors.New("not the owner of this link")
	ErrInvalidShortCode = errors.New("invalid short code")
	ErrShortCodeTaken   = errors.New("short code already taken")
	// ErrLinkNotDeletable: the link was redirected or has a custom back-half
	// (spec §5.2); it can only be hidden.
	ErrLinkNotDeletable = errors.New("link can't be deleted")
	// ErrGroupRequired: links must belong to a group.
	ErrGroupRequired = errors.New("group is required")
	// ErrDomainNotFound: the domain does not exist or belongs to another user.
	ErrDomainNotFound = errors.New("domain not found")
	// ErrInvalidRoutingRule: a routing rule has no valid http(s) destination.
	ErrInvalidRoutingRule = errors.New("invalid routing rule")
	// ErrExpirationTooSoon: expires_at must leave time to use the link.
	ErrExpirationTooSoon = errors.New("expiration must be at least 5 minutes in the future")
)

const (
	maxPageSize = 100
	// maxURLLength bounds destination URLs (spec §5.3).
	maxURLLength = 2048
	// maxHistoryDepth bounds the back-half history walk.
	maxHistoryDepth = 20
	// exportPageSize is the batch size used while exporting.
	exportPageSize = 500
	// maxExportRows caps one export.
	maxExportRows = 100000
	// minExpirationLead is how far in the future a new expiration must be.
	minExpirationLead = 5 * time.Minute
)

// checkExpiration rejects an expiration earlier than now + 5 minutes.
func checkExpiration(at time.Time) error {
	if at.Before(time.Now().Add(minExpirationLead)) {
		return ErrExpirationTooSoon
	}
	return nil
}

// LinkExportHeader is the CSV header of GET /links/export (spec §5.2).
var LinkExportHeader = []string{"Link", "Custom Link", "Date created", "Title", "Destination URL", "Engagements", "Status"}

// Compile-time check: LinkServiceImpl implements LinkService
var _ LinkService = (*LinkServiceImpl)(nil)

// LinkServiceDeps are the collaborators of LinkServiceImpl.
type LinkServiceDeps struct {
	Links      repository.LinkRepository
	ShortCodes ShortCodeService
	Tags       repository.TagRepository
	Rules      repository.RoutingRuleRepository
	Domains    repository.DomainRepository
	// Groups supplies the active group's default domain; nil disables it.
	Groups   repository.GroupRepository
	Clicks   repository.ClickRepository
	Activity ActivityService
	// TitleFetcher fetches a destination's title when none is given;
	// nil selects FetchTitle.
	TitleFetcher TitleFetcher
	// BaseURL is the default short domain (urls.base_url), used by Export.
	BaseURL string
}

type LinkServiceImpl struct {
	linkRepo     repository.LinkRepository
	shortCode    ShortCodeService
	tagRepo      repository.TagRepository
	ruleRepo     repository.RoutingRuleRepository
	domainRepo   repository.DomainRepository
	groupRepo    repository.GroupRepository
	clickRepo    repository.ClickRepository
	activity     ActivityService
	titleFetcher TitleFetcher
	baseURL      string
}

func NewLinkService(d LinkServiceDeps) *LinkServiceImpl {
	tf := d.TitleFetcher
	if tf == nil {
		tf = FetchTitle
	}
	return &LinkServiceImpl{
		linkRepo:     d.Links,
		shortCode:    d.ShortCodes,
		tagRepo:      d.Tags,
		ruleRepo:     d.Rules,
		domainRepo:   d.Domains,
		groupRepo:    d.Groups,
		clickRepo:    d.Clicks,
		activity:     d.Activity,
		titleFetcher: tf,
		baseURL:      strings.TrimRight(d.BaseURL, "/"),
	}
}

// RoutingRuleInput is a dynamic routing rule as sent by the client. Empty
// match lists match everything.
type RoutingRuleInput struct {
	LongURL        string   `json:"long_url"`
	DeviceMatch    []string `json:"device_match"`
	OSMatch        []string `json:"os_match"`
	CountryMatch   []string `json:"country_match"`
	CountryExclude []string `json:"country_exclude"`
}

type CreateLinkInput struct {
	OriginalURL string     `json:"original_url" binding:"required,url"`
	CustomCode  string     `json:"custom_code,omitempty"`
	Title       string     `json:"title,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	// DomainID nil uses the active group's default domain (if any); 0
	// explicitly selects the service's default domain.
	DomainID     *uint64            `json:"domain_id,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
	UTM          *UTMParams         `json:"utm,omitempty"`
	RoutingRules []RoutingRuleInput `json:"routing_rules,omitempty"`
	// GroupID is set by the handler from the active group, never from JSON.
	GroupID uint64 `json:"-"`
}

type UpdateLinkInput struct {
	OriginalURL string `json:"original_url,omitempty"`
	// Title nil keeps the title; "" clears it.
	Title     *string    `json:"title,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// ClearExpiration removes the expiration (takes precedence over ExpiresAt).
	ClearExpiration bool    `json:"clear_expiration,omitempty"`
	IsActive        *bool   `json:"is_active,omitempty"`
	DomainID        *uint64 `json:"domain_id,omitempty"`
	// Tags replaces the link's tags when non-nil.
	Tags *[]string `json:"tags,omitempty"`
}

type ListLinksParams struct {
	Filter repository.LinkFilter
	Page   int
	Limit  int
}

type ListLinksResult struct {
	Links      []model.LinkWithStats `json:"links"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	TotalPages int                   `json:"total_pages"`
}

// LinkDetails is a link with everything the details page shows.
type LinkDetails struct {
	model.Link
	Tags         []string
	TotalClicks  int64
	RoutingRules []model.RoutingRule
	// History is the chain of ancestor links (parent first) created by
	// back-half edits.
	History []model.Link
}

// hostOf returns the lowercased hostname of rawURL, or nil.
func hostOf(rawURL string) *string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	h := strings.ToLower(u.Hostname())
	return &h
}

// validDestination reports whether rawURL is an absolute http(s) URL.
func validDestination(rawURL string) bool {
	if len(rawURL) > maxURLLength {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// maxRoutingRules bounds the routing rules of one link.
const maxRoutingRules = 20

var (
	routingDevices = map[string]bool{"mobile": true, "tablet": true, "desktop": true}
	routingOSes    = map[string]bool{"ios": true, "android": true}
)

// normalizeRuleValues trims and lowercases (or uppercases) values and checks
// each with valid; field and pos name the problem in the error.
func normalizeRuleValues(pos int, field string, values []string, upper bool, valid func(string) bool, want string) (model.StringList, error) {
	out := make(model.StringList, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if upper {
			v = strings.ToUpper(v)
		} else {
			v = strings.ToLower(v)
		}
		if !valid(v) {
			return nil, fmt.Errorf("%w: rule %d: %s value %q must be %s", ErrInvalidRoutingRule, pos, field, v, want)
		}
		out = append(out, v)
	}
	return out, nil
}

func isCountryCode(v string) bool {
	return len(v) == 2 && v[0] >= 'A' && v[0] <= 'Z' && v[1] >= 'A' && v[1] <= 'Z'
}

// toRoutingRules validates and normalizes client routing rules (used by
// Create and SetRoutingRules). Errors wrap ErrInvalidRoutingRule and name the
// offending rule (1-based) and field.
func toRoutingRules(in []RoutingRuleInput) ([]model.RoutingRule, error) {
	if len(in) > maxRoutingRules {
		return nil, fmt.Errorf("%w: a link can have at most %d routing rules", ErrInvalidRoutingRule, maxRoutingRules)
	}
	rules := make([]model.RoutingRule, 0, len(in))
	for i, r := range in {
		pos := i + 1
		dest := strings.TrimSpace(r.LongURL)
		if !validDestination(dest) {
			return nil, fmt.Errorf("%w: rule %d: long_url must be a valid http(s) URL of at most %d characters",
				ErrInvalidRoutingRule, pos, maxURLLength)
		}
		rule := model.RoutingRule{LongURL: dest}
		var err error
		if rule.DeviceMatch, err = normalizeRuleValues(pos, "device_match", r.DeviceMatch, false,
			func(v string) bool { return routingDevices[v] }, "one of mobile, tablet, desktop"); err != nil {
			return nil, err
		}
		if rule.OSMatch, err = normalizeRuleValues(pos, "os_match", r.OSMatch, false,
			func(v string) bool { return routingOSes[v] }, "one of ios, android"); err != nil {
			return nil, err
		}
		if rule.CountryMatch, err = normalizeRuleValues(pos, "country_match", r.CountryMatch, true,
			isCountryCode, "a 2-letter ISO country code"); err != nil {
			return nil, err
		}
		if rule.CountryExclude, err = normalizeRuleValues(pos, "country_exclude", r.CountryExclude, true,
			isCountryCode, "a 2-letter ISO country code"); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// detached returns a context that outlives request cancellation, so that
// activity entries are not dropped when the client disconnects.
func detached(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}

// checkDomain verifies that domainID (when set) belongs to userID, is
// verified, and is usable by groupID (domains bound to another group are
// reported as not found).
func (s *LinkServiceImpl) checkDomain(ctx context.Context, userID, groupID uint64, domainID *uint64) error {
	if domainID == nil {
		return nil
	}
	d, err := s.domainRepo.GetByID(ctx, *domainID)
	if errors.Is(err, repository.ErrDomainNotFound) {
		return ErrDomainNotFound
	}
	if err != nil {
		return err
	}
	if d.UserID != userID || (d.GroupID != nil && *d.GroupID != groupID) {
		return ErrDomainNotFound
	}
	if d.VerificationStatus != model.DomainVerificationOK {
		return ErrDomainNotVerified
	}
	return nil
}

// groupDefaultDomain returns the group's default_domain_id when it is still
// usable for new links (owned, verified, not bound to another group);
// otherwise nil, the service's default domain.
func (s *LinkServiceImpl) groupDefaultDomain(ctx context.Context, userID, groupID uint64) *uint64 {
	if s.groupRepo == nil {
		return nil
	}
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		if !errors.Is(err, repository.ErrGroupNotFound) {
			logger.Warn(ctx, "link-service: failed to load group default domain",
				zap.Uint64("group_id", groupID),
				zap.Error(err),
			)
		}
		return nil
	}
	if group.DefaultDomainID == nil {
		return nil
	}
	id := *group.DefaultDomainID
	if err := s.checkDomain(ctx, userID, groupID, &id); err != nil {
		logger.Warn(ctx, "link-service: group default domain is not usable; using the default domain",
			zap.Uint64("group_id", groupID),
			zap.Uint64("domain_id", id),
			zap.Error(err),
		)
		return nil
	}
	return &id
}

// claimBackHalf validates code and checks it is free on the domain.
func (s *LinkServiceImpl) claimBackHalf(ctx context.Context, domainID *uint64, code string) error {
	if !s.shortCode.IsValid(code) {
		return ErrInvalidShortCode
	}
	available, err := s.shortCode.IsAvailable(ctx, domainID, code)
	if err != nil {
		return err
	}
	if !available {
		return ErrShortCodeTaken
	}
	return nil
}

func (s *LinkServiceImpl) Create(ctx context.Context, userID uint64, input CreateLinkInput) (*model.Link, error) {
	if input.GroupID == 0 {
		return nil, ErrGroupRequired
	}
	dest := strings.TrimSpace(input.OriginalURL)
	if input.UTM != nil {
		withUTM, err := ApplyUTM(dest, *input.UTM)
		if err != nil {
			return nil, err
		}
		dest = withUTM
	}
	if !validDestination(dest) {
		return nil, ErrInvalidURL
	}
	rules, err := toRoutingRules(input.RoutingRules)
	if err != nil {
		return nil, err
	}
	if input.ExpiresAt != nil {
		if err := checkExpiration(*input.ExpiresAt); err != nil {
			return nil, err
		}
	}
	switch {
	case input.DomainID == nil:
		input.DomainID = s.groupDefaultDomain(ctx, userID, input.GroupID)
	case *input.DomainID == 0:
		input.DomainID = nil
	default:
		if err := s.checkDomain(ctx, userID, input.GroupID, input.DomainID); err != nil {
			return nil, err
		}
	}

	var shortCode string
	if input.CustomCode != "" {
		if err := s.claimBackHalf(ctx, input.DomainID, input.CustomCode); err != nil {
			if !errors.Is(err, ErrInvalidShortCode) && !errors.Is(err, ErrShortCodeTaken) {
				logger.Error(ctx, "link-service: failed to check code availability",
					zap.Uint64("user_id", userID),
					zap.String("custom_code", input.CustomCode),
					zap.Error(err),
				)
			}
			return nil, err
		}
		shortCode = input.CustomCode
	} else {
		shortCode, err = s.shortCode.Generate(ctx, input.DomainID)
		if err != nil {
			logger.Error(ctx, "link-service: failed to generate short code",
				zap.Uint64("user_id", userID),
				zap.Error(err),
			)
			return nil, err
		}
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = s.titleFetcher(ctx, dest)
	}

	link := &model.Link{
		UserID:            userID,
		GroupID:           input.GroupID,
		ShortCode:         shortCode,
		OriginalURL:       dest,
		LongURLHost:       hostOf(dest),
		IsActive:          true,
		HasCustomBackHalf: input.CustomCode != "",
		Source:            model.LinkSourceLink,
		DomainID:          input.DomainID,
	}
	if title != "" {
		link.Title = &title
	}
	if input.ExpiresAt != nil {
		link.ExpiresAt = model.NullTime{NullTime: sql.NullTime{Time: *input.ExpiresAt, Valid: true}}
	}

	if err := s.linkRepo.Create(ctx, link); err != nil {
		if errors.Is(err, repository.ErrShortCodeExists) {
			return nil, ErrShortCodeTaken
		}
		logger.Error(ctx, "link-service: failed to create link",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	if err := s.saveExtras(ctx, link.ID, repository.NormalizeTags(input.Tags), rules); err != nil {
		logger.Error(ctx, "link-service: failed to save tags/routing rules; removing link",
			zap.Uint64("link_id", link.ID),
			zap.Error(err),
		)
		if delErr := s.linkRepo.Delete(detached(ctx), link.ID); delErr != nil {
			logger.Error(ctx, "link-service: failed to remove partially created link",
				zap.Uint64("link_id", link.ID),
				zap.Error(delErr),
			)
		}
		return nil, err
	}

	s.activity.Log(detached(ctx), userID, link.GroupID, ActionLinkCreate, EntityLink, link.ID,
		map[string]any{"short_code": link.ShortCode})
	return link, nil
}

// saveExtras stores tags and routing rules of a freshly created link.
func (s *LinkServiceImpl) saveExtras(ctx context.Context, linkID uint64, tags []string, rules []model.RoutingRule) error {
	if len(tags) > 0 {
		if err := s.tagRepo.SetLinkTags(ctx, linkID, tags); err != nil {
			return err
		}
	}
	if len(rules) > 0 {
		for i := range rules {
			rules[i].ID = 0
			rules[i].LinkID = linkID
		}
		if err := s.ruleRepo.ReplaceForLink(ctx, linkID, rules); err != nil {
			return err
		}
	}
	return nil
}

func (s *LinkServiceImpl) GetByID(ctx context.Context, userID, linkID uint64) (*model.Link, error) {
	link, err := s.linkRepo.GetByID(ctx, linkID)
	if errors.Is(err, repository.ErrLinkNotFound) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		logger.Error(ctx, "link-service: failed to get link by ID",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	if link.UserID != userID {
		return nil, ErrNotLinkOwner
	}

	return link, nil
}

// GetDetails returns the link with tags, total clicks, routing rules and its
// back-half history.
func (s *LinkServiceImpl) GetDetails(ctx context.Context, userID, linkID uint64) (*LinkDetails, error) {
	link, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return nil, err
	}
	d := &LinkDetails{Link: *link, History: []model.Link{}}

	if d.Tags, err = s.tagRepo.GetLinkTags(ctx, link.ID); err != nil {
		return nil, err
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if d.RoutingRules, err = s.ruleRepo.ListByLinkID(ctx, link.ID); err != nil {
		return nil, err
	}
	if d.RoutingRules == nil {
		d.RoutingRules = []model.RoutingRule{}
	}
	if d.TotalClicks, err = s.clickRepo.GetTotalByLinkID(ctx, link.ID); err != nil {
		return nil, err
	}

	parent := link.ParentLinkID
	for i := 0; parent != nil && i < maxHistoryDepth; i++ {
		p, err := s.linkRepo.GetByID(ctx, *parent)
		if errors.Is(err, repository.ErrLinkNotFound) {
			break
		}
		if err != nil {
			return nil, err
		}
		d.History = append(d.History, *p)
		parent = p.ParentLinkID
	}
	return d, nil
}

func normalizePaging(page, limit int) (int, int) {
	if limit <= 0 {
		limit = 20
	} else if limit > maxPageSize {
		limit = maxPageSize
	}
	if page <= 0 {
		page = 1
	}
	return page, limit
}

// List pages through the group's links matching params.Filter, with tags.
func (s *LinkServiceImpl) List(ctx context.Context, groupID uint64, params ListLinksParams) (*ListLinksResult, error) {
	page, limit := normalizePaging(params.Page, params.Limit)
	offset := (page - 1) * limit

	links, total, err := s.linkRepo.ListByGroup(ctx, groupID, params.Filter, limit, offset)
	if err != nil {
		logger.Error(ctx, "link-service: failed to list links",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return nil, err
	}
	if err := s.attachTags(ctx, links); err != nil {
		return nil, err
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	return &ListLinksResult{
		Links:      links,
		Total:      total,
		Page:       page,
		TotalPages: totalPages,
	}, nil
}

// attachTags fills Tags (never nil) on every link.
func (s *LinkServiceImpl) attachTags(ctx context.Context, links []model.LinkWithStats) error {
	if len(links) == 0 {
		return nil
	}
	ids := make([]uint64, len(links))
	for i := range links {
		ids[i] = links[i].ID
	}
	byLink, err := s.tagRepo.GetTagsForLinks(ctx, ids)
	if err != nil {
		logger.Error(ctx, "link-service: failed to load tags",
			zap.Int("links", len(ids)),
			zap.Error(err),
		)
		return err
	}
	for i := range links {
		if tags, ok := byLink[links[i].ID]; ok {
			links[i].Tags = tags
		} else {
			links[i].Tags = []string{}
		}
	}
	return nil
}

// Update edits the link in place and returns it together with a snapshot of
// the link before the update (so callers can invalidate the old cache keys).
func (s *LinkServiceImpl) Update(ctx context.Context, userID, linkID uint64, input UpdateLinkInput) (*model.Link, *model.Link, error) {
	link, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return nil, nil, err
	}
	prev := *link

	changed := []string{}
	redirected := false
	if dest := strings.TrimSpace(input.OriginalURL); dest != "" && dest != link.OriginalURL {
		if !validDestination(dest) {
			return nil, nil, ErrInvalidURL
		}
		link.OriginalURL = dest
		link.LongURLHost = hostOf(dest)
		redirected = true
		changed = append(changed, "original_url")
	}
	if input.Title != nil {
		if title := strings.TrimSpace(*input.Title); title == "" {
			link.Title = nil
		} else {
			link.Title = &title
		}
		changed = append(changed, "title")
	}
	if input.ClearExpiration {
		link.ExpiresAt = model.NullTime{}
		changed = append(changed, "expires_at")
	} else if input.ExpiresAt != nil {
		// Re-sending the current expiration (e.g. from an edit form) is
		// not a change and is not re-validated.
		if !link.ExpiresAt.Valid || !link.ExpiresAt.Time.Equal(*input.ExpiresAt) {
			if err := checkExpiration(*input.ExpiresAt); err != nil {
				return nil, nil, err
			}
		}
		link.ExpiresAt = model.NullTime{NullTime: sql.NullTime{Time: *input.ExpiresAt, Valid: true}}
		changed = append(changed, "expires_at")
	}
	if input.IsActive != nil {
		link.IsActive = *input.IsActive
		changed = append(changed, "is_active")
	}
	if input.DomainID != nil && (link.DomainID == nil || *link.DomainID != *input.DomainID) {
		if err := s.checkDomain(ctx, userID, link.GroupID, input.DomainID); err != nil {
			return nil, nil, err
		}
		available, err := s.shortCode.IsAvailable(ctx, input.DomainID, link.ShortCode)
		if err != nil {
			return nil, nil, err
		}
		if !available {
			return nil, nil, ErrShortCodeTaken
		}
		link.DomainID = input.DomainID
		changed = append(changed, "domain_id")
	}

	if err := s.linkRepo.Update(ctx, link); err != nil {
		if errors.Is(err, repository.ErrLinkNotFound) {
			return nil, nil, ErrLinkNotFound
		}
		if errors.Is(err, repository.ErrShortCodeExists) {
			return nil, nil, ErrShortCodeTaken
		}
		logger.Error(ctx, "link-service: failed to update link",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, nil, err
	}
	if input.Tags != nil {
		if err := s.tagRepo.SetLinkTags(ctx, link.ID, repository.NormalizeTags(*input.Tags)); err != nil {
			return nil, nil, err
		}
		changed = append(changed, "tags")
	}
	if redirected {
		if err := s.linkRepo.IncrementRedirectCount(ctx, link.ID); err != nil {
			logger.Error(ctx, "link-service: failed to increment redirect count",
				zap.Uint64("link_id", link.ID),
				zap.Error(err),
			)
			return nil, nil, err
		}
		link.RedirectCount++
	}

	s.activity.Log(detached(ctx), userID, link.GroupID, ActionLinkUpdate, EntityLink, link.ID,
		map[string]any{"short_code": link.ShortCode, "fields": changed})
	return link, &prev, nil
}

// SetTags replaces a link's tags and returns the normalized set.
func (s *LinkServiceImpl) SetTags(ctx context.Context, userID, linkID uint64, tags []string) ([]string, error) {
	link, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return nil, err
	}
	normalized := repository.NormalizeTags(tags)
	if err := s.tagRepo.SetLinkTags(ctx, link.ID, normalized); err != nil {
		return nil, err
	}
	s.activity.Log(detached(ctx), userID, link.GroupID, ActionLinkUpdate, EntityLink, link.ID,
		map[string]any{"short_code": link.ShortCode, "fields": []string{"tags"}})
	return normalized, nil
}

// GetRoutingRules returns the link's routing rules in evaluation order.
func (s *LinkServiceImpl) GetRoutingRules(ctx context.Context, userID, linkID uint64) ([]model.RoutingRule, error) {
	link, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return nil, err
	}
	rules, err := s.ruleRepo.ListByLinkID(ctx, link.ID)
	if err != nil {
		return nil, err
	}
	if rules == nil {
		rules = []model.RoutingRule{}
	}
	return rules, nil
}

// SetRoutingRules validates and replaces the link's routing rules (an empty
// slice clears them) and returns the stored rules with ids and positions.
// Callers must invalidate the link's cached redirect.
func (s *LinkServiceImpl) SetRoutingRules(ctx context.Context, userID, linkID uint64, in []RoutingRuleInput) ([]model.RoutingRule, error) {
	link, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return nil, err
	}
	rules, err := toRoutingRules(in)
	if err != nil {
		return nil, err
	}
	for i := range rules {
		rules[i].LinkID = link.ID
	}
	if err := s.ruleRepo.ReplaceForLink(ctx, link.ID, rules); err != nil {
		logger.Error(ctx, "link-service: failed to replace routing rules",
			zap.Uint64("link_id", link.ID),
			zap.Error(err),
		)
		return nil, err
	}
	s.activity.Log(detached(ctx), userID, link.GroupID, ActionLinkUpdate, EntityLink, link.ID,
		map[string]any{"short_code": link.ShortCode, "routing_rules": len(rules)})
	return rules, nil
}

// groupLinks loads the distinct ids and fails with ErrLinkNotFound unless
// every one exists and belongs to groupID.
func (s *LinkServiceImpl) groupLinks(ctx context.Context, groupID uint64, ids []uint64) ([]uint64, []model.Link, error) {
	seen := make(map[uint64]bool, len(ids))
	unique := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return nil, nil, ErrLinkNotFound
	}
	links, err := s.linkRepo.GetByIDs(ctx, unique)
	if err != nil {
		return nil, nil, err
	}
	if len(links) != len(unique) {
		return nil, nil, ErrLinkNotFound
	}
	for _, l := range links {
		if l.GroupID != groupID {
			return nil, nil, ErrLinkNotFound
		}
	}
	return unique, links, nil
}

// SetArchived hides or shows the links (all must belong to groupID) and
// returns them with the new state.
func (s *LinkServiceImpl) SetArchived(ctx context.Context, userID, groupID uint64, ids []uint64, archived bool) ([]model.Link, error) {
	unique, links, err := s.groupLinks(ctx, groupID, ids)
	if err != nil {
		return nil, err
	}
	if err := s.linkRepo.SetArchived(ctx, unique, archived); err != nil {
		logger.Error(ctx, "link-service: failed to set archived",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return nil, err
	}
	action := ActionLinkUnhide
	if archived {
		action = ActionLinkHide
	}
	for i := range links {
		links[i].Archived = archived
		s.activity.Log(detached(ctx), userID, groupID, action, EntityLink, links[i].ID,
			map[string]any{"short_code": links[i].ShortCode})
	}
	return links, nil
}

// BulkTags adds and removes tags on links that must all belong to groupID.
func (s *LinkServiceImpl) BulkTags(ctx context.Context, groupID uint64, ids []uint64, add, remove []string) error {
	unique, _, err := s.groupLinks(ctx, groupID, ids)
	if err != nil {
		return err
	}
	if add = repository.NormalizeTags(add); len(add) > 0 {
		if err := s.tagRepo.AddTagsToLinks(ctx, unique, add); err != nil {
			return err
		}
	}
	if remove = repository.NormalizeTags(remove); len(remove) > 0 {
		if err := s.tagRepo.RemoveTagsFromLinks(ctx, unique, remove); err != nil {
			return err
		}
	}
	return nil
}

// EditBackHalf creates a new link with backHalf on the domain, copying the
// old link's destination, title, tags, expiration and routing rules, with
// ParentLinkID pointing at the old link (which is left untouched).
// domainID nil keeps the old link's domain; 0 selects the default domain.
func (s *LinkServiceImpl) EditBackHalf(ctx context.Context, userID, linkID uint64, domainID *uint64, backHalf string) (*model.Link, error) {
	old, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return nil, err
	}
	switch {
	case domainID == nil:
		domainID = old.DomainID
	case *domainID == 0:
		domainID = nil
	default:
		if err := s.checkDomain(ctx, userID, old.GroupID, domainID); err != nil {
			return nil, err
		}
	}
	if err := s.claimBackHalf(ctx, domainID, backHalf); err != nil {
		return nil, err
	}

	tags, err := s.tagRepo.GetLinkTags(ctx, old.ID)
	if err != nil {
		return nil, err
	}
	rules, err := s.ruleRepo.ListByLinkID(ctx, old.ID)
	if err != nil {
		return nil, err
	}

	parentID := old.ID
	link := &model.Link{
		UserID:            old.UserID,
		GroupID:           old.GroupID,
		ShortCode:         backHalf,
		OriginalURL:       old.OriginalURL,
		LongURLHost:       old.LongURLHost,
		Title:             old.Title,
		ExpiresAt:         old.ExpiresAt,
		IsActive:          true,
		HasCustomBackHalf: true,
		ParentLinkID:      &parentID,
		Source:            old.Source,
		DomainID:          domainID,
	}
	if link.Source == "" {
		link.Source = model.LinkSourceLink
	}
	if err := s.linkRepo.Create(ctx, link); err != nil {
		if errors.Is(err, repository.ErrShortCodeExists) {
			return nil, ErrShortCodeTaken
		}
		logger.Error(ctx, "link-service: failed to create back-half link",
			zap.Uint64("parent_link_id", old.ID),
			zap.Error(err),
		)
		return nil, err
	}
	if err := s.saveExtras(ctx, link.ID, tags, rules); err != nil {
		logger.Error(ctx, "link-service: failed to copy tags/routing rules; removing link",
			zap.Uint64("link_id", link.ID),
			zap.Error(err),
		)
		if delErr := s.linkRepo.Delete(detached(ctx), link.ID); delErr != nil {
			logger.Error(ctx, "link-service: failed to remove partially created link",
				zap.Uint64("link_id", link.ID),
				zap.Error(delErr),
			)
		}
		return nil, err
	}

	s.activity.Log(detached(ctx), userID, link.GroupID, ActionLinkBackHalf, EntityLink, link.ID,
		map[string]any{"short_code": link.ShortCode, "parent_link_id": old.ID, "parent_short_code": old.ShortCode})
	return link, nil
}

// Delete removes a deletable link and returns it (for cache invalidation).
func (s *LinkServiceImpl) Delete(ctx context.Context, userID, linkID uint64) (*model.Link, error) {
	link, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return nil, err
	}
	if link.RedirectCount > 0 || link.HasCustomBackHalf {
		return nil, ErrLinkNotDeletable
	}

	if err := s.linkRepo.Delete(ctx, link.ID); err != nil {
		if errors.Is(err, repository.ErrLinkNotFound) {
			return nil, ErrLinkNotFound
		}
		logger.Error(ctx, "link-service: failed to delete link",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	s.activity.Log(detached(ctx), userID, link.GroupID, ActionLinkDelete, EntityLink, link.ID,
		map[string]any{"short_code": link.ShortCode})
	return link, nil
}

// ShortURL builds a link's public short URL: the custom domain (with the
// base URL's scheme) when domainName is set, otherwise baseURL.
func ShortURL(baseURL string, link *model.Link, domainName string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if link.DomainID != nil && domainName != "" {
		scheme := "https"
		if u, err := url.Parse(baseURL); err == nil && u.Scheme != "" {
			scheme = u.Scheme
		}
		return scheme + "://" + domainName + "/" + link.ShortCode
	}
	return baseURL + "/" + link.ShortCode
}

// Export returns one CSV row per link matching f (columns per
// LinkExportHeader, header not included), newest first by default.
func (s *LinkServiceImpl) Export(ctx context.Context, groupID uint64, f repository.LinkFilter) ([][]string, error) {
	rows := [][]string{}
	domains := map[uint64]string{}
	for offset := 0; offset < maxExportRows; offset += exportPageSize {
		links, _, err := s.linkRepo.ListByGroup(ctx, groupID, f, exportPageSize, offset)
		if err != nil {
			logger.Error(ctx, "link-service: failed to export links",
				zap.Uint64("group_id", groupID),
				zap.Error(err),
			)
			return nil, err
		}
		for i := range links {
			l := &links[i]
			domainName := ""
			if l.DomainID != nil {
				name, ok := domains[*l.DomainID]
				if !ok {
					if d, err := s.domainRepo.GetByID(ctx, *l.DomainID); err == nil {
						name = d.Domain
					}
					domains[*l.DomainID] = name
				}
				domainName = name
			}
			short := ShortURL(s.baseURL, &l.Link, domainName)
			custom := ""
			if l.HasCustomBackHalf {
				custom = short
			}
			title := ""
			if l.Title != nil {
				title = *l.Title
			}
			status := "active"
			if l.Archived {
				status = "hidden"
			}
			rows = append(rows, []string{
				csvSafe(short), csvSafe(custom), l.CreatedAt.UTC().Format(time.RFC3339), csvSafe(title), csvSafe(l.OriginalURL),
				strconv.FormatInt(l.TotalClicks, 10), status,
			})
		}
		if len(links) < exportPageSize {
			break
		}
	}
	return rows, nil
}

// csvSafe neutralizes spreadsheet formula injection: a cell starting with
// = + - @ TAB or CR is prefixed with an apostrophe.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}
