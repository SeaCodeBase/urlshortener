package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
)

// ExportZip returns a zip of one CSV per dashboard module for the window.
func (s *AnalyticsServiceImpl) ExportZip(ctx context.Context, groupID uint64, name string, filter DashboardFilter, q StatsQuery) ([]byte, error) {
	d, err := s.DashboardData(ctx, groupID, filter, q)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range exportFiles(name, d) {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, err
		}
		cw := csv.NewWriter(w)
		if err := cw.WriteAll(f.rows); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type exportFile struct {
	name string
	rows [][]string
}

func fmtInt(n int64) string   { return strconv.FormatInt(n, 10) }
func fmtPct(p float64) string { return strconv.FormatFloat(p, 'f', 1, 64) }
func fmtPctPtr(p *float64) string {
	if p == nil {
		return ""
	}
	return fmtPct(*p)
}

// exportFiles lays out the seven module CSVs; every string cell goes
// through csvSafe, numbers are written as-is.
func exportFiles(name string, d *DashboardData) []exportFile {
	summary := [][]string{
		{"Metric", "Value"},
		{"Dashboard", csvSafe(name)},
		{"Range", csvSafe(d.From + " to " + d.To)},
		{"Total", fmtInt(d.Total)},
		{"Previous total", fmtInt(d.PrevTotal)},
		{"Change %", fmtPctPtr(d.ChangePct)},
	}

	overTime := [][]string{{"Date", "Engagements", "Previous period"}}
	for i, p := range d.Series {
		var prev int64
		if i < len(d.PrevSeries) {
			prev = d.PrevSeries[i].Clicks
		}
		overTime = append(overTime, []string{csvSafe(p.Date), fmtInt(p.Clicks), fmtInt(prev)})
	}

	devices := [][]string{{"Device", "Engagements", "Percentage", "Previous engagements"}}
	for _, r := range d.Devices {
		devices = append(devices, []string{csvSafe(r.Key), fmtInt(r.Clicks), fmtPct(r.Percentage), fmtInt(r.PrevClicks)})
	}

	referrers := [][]string{{"Referrer", "Engagements", "Percentage", "Previous engagements"}}
	for _, r := range d.Referrers {
		referrers = append(referrers, []string{csvSafe(r.Referrer), fmtInt(r.Clicks), fmtPct(r.Percentage), fmtInt(r.PrevClicks)})
	}

	locations := [][]string{{"Country code", "Country", "Engagements", "Percentage", "Previous engagements"}}
	for _, r := range d.Countries {
		locations = append(locations, []string{csvSafe(r.Code), csvSafe(r.Name), fmtInt(r.Clicks), fmtPct(r.Percentage), fmtInt(r.PrevClicks)})
	}
	locations = append(locations, []string{},
		[]string{"City", "Country", "Engagements", "Percentage", "Previous engagements"})
	for _, r := range d.Cities {
		locations = append(locations, []string{csvSafe(r.Name), csvSafe(r.Country), fmtInt(r.Clicks), fmtPct(r.Percentage), fmtInt(r.PrevClicks)})
	}

	items := [][]string{{"Type", "Title", "Short URL", "Engagements", "Percentage", "Previous engagements"}}
	for _, it := range d.Items {
		items = append(items, []string{csvSafe(it.Type), csvSafe(it.Title), csvSafe(it.ShortURL),
			fmtInt(it.Clicks), fmtPct(it.Percentage), fmtInt(it.PrevClicks)})
	}

	top := [][]string{{"Metric", "Value", "Engagements"}}
	if d.TopDay != nil {
		top = append(top, []string{"Top day", csvSafe(d.TopDay.Date), fmtInt(d.TopDay.Count)})
	}
	if d.TopLocation != nil {
		top = append(top, []string{"Top location", csvSafe(d.TopLocation.Name), fmtInt(d.TopLocation.Count)})
	}

	return []exportFile{
		{"summary.csv", summary},
		{"engagements_over_time.csv", overTime},
		{"devices.csv", devices},
		{"referrers.csv", referrers},
		{"locations.csv", locations},
		{"links_and_qr_codes.csv", items},
		{"top.csv", top},
	}
}
