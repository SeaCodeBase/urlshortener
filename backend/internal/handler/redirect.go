// backend/internal/handler/redirect.go
package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/util"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LinkResolver resolves a host+short code to a link (service.RedirectService).
type LinkResolver interface {
	Resolve(ctx context.Context, host, code string) (*service.ResolvedLink, error)
}

// ClickRecorder buffers click events (service.ClickService).
type ClickRecorder interface {
	RecordClick(ctx context.Context, ev service.ClickEvent) error
}

const (
	titleNotFound       = "Link not found"
	messageNotFound     = "The link you followed doesn't exist or may have been removed."
	titleExpired        = "This link has expired"
	messageExpired      = "The link you followed is no longer active because it has expired."
	titleInactive       = "This link is no longer available"
	messageInactive     = "The link you followed has been disabled by its owner."
	titlePageNotFound   = "Page not found"
	messagePageNotFound = "The page you followed doesn't exist or may have been removed."
)

type RedirectHandler struct {
	resolver    LinkResolver
	clicks      ClickRecorder
	domainRepo  repository.DomainRepository
	routing     *service.RoutingService
	ipSalt      string
	frontendURL string
	reserved    map[string]bool
}

// NewRedirectHandler builds the handler. reservedHosts (and the frontend
// URL's host) are the service's own hosts: Root never applies a domain's
// root_redirect for them.
func NewRedirectHandler(resolver LinkResolver, clicks ClickRecorder, domainRepo repository.DomainRepository, ipSalt, frontendURL string, reservedHosts ...string) *RedirectHandler {
	if u, err := url.Parse(frontendURL); err == nil && u.Host != "" {
		reservedHosts = append(reservedHosts, u.Host)
	}
	return &RedirectHandler{
		reserved:    service.ReservedHostSet(reservedHosts...),
		resolver:    resolver,
		clicks:      clicks,
		domainRepo:  domainRepo,
		routing:     service.NewRoutingService(),
		ipSalt:      ipSalt,
		frontendURL: frontendURL,
	}
}

func (h *RedirectHandler) hashIP(ip string) string {
	return util.HashIP(h.ipSalt, ip)
}

// clickSourceFromQuery maps ?r=qr|page to a click source; anything else is a
// plain link click.
func clickSourceFromQuery(r string) string {
	switch r {
	case model.LinkSourceQR, model.LinkSourcePage:
		return r
	}
	return model.LinkSourceLink
}

func (h *RedirectHandler) Redirect(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		renderErrorPage(c, http.StatusNotFound, titleNotFound, messageNotFound, h.frontendURL)
		return
	}

	res, err := h.resolver.Resolve(c.Request.Context(), c.Request.Host, code)
	switch {
	case errors.Is(err, service.ErrLinkNotFound):
		if res != nil && res.ExpiredRedirect != "" {
			c.Redirect(http.StatusFound, res.ExpiredRedirect)
			return
		}
		renderErrorPage(c, http.StatusNotFound, titleNotFound, messageNotFound, h.frontendURL)
		return
	case errors.Is(err, service.ErrLinkExpired):
		if res != nil && res.ExpiredRedirect != "" {
			c.Redirect(http.StatusFound, res.ExpiredRedirect)
			return
		}
		renderErrorPage(c, http.StatusGone, titleExpired, messageExpired, h.frontendURL)
		return
	case errors.Is(err, service.ErrLinkInactive):
		renderErrorPage(c, http.StatusGone, titleInactive, messageInactive, h.frontendURL)
		return
	case err != nil:
		logger.Error(c.Request.Context(), "redirect: failed to resolve link",
			zap.String("code", code),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve link"})
		return
	}

	// Capture request data now: gin reuses the context after the handler returns.
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	event := service.ClickEvent{
		LinkID:      res.LinkID,
		ClickedAt:   time.Now().UTC(),
		IPHash:      h.hashIP(clientIP),
		IPAddress:   clientIP,
		UserAgent:   userAgent,
		Referrer:    c.GetHeader("Referer"),
		Source:      clickSourceFromQuery(c.Query("r")),
		UTMSource:   c.Query("utm_source"),
		UTMMedium:   c.Query("utm_medium"),
		UTMCampaign: c.Query("utm_campaign"),
	}

	target := res.LongURL
	// Only links with dynamic routing rules pay for synchronous UA/GeoIP parsing.
	if len(res.Rules) > 0 {
		ua := util.ParseUserAgent(userAgent)
		geo := util.LookupIP(c.Request.Context(), clientIP)
		if url, ok := h.routing.Pick(res.Rules, service.RoutingRequest{
			DeviceType: ua.DeviceType,
			OS:         ua.OS,
			Country:    geo.Country,
		}); ok {
			target = url
		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.clicks.RecordClick(ctx, event); err != nil {
			logger.Warn(ctx, "failed to record click",
				zap.Uint64("link_id", event.LinkID),
				zap.Error(err),
			)
		}
	}()

	c.Redirect(http.StatusFound, target)
}

// PageHop serves GET /m/:slug : a 302 to the frontend's page route. There is
// no lookup (the frontend 404s unknown slugs); a malformed or reserved slug
// gets the HTML 404 page.
func (h *RedirectHandler) PageHop(c *gin.Context) {
	slug, ok := service.ValidPageSlug(c.Param("slug"))
	if !ok {
		renderErrorPage(c, http.StatusNotFound, titlePageNotFound, messagePageNotFound, h.frontendURL)
		return
	}
	c.Redirect(http.StatusFound, strings.TrimRight(h.frontendURL, "/")+"/m/"+slug)
}

// Root serves GET / : a verified custom domain with root_redirect gets a 302
// there, anything else the "Link not found" page.
func (h *RedirectHandler) Root(c *gin.Context) {
	// Same normalisation as RedirectService.Resolve: domains are stored
	// lowercased without port or trailing dot.
	host := service.NormalizeRequestHost(c.Request.Host)
	if h.reserved[host] {
		renderErrorPage(c, http.StatusNotFound, titleNotFound, messageNotFound, h.frontendURL)
		return
	}
	domain, err := h.domainRepo.GetByDomain(c.Request.Context(), host)
	if err != nil && !errors.Is(err, repository.ErrDomainNotFound) {
		logger.Warn(c.Request.Context(), "redirect: failed to look up domain for root",
			zap.String("host", host),
			zap.Error(err),
		)
	}
	if err == nil && domain.VerificationStatus == model.DomainVerificationOK &&
		domain.RootRedirect != nil && *domain.RootRedirect != "" {
		c.Redirect(http.StatusFound, *domain.RootRedirect)
		return
	}
	renderErrorPage(c, http.StatusNotFound, titleNotFound, messageNotFound, h.frontendURL)
}
