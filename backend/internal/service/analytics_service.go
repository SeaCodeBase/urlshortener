package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

// Compile-time check: AnalyticsServiceImpl implements AnalyticsService
var _ AnalyticsService = (*AnalyticsServiceImpl)(nil)

const (
	// dashboardLimit caps dashboard breakdowns and item lists.
	dashboardLimit = 200
	// itemsFetchLimit is how many (link, source) rows are read before they
	// are merged into items and cut to the item limit.
	itemsFetchLimit = 1000
	// topPerformingLimit and topPerformingDays define the overview's list.
	topPerformingLimit = 5
	topPerformingDays  = 30
)

// Dashboard filter types and item types.
const (
	FilterTypeAll   = "all"
	FilterTypeLinks = "links"
	FilterTypeQR    = "qr"

	ItemTypeLink = "link"
	ItemTypeQR   = "qr"
)

// DashboardFilter selects what a dashboard shows: every link and QR code of
// the group ("all"), the given links ("links") or QR codes ("qr").
type DashboardFilter struct {
	Type string   `json:"type"`
	IDs  []uint64 `json:"ids,omitempty"`
}

// AnalyticsItem is a link or QR code with its engagements in a window.
type AnalyticsItem struct {
	Type        string  `json:"type"`
	ID          uint64  `json:"id"`
	LinkID      uint64  `json:"link_id"`
	Title       string  `json:"title"`
	ShortURL    string  `json:"short_url"`
	OriginalURL string  `json:"original_url"`
	Clicks      int64   `json:"clicks"`
	Percentage  float64 `json:"percentage"`
	PrevClicks  int64   `json:"prev_clicks"`
}

// TopDay is the series bucket with the most engagements.
type TopDay struct {
	Date      string   `json:"date"`
	Count     int64    `json:"count"`
	PrevCount int64    `json:"prev_count"`
	ChangePct *float64 `json:"change_pct"`
}

// TopLocation is the country with the most engagements.
type TopLocation struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// DashboardData is a dashboard's module data for one window.
type DashboardData struct {
	From        string          `json:"from"`
	To          string          `json:"to"`
	Unit        string          `json:"unit"`
	Compare     bool            `json:"compare"`
	Total       int64           `json:"total"`
	PrevTotal   int64           `json:"prev_total"`
	ChangePct   *float64        `json:"change_pct"`
	TopDay      *TopDay         `json:"top_day"`
	TopLocation *TopLocation    `json:"top_location"`
	Series      []SeriesPoint   `json:"series"`
	PrevSeries  []SeriesPoint   `json:"prev_series"`
	Devices     []KeyStat       `json:"devices"`
	Referrers   []ReferrerStat  `json:"referrers"`
	Countries   []CountryStat   `json:"countries"`
	Cities      []CityStat      `json:"cities"`
	Items       []AnalyticsItem `json:"items"`
}

// OverviewWeek is one 7-day window of the overview.
type OverviewWeek struct {
	From   string        `json:"from"`
	To     string        `json:"to"`
	Total  int64         `json:"total"`
	Series []SeriesPoint `json:"series"`
}

// Overview is the analytics landing page data.
type Overview struct {
	ThisWeek      OverviewWeek    `json:"this_week"`
	LastWeek      OverviewWeek    `json:"last_week"`
	ChangePct     *float64        `json:"change_pct"`
	TopPerforming []AnalyticsItem `json:"top_performing"`
}

type AnalyticsServiceImpl struct {
	clickRepo  repository.ClickRepository
	linkRepo   repository.LinkRepository
	qrRepo     repository.QRRepository
	domainRepo repository.DomainRepository
	baseURL    string
	now        func() time.Time
}

// NewAnalyticsService builds the service; domainRepo and baseURL (config
// urls.base_url) build item short URLs like the link handlers do.
func NewAnalyticsService(clickRepo repository.ClickRepository, linkRepo repository.LinkRepository,
	qrRepo repository.QRRepository, domainRepo repository.DomainRepository, baseURL string) *AnalyticsServiceImpl {
	return &AnalyticsServiceImpl{
		clickRepo:  clickRepo,
		linkRepo:   linkRepo,
		qrRepo:     qrRepo,
		domainRepo: domainRepo,
		baseURL:    strings.TrimRight(baseURL, "/"),
		now:        time.Now,
	}
}

