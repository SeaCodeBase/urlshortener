package service

import (
	"context"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/util"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

// Compile-time check: PagePublicServiceImpl implements PagePublicService
var _ PagePublicService = (*PagePublicServiceImpl)(nil)

// page_views column widths (characters).
const (
	viewReferrerMax = 512
	viewCountryMax  = 2
	viewCityMax     = 100
	viewDeviceMax   = 20
	viewBrowserMax  = 50
)

// pageStatsFacets are the page view breakdowns of Stats.
var pageStatsFacets = []string{"referrer", "device", "country", "city"}

// ViewInput is the request data of a public page view.
type ViewInput struct {
	IP        string
	UserAgent string
	Referrer  string
}

// PageBlockStat is a link block's page-sourced clicks in the stats window.
type PageBlockStat struct {
	BlockID  uint64 `json:"block_id"`
	Title    string `json:"title"`
	LinkID   uint64 `json:"link_id"`
	ShortURL string `json:"short_url"`
	Clicks   int64  `json:"clicks"`
}

// PageStats is a page's analytics for one window. Prev* fields are filled
// only when the query asks for a comparison; ChangePct is nil without a
// comparison or when the previous total is 0.
type PageStats struct {
	From              string          `json:"from"`
	To                string          `json:"to"`
	Unit              string          `json:"unit"`
	TotalViewsAllTime int64           `json:"total_views_all_time"`
	TotalViews        int64           `json:"total_views"`
	PrevTotalViews    int64           `json:"prev_total_views"`
	ChangePct         *float64        `json:"change_pct"`
	Series            []SeriesPoint   `json:"series"`
	PrevSeries        []SeriesPoint   `json:"prev_series"`
	Referrers         []ReferrerStat  `json:"referrers"`
	Devices           []KeyStat       `json:"devices"`
	Countries         []CountryStat   `json:"countries"`
	Cities            []CityStat      `json:"cities"`
	Blocks            []PageBlockStat `json:"blocks"`
}

type PagePublicServiceImpl struct {
	pages  repository.PageRepository
	views  repository.PageViewRepository
	clicks repository.ClickRepository
	// linker builds link summaries (short URLs) exactly like PageService.
	linker *PageServiceImpl
	ipSalt string
	now    func() time.Time
}

// NewPagePublicService builds the service. linkRepo, domainRepo and baseURL
// build the stats blocks' short URLs like PageService; ipSalt is the click
// IP salt (util.HashIP).
func NewPagePublicService(pageRepo repository.PageRepository, viewRepo repository.PageViewRepository,
	clickRepo repository.ClickRepository, linkRepo repository.LinkRepository,
	domainRepo repository.DomainRepository, baseURL, ipSalt string) *PagePublicServiceImpl {
	return &PagePublicServiceImpl{
		pages:  pageRepo,
		views:  viewRepo,
		clicks: clickRepo,
		linker: NewPageService(PageServiceDeps{Pages: pageRepo, Views: viewRepo, Links: linkRepo,
			Domains: domainRepo, BaseURL: baseURL}),
		ipSalt: ipSalt,
		now:    time.Now,
	}
}

// SetClock replaces the clock used for default stats windows (tests).
func (s *PagePublicServiceImpl) SetClock(now func() time.Time) { s.now = now }

// publishedPage loads the page served at slug; ErrPageNotFound unless it is
// published, not archived and has a snapshot.
func (s *PagePublicServiceImpl) publishedPage(ctx context.Context, slug string) (*model.Page, error) {
	slug, reason := normalizeSlug(slug)
	if reason == SlugReasonInvalid {
		return nil, repository.ErrPageNotFound
	}
	p, err := s.pages.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if p.Archived || p.Status != model.PageStatusPublished || p.PublishedContent == nil {
		return nil, repository.ErrPageNotFound
	}
	return p, nil
}

func (s *PagePublicServiceImpl) GetPublished(ctx context.Context, slug string) (map[string]any, error) {
	p, err := s.publishedPage(ctx, slug)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(p.PublishedContent)+1)
	for k, v := range p.PublishedContent {
		out[k] = v
	}
	out["slug"] = p.Slug
	return out, nil
}

func (s *PagePublicServiceImpl) RecordView(ctx context.Context, slug string, v ViewInput) error {
	p, err := s.publishedPage(ctx, slug)
	if err != nil {
		return err
	}
	ua := util.ParseUserAgent(v.UserAgent)
	geo := safeLookupIP(ctx, v.IP)
	view := &model.PageView{
		PageID:     p.ID,
		Referrer:   truncateRunes(v.Referrer, viewReferrerMax),
		Country:    truncateRunes(geo.Country, viewCountryMax),
		City:       truncateRunes(geo.City, viewCityMax),
		DeviceType: truncateRunes(ua.DeviceType, viewDeviceMax),
		Browser:    truncateRunes(ua.Browser, viewBrowserMax),
	}
	if v.IP != "" {
		view.IPHash = util.HashIP(s.ipSalt, v.IP)
	}
	if err := s.views.Insert(ctx, view); err != nil {
		logger.Error(ctx, "page-public-service: failed to record view",
			zap.Uint64("page_id", p.ID),
			zap.Error(err),
		)
		return err
	}
	return nil
}

