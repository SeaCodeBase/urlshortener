package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

var (
	ErrInvalidDomain      = errors.New("enter a valid domain name like links.example.com (no http:// or path)")
	ErrDomainExists       = errors.New("domain already in use")
	ErrInvalidRedirectURL = errors.New("redirect URLs must be valid http(s) URLs")
	// ErrDomainReserved: the domain is one of the service's own hosts.
	ErrDomainReserved = errors.New("this domain is reserved")
	// ErrDomainHasLinks: a domain can't be deleted while links use it
	// (deleting it would silently move them to another namespace).
	ErrDomainHasLinks = errors.New("this domain still has links; delete or move them first")
)

const (
	maxDomainLen       = 253
	maxRedirectURLLen  = 2048
	verifyTXTPrefix    = "_urlshortener."
	verifyValuePrefix  = "verify="
	dnsLookupTimeout   = 10 * time.Second
	verificationTokenN = 16 // bytes → 32 hex characters
)

// DNSResolver is the subset of net.Resolver used for domain verification.
type DNSResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupCNAME(ctx context.Context, name string) (string, error)
}

// NewNetResolver returns a DNSResolver backed by net.DefaultResolver.
func NewNetResolver() DNSResolver { return net.DefaultResolver }

// CreateDomainInput is the body of POST /domains. GroupID nil makes the
// domain usable by all of the user's groups.
type CreateDomainInput struct {
	Domain  string  `json:"domain"`
	GroupID *uint64 `json:"group_id"`
}

// UpdateDomainInput is a partial settings update: nil fields are unchanged,
// an empty redirect URL clears it, ClearGroup wins over GroupID.
type UpdateDomainInput struct {
	RootRedirect     *string
	NotFoundRedirect *string
	GroupID          *uint64
	ClearGroup       bool
}

// DomainCacheInvalidator drops cached redirect data for a domain
// (implemented by *RedirectService).
type DomainCacheInvalidator interface {
	// InvalidateDomain drops the cached host lookup.
	InvalidateDomain(ctx context.Context, host string) error
	// InvalidateHost drops the host lookup and every link cached under host.
	InvalidateHost(ctx context.Context, host string) error
}

// Compile-time check: DomainServiceImpl implements DomainService
var _ DomainService = (*DomainServiceImpl)(nil)

type DomainServiceImpl struct {
	domains      repository.DomainRepository
	groups       repository.GroupRepository
	links        repository.LinkRepository
	resolver     DNSResolver
	redirectHost string
	reserved     map[string]bool
	cache        DomainCacheInvalidator
}

// SetCacheInvalidator makes verify/update/delete drop the redirect server's
// cached lookup of the domain so changes apply immediately.
func (s *DomainServiceImpl) SetCacheInvalidator(c DomainCacheInvalidator) { s.cache = c }

func (s *DomainServiceImpl) invalidate(ctx context.Context, host string) {
	if s.cache == nil {
		return
	}
	if err := s.cache.InvalidateDomain(ctx, host); err != nil {
		logger.Warn(ctx, "domain-service: failed to invalidate domain cache",
			zap.String("domain", host),
			zap.Error(err),
		)
	}
}

// invalidateHost drops every cached entry of a removed domain.
func (s *DomainServiceImpl) invalidateHost(ctx context.Context, host string) {
	if s.cache == nil {
		return
	}
	if err := s.cache.InvalidateHost(ctx, host); err != nil {
		logger.Warn(ctx, "domain-service: failed to invalidate host cache",
			zap.String("domain", host),
			zap.Error(err),
		)
	}
}

// NewDomainService creates the domain service. redirectHost is the hostname
// custom domains CNAME to (config urls.redirect_host); it and reservedHosts
// (the service's own hosts, e.g. the base URL and frontend hosts; ports are
// ignored) can never be added as custom domains.
func NewDomainService(domains repository.DomainRepository, groups repository.GroupRepository, links repository.LinkRepository, resolver DNSResolver, redirectHost string, reservedHosts ...string) *DomainServiceImpl {
	s := &DomainServiceImpl{
		domains:      domains,
		groups:       groups,
		links:        links,
		resolver:     resolver,
		redirectHost: normalizeHost(redirectHost),
		reserved:     ReservedHostSet(append([]string{redirectHost}, reservedHosts...)...),
	}
	return s
}