// SetClock replaces the clock used for default date ranges (tests).
func (s *AnalyticsServiceImpl) SetClock(now func() time.Time) { s.now = now }

const dateFmt = "2006-01-02"

// scopeFor maps a dashboard filter to the clicks it covers.
func (s *AnalyticsServiceImpl) scopeFor(ctx context.Context, groupID uint64, f DashboardFilter) (repository.ClickScope, error) {
	switch f.Type {
	case "", FilterTypeAll:
		return repository.ClickScope{GroupID: groupID}, nil
	case FilterTypeLinks:
		ids := append(make([]uint64, 0, len(f.IDs)), f.IDs...)
		return repository.ClickScope{GroupID: groupID, LinkIDs: ids, Source: "not_qr"}, nil
	case FilterTypeQR:
		linkIDs := make([]uint64, 0, len(f.IDs))
		if len(f.IDs) > 0 {
			rows, err := s.qrRepo.GetByIDs(ctx, f.IDs)
			if err != nil {
				return repository.ClickScope{}, err
			}
			seen := map[uint64]bool{}
			for _, r := range rows {
				if r.GroupID != groupID || seen[r.LinkID] {
					continue
				}
				seen[r.LinkID] = true
				linkIDs = append(linkIDs, r.LinkID)
			}
		}
		return repository.ClickScope{GroupID: groupID, LinkIDs: linkIDs, Source: "qr"}, nil
	default:
		return repository.ClickScope{}, fmt.Errorf("analytics: unknown filter type %q", f.Type)
	}
}

