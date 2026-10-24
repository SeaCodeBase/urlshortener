package repository

import (
	"time"
)

// Shared time-series and breakdown helpers for event tables (clicks,
// page_views). Both tables use the same column names for the facet columns.

// utcDay truncates t to midnight of its UTC day.
func utcDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// halfOpen turns inclusive UTC dates into the [start, end) instant range.
func halfOpen(from, to time.Time) (time.Time, time.Time) {
	return utcDay(from), utcDay(to).AddDate(0, 0, 1)
}

// seriesUnit is a bucket size: its SQL DATE_FORMAT pattern, Go layout and step.
type seriesUnit struct {
	sqlFormat string
	goLayout  string
	step      time.Duration
}

// seriesUnits maps a unit name to its bucket format.
var seriesUnits = map[string]seriesUnit{
	"day":  {"%Y-%m-%d", "2006-01-02", 24 * time.Hour},
	"hour": {"%Y-%m-%dT%H:00", "2006-01-02T15:00", time.Hour},
}

// bucketExpr is the SQL expression that formats the time column col into the
// unit's bucket key.
func (u seriesUnit) bucketExpr(col string) string {
	return `DATE_FORMAT(` + col + `, '` + u.sqlFormat + `')`
}

// zeroFill returns one bucket per unit step over the inclusive UTC date range,
// taking counts from rows and 0 for missing keys.
func (u seriesUnit) zeroFill(rows []Bucket, from, to time.Time) []Bucket {
	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.Date] = row.Count
	}
	start, end := halfOpen(from, to)
	out := []Bucket{}
	for t := start; t.Before(end); t = t.Add(u.step) {
		key := t.Format(u.goLayout)
		out = append(out, Bucket{Date: key, Count: counts[key]})
	}
	return out
}

// breakdownColumns maps a facet to its key and secondary key expressions.
var breakdownColumns = map[string][2]string{
	"country":  {`COALESCE(NULLIF(country, ''), 'Unknown')`, `''`},
	"city":     {`COALESCE(NULLIF(city, ''), 'Unknown')`, `COALESCE(NULLIF(country, ''), 'Unknown')`},
	"referrer": {`COALESCE(NULLIF(referrer, ''), 'Direct')`, `''`},
	"device":   {`CASE WHEN device_type IS NULL OR device_type IN ('', 'unknown') THEN 'Unknown' ELSE device_type END`, `''`},
	"browser":  {`COALESCE(NULLIF(browser, ''), 'Unknown')`, `''`},
	"os":       {`COALESCE(NULLIF(os, ''), 'Unknown')`, `''`},
}

// breakdownOrder is the shared GROUP BY / ORDER BY tail of a breakdown query
// (followed by a LIMIT ? argument).
const breakdownOrder = ` GROUP BY k, k2 ORDER BY cnt DESC, k ASC, k2 ASC LIMIT ?`