// ReservedHostSet normalises hosts (trimmed, lowercased, port and trailing
// dot removed) into a set; empty entries are dropped.
func ReservedHostSet(hosts ...string) map[string]bool {
	set := map[string]bool{}
	for _, h := range hosts {
		if h = normalizeHost(stripPort(strings.TrimSpace(h))); h != "" {
			set[h] = true
		}
	}
	return set
}

// reservedHostError is stored on legacy domain rows for reserved hosts.
const reservedHostError = "reserved host"

// DemoteReservedDomains marks existing domain rows whose host is reserved
// (e.g. backfilled as "ok" by migration 002) as status "error" with
// verification_error "reserved host", and returns how many rows changed.
func (s *DomainServiceImpl) DemoteReservedDomains(ctx context.Context) (int, error) {
	hosts := make([]string, 0, len(s.reserved))
	for h := range s.reserved {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	n := 0
	for _, h := range hosts {
		domain, err := s.domains.GetByDomain(ctx, h)
		if errors.Is(err, repository.ErrDomainNotFound) {
			continue
		}
		if err != nil {
			return n, err
		}
		if domain.VerificationStatus == model.DomainVerificationError &&
			domain.VerificationError != nil && *domain.VerificationError == reservedHostError {
			continue
		}
		msg := reservedHostError
		if err := s.domains.UpdateVerification(ctx, domain.ID, model.DomainVerificationError, &msg,
			domain.VerifiedAt, domain.LastCheckedAt); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (s *DomainServiceImpl) List(ctx context.Context, userID uint64) ([]*model.Domain, error) {
	domains, err := s.domains.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if domains == nil {
		domains = []*model.Domain{}
	}
	return domains, nil
}

func (s *DomainServiceImpl) Create(ctx context.Context, userID uint64, input CreateDomainInput) (*model.Domain, error) {
	name, ok := normalizeDomainName(input.Domain)
	if !ok {
		return nil, ErrInvalidDomain
	}
	if s.reserved[name] {
		return nil, ErrDomainReserved
	}
	if input.GroupID != nil {
		if err := s.checkGroup(ctx, userID, *input.GroupID); err != nil {
			return nil, err
		}
	}
	token, err := newVerificationToken()
	if err != nil {
		logger.Error(ctx, "domain-service: failed to generate verification token", zap.Error(err))
		return nil, err
	}
	domain := &model.Domain{
		UserID:             userID,
		GroupID:            input.GroupID,
		Domain:             name,
		VerificationToken:  token,
		VerificationStatus: model.DomainVerificationPending,
	}
	err = s.domains.Create(ctx, domain)
	if errors.Is(err, repository.ErrDomainExists) {
		var released bool
		if released, err = s.releaseStaleClaim(ctx, userID, name); err != nil {
			return nil, err
		}
		if !released {
			return nil, ErrDomainExists
		}
		err = s.domains.Create(ctx, domain)
	}
	if err != nil {
		if errors.Is(err, repository.ErrDomainExists) {
			return nil, ErrDomainExists
		}
		return nil, err
	}
	// Reload to pick up database defaults (created_at).
	return s.domains.GetByID(ctx, domain.ID)
}

// releaseStaleClaim deletes another user's never-verified (pending or
// error) row for name so the domain can't be squatted by an unproven claim.
// It reports false, leaving the row alone, when the row is verified, belongs
// to userID, or still has links.
func (s *DomainServiceImpl) releaseStaleClaim(ctx context.Context, userID uint64, name string) (bool, error) {
	existing, err := s.domains.GetByDomain(ctx, name)
	if errors.Is(err, repository.ErrDomainNotFound) {
		return true, nil // deleted concurrently: retry the insert
	}
	if err != nil {
		return false, err
	}
	if existing.UserID == userID || existing.VerificationStatus == model.DomainVerificationOK {
		return false, nil
	}
	if s.links != nil {
		n, err := s.links.CountByDomainID(ctx, existing.ID)
		if err != nil {
			return false, err
		}
		if n > 0 {
			return false, nil
		}
	}
	if err := s.domains.Delete(ctx, existing.ID); err != nil {
		if errors.Is(err, repository.ErrDomainHasLinks) {
			return false, nil
		}
		if !errors.Is(err, repository.ErrDomainNotFound) {
			return false, err
		}
	}
	logger.Info(ctx, "domain-service: released stale unverified domain claim",
		zap.String("domain", name),
		zap.Uint64("previous_user_id", existing.UserID),
		zap.Uint64("user_id", userID),
	)
	s.invalidateHost(ctx, name)
	return true, nil
}

func (s *DomainServiceImpl) Verify(ctx context.Context, userID, domainID uint64) (*model.Domain, error) {
	domain, err := s.getOwned(ctx, userID, domainID)
	if err != nil {
		return nil, err
	}

	lookupCtx, cancel := context.WithTimeout(ctx, dnsLookupTimeout)
	defer cancel()
	failure, transient := s.checkDNS(lookupCtx, domain)

	now := model.NullTime{NullTime: sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}}
	domain.LastCheckedAt = now
	switch {
	case failure == "":
		domain.VerificationStatus = model.DomainVerificationOK
		domain.VerificationError = nil
		domain.VerifiedAt = now
	case transient != nil && domain.VerificationStatus == model.DomainVerificationOK:
		// A lookup failed without a definite answer: keep the verified
		// status rather than taking a working domain offline.
		msg := "DNS lookup failed (" + transient.Error() + "); keeping previous verification"
		domain.VerificationError = &msg
	default:
		domain.VerificationStatus = model.DomainVerificationError
		domain.VerificationError = &failure
	}
	// Only the verification columns are written so a concurrent settings
	// edit (redirects, group) is not overwritten with the values read above.
	if err := s.domains.UpdateVerification(ctx, domain.ID, domain.VerificationStatus, domain.VerificationError,
		domain.VerifiedAt, domain.LastCheckedAt); err != nil {
		if errors.Is(err, repository.ErrDomainNotFound) {
			return nil, ErrNotDomainOwner
		}
		logger.Error(ctx, "domain-service: failed to save verification result",
			zap.Uint64("domain_id", domainID),
			zap.Error(err),
		)
		return nil, err
	}
	s.invalidate(ctx, domain.Domain)
	return domain, nil
}

// checkDNS returns "" when the domain proves ownership, otherwise a
// human-readable reason. transient is the first lookup error that was not a
// definite "no such record" answer (timeouts, server failures).
func (s *DomainServiceImpl) checkDNS(ctx context.Context, domain *model.Domain) (failure string, transient error) {
	txtName := verifyTXTPrefix + domain.Domain
	want := verifyValuePrefix + domain.VerificationToken
	txtReason := "TXT record not found at " + txtName
	records, err := s.resolver.LookupTXT(ctx, txtName)
	if err != nil && !isDefiniteNotFound(err) {
		transient = err
		txtReason = "TXT lookup of " + txtName + " failed"
	}
	if err == nil && len(records) > 0 {
		for _, r := range records {
			if strings.TrimSpace(r) == want {
				return "", nil
			}
		}
		txtReason = "TXT record at " + txtName + " does not contain " + want
	}

	// The CNAME check only applies to other hosts: LookupCNAME of a name with
	// no CNAME returns the name itself, so the redirect host would "match".
	if s.redirectHost == "" || domain.Domain == s.redirectHost {
		return txtReason, transient
	}
	cname, err := s.resolver.LookupCNAME(ctx, domain.Domain)
	if err == nil && normalizeHost(cname) == s.redirectHost {
		return "", nil
	}
	if err != nil && !isDefiniteNotFound(err) && transient == nil {
		transient = err
	}
	return txtReason + ", and no CNAME to " + s.redirectHost + " was found", transient
}

// isDefiniteNotFound reports whether a resolver error is an authoritative
// "no such host/record" answer rather than a lookup failure.
func isDefiniteNotFound(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

func (s *DomainServiceImpl) Update(ctx context.Context, userID, domainID uint64, input UpdateDomainInput) (*model.Domain, error) {
	domain, err := s.getOwned(ctx, userID, domainID)
	if err != nil {
		return nil, err
	}
	if input.RootRedirect != nil {
		if domain.RootRedirect, err = normalizeRedirectURL(*input.RootRedirect); err != nil {
			return nil, err
		}
	}
	if input.NotFoundRedirect != nil {
		if domain.NotFoundRedirect, err = normalizeRedirectURL(*input.NotFoundRedirect); err != nil {
			return nil, err
		}
	}
	if input.ClearGroup {
		domain.GroupID = nil
	} else if input.GroupID != nil {
		if err := s.checkGroup(ctx, userID, *input.GroupID); err != nil {
			return nil, err
		}
		id := *input.GroupID
		domain.GroupID = &id
	}
	if err := s.domains.Update(ctx, domain); err != nil {
		if errors.Is(err, repository.ErrDomainNotFound) {
			return nil, ErrNotDomainOwner
		}
		return nil, err
	}
	s.invalidate(ctx, domain.Domain)
	return domain, nil
}

func (s *DomainServiceImpl) Delete(ctx context.Context, userID, domainID uint64) (*model.Domain, error) {
	domain, err := s.getOwned(ctx, userID, domainID)
	if err != nil {
		return nil, err
	}
	if s.links != nil {
		n, err := s.links.CountByDomainID(ctx, domainID)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, ErrDomainHasLinks
		}
	}
	if err := s.domains.Delete(ctx, domainID); err != nil {
		if errors.Is(err, repository.ErrDomainNotFound) {
			return nil, ErrNotDomainOwner
		}
		// A link was created on the domain after the count above.
		if errors.Is(err, repository.ErrDomainHasLinks) {
			return nil, ErrDomainHasLinks
		}
		return nil, err
	}
	s.invalidateHost(ctx, domain.Domain)
	return domain, nil
}

// getOwned loads a domain owned by userID; a missing or foreign domain is
// ErrNotDomainOwner so the API does not reveal which IDs exist.
func (s *DomainServiceImpl) getOwned(ctx context.Context, userID, domainID uint64) (*model.Domain, error) {
	domain, err := s.domains.GetByID(ctx, domainID)
	if errors.Is(err, repository.ErrDomainNotFound) {
		return nil, ErrNotDomainOwner
	}
	if err != nil {
		return nil, err
	}
	if domain.UserID != userID {
		return nil, ErrNotDomainOwner
	}
	return domain, nil
}

// checkGroup requires groupID to be one of the user's groups.
func (s *DomainServiceImpl) checkGroup(ctx context.Context, userID, groupID uint64) error {
	group, err := s.groups.GetByID(ctx, groupID)
	if errors.Is(err, repository.ErrGroupNotFound) {
		return ErrGroupNotFound
	}
	if err != nil {
		logger.Error(ctx, "domain-service: failed to get group",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return err
	}
	if group.UserID != userID {
		return ErrGroupNotFound
	}
	return nil
}

func newVerificationToken() (string, error) {
	b := make([]byte, verificationTokenN)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// normalizeHost lowercases a DNS name and drops one trailing dot.
func normalizeHost(h string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
}

// normalizeDomainName lowercases the input and checks it is a hostname of at
// least two RFC 1123 labels, at most 253 characters, with a non-numeric TLD
// (so IP addresses are rejected). Schemes, paths and ports are not allowed.
func normalizeDomainName(in string) (string, bool) {
	name := normalizeHost(in)
	if name == "" || len(name) > maxDomainLen {
		return "", false
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return "", false
	}
	for _, l := range labels {
		if !validLabel(l) {
			return "", false
		}
	}
	tld := labels[len(labels)-1]
	if strings.Trim(tld, "0123456789") == "" {
		return "", false
	}
	return name, true
}

func validLabel(l string) bool {
	if len(l) == 0 || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
		return false
	}
	for i := 0; i < len(l); i++ {
		c := l[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// normalizeRedirectURL trims raw; "" clears the setting (nil), otherwise it
// must be an absolute http(s) URL with a host.
func normalizeRedirectURL(raw string) (*string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if len(raw) > maxRedirectURLLen {
		return nil, ErrInvalidRedirectURL
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, ErrInvalidRedirectURL
	}
	return &raw, nil
}