func (s *AnalyticsServiceImpl) Overview(ctx context.Context, groupID uint64) (*Overview, error) {
	today := dateOnly(s.now())
	scope := repository.ClickScope{GroupID: groupID}
	week := func(from, to time.Time) (OverviewWeek, error) {
		total, err := s.clickRepo.CountScope(ctx, scope, from, to)
		if err != nil {
			return OverviewWeek{}, err
		}
		series, err := s.clickRepo.SeriesScope(ctx, scope, from, to, "day")
		if err != nil {
			return OverviewWeek{}, err
		}
		return OverviewWeek{From: from.Format(dateFmt), To: to.Format(dateFmt), Total: total, Series: toSeries(series)}, nil
	}
	fail := func(what string, err error) (*Overview, error) {
		logger.Error(ctx, "analytics-service: failed to get overview "+what,
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return nil, err
	}

	out := &Overview{}
	var err error
	if out.ThisWeek, err = week(today.AddDate(0, 0, -6), today); err != nil {
		return fail("this week", err)
	}
	if out.LastWeek, err = week(today.AddDate(0, 0, -13), today.AddDate(0, 0, -7)); err != nil {
		return fail("last week", err)
	}
	out.ChangePct = changePct(out.ThisWeek.Total, out.LastWeek.Total)

	month := window{today.AddDate(0, 0, -(topPerformingDays - 1)), today}
	total, err := s.clickRepo.CountScope(ctx, scope, month.from, month.to)
	if err != nil {
		return fail("top performing total", err)
	}
	if out.TopPerforming, err = s.items(ctx, scope, month, nil, total, topPerformingLimit); err != nil {
		return fail("top performing", err)
	}
	return out, nil
}

func (s *AnalyticsServiceImpl) DashboardData(ctx context.Context, groupID uint64, filter DashboardFilter, query StatsQuery) (*DashboardData, error) {
	query.Unit, query.Source = "", ""
	q, err := NormalizeStatsQuery(query, s.now())
	if err != nil {
		return nil, err
	}
	days := rangeDays(q.From, q.To)
	if days <= maxHourlyDays {
		q.Unit = "hour"
	}
	cur := window{q.From, q.To}
	// The previous window has the same length and ends the day before from.
	prev := window{q.From.AddDate(0, 0, -days), q.From.AddDate(0, 0, -1)}

	fail := func(what string, err error) (*DashboardData, error) {
		logger.Error(ctx, "analytics-service: failed to get "+what,
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return nil, err
	}

	scope, err := s.scopeFor(ctx, groupID, filter)
	if err != nil {
		return fail("scope", err)
	}

	out := &DashboardData{
		From:       q.From.Format(dateFmt),
		To:         q.To.Format(dateFmt),
		Unit:       q.Unit,
		Compare:    q.Compare,
		PrevSeries: []SeriesPoint{},
	}

	if out.Total, err = s.clickRepo.CountScope(ctx, scope, cur.from, cur.to); err != nil {
		return fail("total", err)
	}
	series, err := s.clickRepo.SeriesScope(ctx, scope, cur.from, cur.to, q.Unit)
	if err != nil {
		return fail("series", err)
	}
	out.Series = toSeries(series)
	if q.Compare {
		if out.PrevTotal, err = s.clickRepo.CountScope(ctx, scope, prev.from, prev.to); err != nil {
			return fail("previous total", err)
		}
		out.ChangePct = changePct(out.Total, out.PrevTotal)
		prevSeries, err := s.clickRepo.SeriesScope(ctx, scope, prev.from, prev.to, q.Unit)
		if err != nil {
			return fail("previous series", err)
		}
		out.PrevSeries = toSeries(prevSeries)
	}

	// Top day: the first bucket with the most clicks; the previous count is
	// the bucket at the same position of the previous window.
	if out.Total > 0 {
		best := -1
		for i, p := range out.Series {
			if best < 0 || p.Clicks > out.Series[best].Clicks {
				best = i
			}
		}
		if best >= 0 {
			td := &TopDay{Date: out.Series[best].Date, Count: out.Series[best].Clicks}
			if q.Compare && best < len(out.PrevSeries) {
				td.PrevCount = out.PrevSeries[best].Clicks
				td.ChangePct = changePct(td.Count, td.PrevCount)
			}
			out.TopDay = td
		}
	}

	fetch := func(from, to time.Time, facet string) ([]repository.Row, error) {
		return s.clickRepo.BreakdownScope(ctx, scope, from, to, facet, dashboardLimit)
	}
	bd, facet, err := loadBreakdowns(fetch, []string{"device", "referrer", "country", "city"},
		cur, prev, q.Compare, out.Total)
	if err != nil {
		return fail(facet+" breakdown", err)
	}
	out.Devices = bd["device"].keys()
	out.Referrers = bd["referrer"].referrers()
	out.Countries = bd["country"].countries()
	out.Cities = bd["city"].cities()
	if len(out.Countries) > 0 {
		c := out.Countries[0]
		out.TopLocation = &TopLocation{Code: c.Code, Name: c.Name, Count: c.Clicks}
	}

	var prevWin *window
	if q.Compare {
		prevWin = &prev
	}
	if out.Items, err = s.items(ctx, scope, cur, prevWin, out.Total, dashboardLimit); err != nil {
		return fail("items", err)
	}
	return out, nil
}

// itemKey identifies an item across windows.
type itemKey struct {
	typ string
	id  uint64
}

// items lists the scope's links and QR codes with clicks in cur, most first,
// at most limit; QR scans of a link owning a QR code count for the QR code,
// QR-owned links (source 'qr') are always their QR code, links that no
// longer exist are dropped. prev (optional) fills prev_clicks.
func (s *AnalyticsServiceImpl) items(ctx context.Context, scope repository.ClickScope, cur window, prev *window, total int64, limit int) ([]AnalyticsItem, error) {
	curRows, err := s.clickRepo.ItemsScope(ctx, scope, cur.from, cur.to, itemsFetchLimit)
	if err != nil {
		return nil, err
	}
	var prevRows []repository.ItemCount
	if prev != nil {
		if prevRows, err = s.clickRepo.ItemsScope(ctx, scope, prev.from, prev.to, itemsFetchLimit); err != nil {
			return nil, err
		}
	}
	out := []AnalyticsItem{}
	if len(curRows) == 0 {
		return out, nil
	}

	// One lookup for the links of both windows.
	seen := map[uint64]bool{}
	linkIDs := []uint64{}
	for _, rows := range [][]repository.ItemCount{curRows, prevRows} {
		for _, r := range rows {
			if !seen[r.LinkID] {
				seen[r.LinkID] = true
				linkIDs = append(linkIDs, r.LinkID)
			}
		}
	}
	links, err := s.linkRepo.GetByIDs(ctx, linkIDs)
	if err != nil {
		return nil, err
	}
	linkByID := make(map[uint64]*model.Link, len(links))
	for i := range links {
		linkByID[links[i].ID] = &links[i]
	}
	qrByLink, err := s.qrRepo.QRCodeIDsForLinks(ctx, linkIDs)
	if err != nil {
		return nil, err
	}

	// attribute maps a (link, source) row to its item; ok is false for rows
	// that belong to no item.
	attribute := func(r repository.ItemCount) (itemKey, *model.Link, bool) {
		link := linkByID[r.LinkID]
		if link == nil {
			return itemKey{}, nil, false
		}
		qrID, hasQR := qrByLink[r.LinkID]
		if link.Source == model.LinkSourceQR {
			if !hasQR {
				return itemKey{}, nil, false
			}
			return itemKey{ItemTypeQR, qrID}, link, true
		}
		if r.Source == model.LinkSourceQR && hasQR {
			return itemKey{ItemTypeQR, qrID}, link, true
		}
		return itemKey{ItemTypeLink, link.ID}, link, true
	}

	prevClicks := map[itemKey]int64{}
	for _, r := range prevRows {
		if k, _, ok := attribute(r); ok {
			prevClicks[k] += r.Count
		}
	}

	byKey := map[itemKey]int{}
	for _, r := range curRows {
		if r.Count == 0 {
			continue
		}
		k, link, ok := attribute(r)
		if !ok {
			continue
		}
		if i, ok := byKey[k]; ok {
			out[i].Clicks += r.Count
			continue
		}
		byKey[k] = len(out)
		out = append(out, AnalyticsItem{Type: k.typ, ID: k.id, LinkID: link.ID, Clicks: r.Count})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Clicks > out[j].Clicks })
	if len(out) > limit {
		out = out[:limit]
	}

	qrTitles, err := s.qrTitles(ctx, out)
	if err != nil {
		return nil, err
	}
	domains := map[uint64]string{}
	for i := range out {
		it := &out[i]
		link := linkByID[it.LinkID]
		// A QR item shows its own title, falling back to its link's.
		if it.Type == ItemTypeQR {
			it.Title = qrTitles[it.ID]
		}
		if it.Title == "" && link.Title != nil {
			it.Title = *link.Title
		}
		it.ShortURL = ShortURL(s.baseURL, link, s.domainName(ctx, link, domains))
		it.OriginalURL = link.OriginalURL
		it.Percentage = percentOf(it.Clicks, total)
		it.PrevClicks = prevClicks[itemKey{it.Type, it.ID}]
	}
	return out, nil
}

// qrTitles returns the set titles of the QR items, keyed by QR code ID.
func (s *AnalyticsServiceImpl) qrTitles(ctx context.Context, items []AnalyticsItem) (map[uint64]string, error) {
	ids := []uint64{}
	for _, it := range items {
		if it.Type == ItemTypeQR {
			ids = append(ids, it.ID)
		}
	}
	titles := map[uint64]string{}
	if len(ids) == 0 {
		return titles, nil
	}
	rows, err := s.qrRepo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.Title != nil {
			titles[r.ID] = *r.Title
		}
	}
	return titles, nil
}

// domainName resolves the link's custom domain, caching lookups in cache;
// a failed lookup falls back to the base URL.
func (s *AnalyticsServiceImpl) domainName(ctx context.Context, link *model.Link, cache map[uint64]string) string {
	if link.DomainID == nil {
		return ""
	}
	name, ok := cache[*link.DomainID]
	if !ok {
		d, err := s.domainRepo.GetByID(ctx, *link.DomainID)
		if err != nil {
			logger.Warn(ctx, "analytics-service: failed to load link domain",
				zap.Uint64("domain_id", *link.DomainID),
				zap.Error(err),
			)
		} else {
			name = d.Domain
		}
		cache[*link.DomainID] = name
	}
	return name
}