// safeLookupIP resolves the IP's location; any failure (even a panic in the
// GeoIP reader) yields an empty result.
func safeLookupIP(ctx context.Context, ip string) (res util.GeoIPResult) {
	if ip == "" {
		return res
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Warn(ctx, "page-public-service: geoip lookup panicked", zap.Any("panic", r))
			res = util.GeoIPResult{}
		}
	}()
	return util.LookupIP(ctx, ip)
}

func (s *PagePublicServiceImpl) Stats(ctx context.Context, groupID, pageID uint64, query StatsQuery) (*PageStats, error) {
	q, err := NormalizeStatsQuery(query, s.now())
	if err != nil {
		return nil, err
	}
	p, err := s.linker.getPage(ctx, groupID, pageID)
	if err != nil {
		return nil, err
	}

	cur := window{q.From, q.To}
	days := rangeDays(q.From, q.To)
	prev := window{q.From.AddDate(0, 0, -days), q.From.AddDate(0, 0, -1)}

	out := &PageStats{
		From:       q.From.Format("2006-01-02"),
		To:         q.To.Format("2006-01-02"),
		Unit:       q.Unit,
		PrevSeries: []SeriesPoint{},
	}
	fail := func(what string, err error) (*PageStats, error) {
		logger.Error(ctx, "page-public-service: failed to get "+what,
			zap.Uint64("page_id", p.ID),
			zap.Error(err),
		)
		return nil, err
	}

	if out.TotalViewsAllTime, err = s.views.CountAllTime(ctx, p.ID); err != nil {
		return fail("total views", err)
	}
	if out.TotalViews, err = s.views.CountRange(ctx, p.ID, cur.from, cur.to); err != nil {
		return fail("range views", err)
	}
	series, err := s.views.Series(ctx, p.ID, cur.from, cur.to, q.Unit)
	if err != nil {
		return fail("series", err)
	}
	out.Series = toSeries(series)

	if q.Compare {
		if out.PrevTotalViews, err = s.views.CountRange(ctx, p.ID, prev.from, prev.to); err != nil {
			return fail("previous range views", err)
		}
		out.ChangePct = changePct(out.TotalViews, out.PrevTotalViews)
		prevSeries, err := s.views.Series(ctx, p.ID, prev.from, prev.to, q.Unit)
		if err != nil {
			return fail("previous series", err)
		}
		out.PrevSeries = toSeries(prevSeries)
	}

	fetch := func(from, to time.Time, facet string) ([]repository.Row, error) {
		return s.views.Breakdown(ctx, p.ID, from, to, facet, breakdownLimit)
	}
	bd, facet, err := loadBreakdowns(fetch, pageStatsFacets, cur, prev, q.Compare, out.TotalViews)
	if err != nil {
		return fail(facet+" breakdown", err)
	}
	out.Referrers = bd["referrer"].referrers()
	out.Devices = bd["device"].keys()
	out.Countries = bd["country"].countries()
	out.Cities = bd["city"].cities()

	if out.Blocks, err = s.blockStats(ctx, p.ID, cur); err != nil {
		return fail("block clicks", err)
	}
	return out, nil
}

// blockStats lists every link block of the page (active or not) with its
// link's page-sourced clicks in the window.
func (s *PagePublicServiceImpl) blockStats(ctx context.Context, pageID uint64, cur window) ([]PageBlockStat, error) {
	blocks, err := s.pages.ListBlocks(ctx, pageID)
	if err != nil {
		return nil, err
	}
	linkBlocks := make([]model.PageBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.Type == model.PageBlockLink && b.LinkID != nil {
			linkBlocks = append(linkBlocks, b)
		}
	}
	out := make([]PageBlockStat, 0, len(linkBlocks))
	if len(linkBlocks) == 0 {
		return out, nil
	}
	links, err := s.linker.blockLinks(ctx, linkBlocks)
	if err != nil {
		return nil, err
	}
	for _, b := range linkBlocks {
		clicks, err := s.clicks.CountRange(ctx, []uint64{*b.LinkID}, cur.from, cur.to, model.LinkSourcePage)
		if err != nil {
			return nil, err
		}
		title, _ := b.Content["title"].(string)
		out = append(out, PageBlockStat{
			BlockID:  b.ID,
			Title:    strings.TrimSpace(title),
			LinkID:   *b.LinkID,
			ShortURL: links[*b.LinkID].ShortURL, // "" when the link vanished meanwhile
			Clicks:   clicks,
		})
	}
	return out, nil
}
