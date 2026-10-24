package service

import (
	"context"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

// Compile-time check: StatsServiceImpl implements StatsService
var _ StatsService = (*StatsServiceImpl)(nil)

// breakdownLimit caps the rows of every breakdown module.
const breakdownLimit = 50

// maxRangeDays and maxHourlyDays bound a stats query's inclusive day count.
const (
	maxRangeDays  = 366
	maxHourlyDays = 2
)

type StatsServiceImpl struct {
	clickRepo repository.ClickRepository
	linkRepo  repository.LinkRepository
	now       func() time.Time
}

func NewStatsService(clickRepo repository.ClickRepository, linkRepo repository.LinkRepository) *StatsServiceImpl {
	return &StatsServiceImpl{
		clickRepo: clickRepo,
		linkRepo:  linkRepo,
		now:       time.Now,
	}
}

// SetClock replaces the clock used for default date ranges (tests).
func (s *StatsServiceImpl) SetClock(now func() time.Time) { s.now = now }

// StatsQuery selects the window and filters of a stats request. From and To
// are inclusive UTC dates; zero values default to the last 30 days ending
// today. Unit is "day" or "hour"; Source is ""/"all", "link", "qr" or "page".
type StatsQuery struct {
	From    time.Time
	To      time.Time
	Unit    string
	Compare bool
	Source  string
}

// StatsQueryError reports an invalid stats query; its message is user-facing.
type StatsQueryError struct{ Msg string }

func (e *StatsQueryError) Error() string { return e.Msg }

func dateOnly(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// rangeDays is the inclusive number of days between two UTC dates.
func rangeDays(from, to time.Time) int {
	return int(to.Sub(from).Hours()/24) + 1
}

// NormalizeStatsQuery applies defaults (to = today, from = to-29d, unit day,
// source all → "") and validates the result against now.
func NormalizeStatsQuery(q StatsQuery, now time.Time) (StatsQuery, error) {
	if q.To.IsZero() {
		q.To = now
	}
	q.To = dateOnly(q.To)
	if q.From.IsZero() {
		q.From = q.To.AddDate(0, 0, -29)
	}
	q.From = dateOnly(q.From)
	if q.Unit == "" {
		q.Unit = "day"
	}
	if q.Unit != "day" && q.Unit != "hour" {
		return q, &StatsQueryError{Msg: "unit must be day or hour"}
	}
	switch q.Source {
	case "", "all":
		q.Source = ""
	case "link", "qr", "page":
	default:
		return q, &StatsQueryError{Msg: "source must be all, link, qr or page"}
	}
	if q.From.After(q.To) {
		return q, &StatsQueryError{Msg: "from must not be after to"}
	}
	days := rangeDays(q.From, q.To)
	if days > maxRangeDays {
		return q, &StatsQueryError{Msg: "date range is limited to 366 days"}
	}
	if q.Unit == "hour" && days > maxHourlyDays {
		return q, &StatsQueryError{Msg: "hourly data is limited to 2 days"}
	}
	return q, nil
}

// SeriesPoint is one bucket of an engagements-over-time series.
type SeriesPoint struct {
	Date   string `json:"date"`
	Clicks int64  `json:"clicks"`
}

type CountryStat struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Clicks     int64   `json:"clicks"`
	Percentage float64 `json:"percentage"`
	PrevClicks int64   `json:"prev_clicks"`
}

type CityStat struct {
	Name       string  `json:"name"`
	Country    string  `json:"country"`
	Clicks     int64   `json:"clicks"`
	Percentage float64 `json:"percentage"`
	PrevClicks int64   `json:"prev_clicks"`
}

type ReferrerStat struct {
	Referrer   string  `json:"referrer"`
	Clicks     int64   `json:"clicks"`
	Percentage float64 `json:"percentage"`
	PrevClicks int64   `json:"prev_clicks"`
}

// KeyStat is a device, browser or OS breakdown row.
type KeyStat struct {
	Key        string  `json:"key"`
	Clicks     int64   `json:"clicks"`
	Percentage float64 `json:"percentage"`
	PrevClicks int64   `json:"prev_clicks"`
}

// LinkStatsV2 is a link's analytics for one window. Prev* fields and
// prev_clicks are filled only when the query asks for a comparison;
// ChangePct is nil when there is no comparison or the previous total is 0.
type LinkStatsV2 struct {
	From           string         `json:"from"`
	To             string         `json:"to"`
	Unit           string         `json:"unit"`
	TotalAllTime   int64          `json:"total_all_time"`
	TotalRange     int64          `json:"total_range"`
	PrevTotalRange int64          `json:"prev_total_range"`
	ChangePct      *float64       `json:"change_pct"`
	Series         []SeriesPoint  `json:"series"`
	PrevSeries     []SeriesPoint  `json:"prev_series"`
	Countries      []CountryStat  `json:"countries"`
	Cities         []CityStat     `json:"cities"`
	Referrers      []ReferrerStat `json:"referrers"`
	Devices        []KeyStat      `json:"devices"`
	Browsers       []KeyStat      `json:"browsers"`
	OS             []KeyStat      `json:"os"`
}

// window is an inclusive UTC date range.
type window struct{ from, to time.Time }

func (s *StatsServiceImpl) GetLinkStats(ctx context.Context, userID, linkID uint64, query StatsQuery) (*LinkStatsV2, error) {
	q, err := NormalizeStatsQuery(query, s.now())
	if err != nil {
		return nil, err
	}

	link, err := s.linkRepo.GetByID(ctx, linkID)
	if err != nil {
		logger.Error(ctx, "stats-service: failed to get link",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	if link.UserID != userID {
		return nil, ErrNotLinkOwner
	}

	ids := []uint64{linkID}
	cur := window{q.From, q.To}
	// The previous window has the same length and ends the day before from.
	days := rangeDays(q.From, q.To)
	prev := window{q.From.AddDate(0, 0, -days), q.From.AddDate(0, 0, -1)}

	out := &LinkStatsV2{
		From:       q.From.Format("2006-01-02"),
		To:         q.To.Format("2006-01-02"),
		Unit:       q.Unit,
		PrevSeries: []SeriesPoint{},
	}

	fail := func(what string, err error) (*LinkStatsV2, error) {
		logger.Error(ctx, "stats-service: failed to get "+what,
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}

	if out.TotalAllTime, err = s.clickRepo.TotalAllTime(ctx, ids, q.Source); err != nil {
		return fail("total", err)
	}
	if out.TotalRange, err = s.clickRepo.CountRange(ctx, ids, cur.from, cur.to, q.Source); err != nil {
		return fail("range total", err)
	}
	series, err := s.clickRepo.Series(ctx, ids, cur.from, cur.to, q.Unit, q.Source)
	if err != nil {
		return fail("series", err)
	}
	out.Series = toSeries(series)

	if q.Compare {
		if out.PrevTotalRange, err = s.clickRepo.CountRange(ctx, ids, prev.from, prev.to, q.Source); err != nil {
			return fail("previous range total", err)
		}
		if out.PrevTotalRange > 0 {
			pct := float64(out.TotalRange-out.PrevTotalRange) / float64(out.PrevTotalRange) * 100
			out.ChangePct = &pct
		}
		prevSeries, err := s.clickRepo.Series(ctx, ids, prev.from, prev.to, q.Unit, q.Source)
		if err != nil {
			return fail("previous series", err)
		}
		out.PrevSeries = toSeries(prevSeries)
	}

	// breakdown loads a facet for the current window and, when comparing,
	// the previous window's counts keyed by Key+"\x00"+Key2.
	breakdown := func(facet string) ([]repository.Row, map[string]int64, error) {
		rows, err := s.clickRepo.Breakdown(ctx, ids, cur.from, cur.to, facet, breakdownLimit, q.Source)
		if err != nil {
			return nil, nil, err
		}
		prevCounts := map[string]int64{}
		if q.Compare {
			prevRows, err := s.clickRepo.Breakdown(ctx, ids, prev.from, prev.to, facet, breakdownLimit, q.Source)
			if err != nil {
				return nil, nil, err
			}
			for _, r := range prevRows {
				prevCounts[rowKey(r)] = r.Count
			}
		}
		return rows, prevCounts, nil
	}
	pctOf := func(n int64) float64 {
		if out.TotalRange == 0 {
			return 0
		}
		return float64(n) / float64(out.TotalRange) * 100
	}

	rows, prevCounts, err := breakdown("country")
	if err != nil {
		return fail("country breakdown", err)
	}
	out.Countries = make([]CountryStat, 0, len(rows))
	for _, r := range rows {
		out.Countries = append(out.Countries, CountryStat{Code: r.Key, Name: getCountryName(r.Key), Clicks: r.Count,
			Percentage: pctOf(r.Count), PrevClicks: prevCounts[rowKey(r)]})
	}

	if rows, prevCounts, err = breakdown("city"); err != nil {
		return fail("city breakdown", err)
	}
	out.Cities = make([]CityStat, 0, len(rows))
	for _, r := range rows {
		out.Cities = append(out.Cities, CityStat{Name: r.Key, Country: r.Key2, Clicks: r.Count,
			Percentage: pctOf(r.Count), PrevClicks: prevCounts[rowKey(r)]})
	}

	if rows, prevCounts, err = breakdown("referrer"); err != nil {
		return fail("referrer breakdown", err)
	}
	out.Referrers = make([]ReferrerStat, 0, len(rows))
	for _, r := range rows {
		out.Referrers = append(out.Referrers, ReferrerStat{Referrer: r.Key, Clicks: r.Count,
			Percentage: pctOf(r.Count), PrevClicks: prevCounts[rowKey(r)]})
	}

	for _, kf := range []struct {
		facet string
		dst   *[]KeyStat
	}{{"device", &out.Devices}, {"browser", &out.Browsers}, {"os", &out.OS}} {
		rows, prevCounts, err := breakdown(kf.facet)
		if err != nil {
			return fail(kf.facet+" breakdown", err)
		}
		stats := make([]KeyStat, 0, len(rows))
		for _, r := range rows {
			stats = append(stats, KeyStat{Key: r.Key, Clicks: r.Count, Percentage: pctOf(r.Count), PrevClicks: prevCounts[rowKey(r)]})
		}
		*kf.dst = stats
	}

	return out, nil
}

// GroupEngagements counts clicks from every source on the group's links in
// the inclusive UTC date range.
func (s *StatsServiceImpl) GroupEngagements(ctx context.Context, groupID uint64, from, to time.Time) (int64, error) {
	n, err := s.clickRepo.CountRangeByGroup(ctx, groupID, from, to, "")
	if err != nil {
		logger.Error(ctx, "stats-service: failed to count group engagements",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return 0, err
	}
	return n, nil
}

func rowKey(r repository.Row) string { return r.Key + "\x00" + r.Key2 }

func toSeries(buckets []repository.Bucket) []SeriesPoint {
	out := make([]SeriesPoint, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, SeriesPoint{Date: b.Date, Clicks: b.Count})
	}
	return out
}

func getCountryName(code string) string {
	names := map[string]string{
		"CN": "China", "US": "United States", "JP": "Japan", "GB": "United Kingdom",
		"DE": "Germany", "FR": "France", "KR": "South Korea", "IN": "India",
		"BR": "Brazil", "RU": "Russia", "CA": "Canada", "AU": "Australia",
		"ES": "Spain", "IT": "Italy", "MX": "Mexico", "NL": "Netherlands",
		"XX": "Local", "Unknown": "Unknown",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}
