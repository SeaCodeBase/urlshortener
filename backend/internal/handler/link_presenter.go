package handler

import (
	"context"
	"net/url"
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

const faviconServiceFormat = "https://www.google.com/s2/favicons?domain=%s&sz=32"

// linkPresenter renders links (short URL, favicon, domain names) for the
// link and QR code handlers.
type linkPresenter struct {
	baseURL    string
	favicons   bool
	domainRepo repository.DomainRepository
}

func newLinkPresenter(cfg *config.Config, domainRepo repository.DomainRepository) linkPresenter {
	return linkPresenter{
		baseURL:    cfg.URLs.BaseURL,
		favicons:   cfg.UI.FaviconService == nil || *cfg.UI.FaviconService,
		domainRepo: domainRepo,
	}
}

func (p *linkPresenter) buildShortURL(link *model.Link, domainMap map[uint64]string) string {
	domainName := ""
	if link.DomainID != nil {
		domainName = domainMap[*link.DomainID]
	}
	return service.ShortURL(p.baseURL, link, domainName)
}

// faviconURL returns the Google favicon URL for the destination host, or ""
// when the favicon service is disabled.
func (p *linkPresenter) faviconURL(link *model.Link) string {
	if !p.favicons {
		return ""
	}
	host := ""
	if link.LongURLHost != nil {
		host = *link.LongURLHost
	} else if u, err := url.Parse(link.OriginalURL); err == nil {
		host = strings.ToLower(u.Hostname())
	}
	if host == "" {
		return ""
	}
	return strings.Replace(faviconServiceFormat, "%s", url.QueryEscape(host), 1)
}

func (p *linkPresenter) toResponse(link *model.Link, tags []string, clicks int64, domainMap map[uint64]string) linkResponse {
	if tags == nil {
		tags = []string{}
	}
	return linkResponse{
		Link:        link,
		ShortURL:    p.buildShortURL(link, domainMap),
		Tags:        tags,
		TotalClicks: clicks,
		FaviconURL:  p.faviconURL(link),
	}
}

func (p *linkPresenter) loadDomainMap(ctx context.Context, userID uint64) map[uint64]string {
	domains, err := p.domainRepo.ListByUserID(ctx, userID)
	if err != nil {
		logger.Warn(ctx, "link-handler: failed to load domains for user",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return make(map[uint64]string)
	}
	domainMap := make(map[uint64]string, len(domains))
	for _, d := range domains {
		domainMap[d.ID] = d.Domain
	}
	return domainMap
}
