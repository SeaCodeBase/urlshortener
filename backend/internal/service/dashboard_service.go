package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
)

// Compile-time check: DashboardServiceImpl implements DashboardService
var _ DashboardService = (*DashboardServiceImpl)(nil)

// ErrDashboardDefault is returned when deleting a group's default dashboard.
var ErrDashboardDefault = errors.New("the default dashboard cannot be deleted")

// ValidationError is invalid user input; handlers map it to 400 with Msg.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}

const (
	// dashboardNameMax is the dashboard name limit in characters.
	dashboardNameMax = 255
	// dashboardFilterMaxIDs caps the ids of a links/qr filter.
	dashboardFilterMaxIDs = 200
	// DefaultDashboardDateRange is a new dashboard's date range.
	DefaultDashboardDateRange = "last_7_days"
)

// DashboardDateRanges are the persisted date range presets.
var DashboardDateRanges = map[string]bool{
	"today": true, "last_7_days": true, "last_30_days": true, "last_60_days": true, "last_90_days": true,
}

// DashboardDateRangeDays is the length in UTC days of each date range preset.
var DashboardDateRangeDays = map[string]int{
	"today": 1, "last_7_days": 7, "last_30_days": 30, "last_60_days": 60, "last_90_days": 90,
}

// DashboardWindow returns the inclusive UTC-day window of a stored date range
// preset ending today (UTC): today → from = to = today; last_N_days → from =
// today−(N−1). An unknown preset reads as DefaultDashboardDateRange.
func DashboardWindow(dateRange string, now time.Time) (from, to time.Time) {
	n, ok := DashboardDateRangeDays[dateRange]
	if !ok {
		n = DashboardDateRangeDays[DefaultDashboardDateRange]
	}
	to = dateOnly(now)
	return to.AddDate(0, 0, -(n - 1)), to
}

// dashboardSorts are the accepted List sort keys.
var dashboardSorts = map[string]bool{"created": true, "modified": true, "name": true}

// DashboardModule is one module of a dashboard layout and its visualization.
type DashboardModule struct {
	Key string `json:"key"`
	Viz string `json:"viz"`
}

// DashboardLayout is the ordered list of a dashboard's modules; it holds each
// of the seven module keys exactly once.
type DashboardLayout struct {
	Modules []DashboardModule `json:"modules"`
}

// DefaultLayout is the layout of new dashboards, in display order.
var DefaultLayout = DashboardLayout{Modules: []DashboardModule{
	{Key: "top_day", Viz: "scorecard"},
	{Key: "top_location", Viz: "scorecard"},
	{Key: "over_time", Viz: "line"},
	{Key: "devices", Viz: "doughnut"},
	{Key: "referrers", Viz: "bar"},
	{Key: "locations", Viz: "table"},
	{Key: "items", Viz: "table"},
}}

// AllowedViz lists the visualizations each module key accepts.
var AllowedViz = map[string][]string{
	"top_day":      {"scorecard"},
	"top_location": {"scorecard"},
	"over_time":    {"line", "bar", "table"},
	"devices":      {"doughnut", "bar", "table"},
	"referrers":    {"bar", "doughnut", "table"},
	"locations":    {"table", "bar"},
	"items":        {"table", "bar"},
}

// DashboardCreator is the user who created a dashboard.
type DashboardCreator struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

// DashboardView is a dashboard as the API returns it.
type DashboardView struct {
	ID        uint64           `json:"id"`
	GroupID   uint64           `json:"group_id"`
	Name      string           `json:"name"`
	IsDefault bool             `json:"is_default"`
	DateRange string           `json:"date_range"`
	Filter    DashboardFilter  `json:"filter"`
	Layout    DashboardLayout  `json:"layout"`
	CreatedBy DashboardCreator `json:"created_by"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// DashboardPatch holds the fields to change; nil fields are left alone.
type DashboardPatch struct {
	Name      *string
	DateRange *string
	Filter    *DashboardFilter
	Layout    *DashboardLayout
}

// DashboardServiceDeps are the collaborators of DashboardServiceImpl.
type DashboardServiceDeps struct {
	Dashboards repository.DashboardRepository
	// Links and QRs validate filter ids.
	Links    repository.LinkRepository
	QRs      repository.QRRepository
	Activity ActivityService
}

type DashboardServiceImpl struct {
	repo     repository.DashboardRepository
	links    repository.LinkRepository
	qrs      repository.QRRepository
	activity ActivityService
}

func NewDashboardService(d DashboardServiceDeps) *DashboardServiceImpl {
	return &DashboardServiceImpl{repo: d.Dashboards, links: d.Links, qrs: d.QRs, activity: d.Activity}
}

// ValidateLayout checks that l holds each module key exactly once with an
// allowed visualization; order is free.
func ValidateLayout(l DashboardLayout) error {
	if len(l.Modules) != len(AllowedViz) {
		return invalid("layout must contain exactly %d modules", len(AllowedViz))
	}
	seen := make(map[string]bool, len(l.Modules))
	for _, m := range l.Modules {
		allowed, ok := AllowedViz[m.Key]
		if !ok {
			return invalid("unknown layout module %q", m.Key)
		}
		if seen[m.Key] {
			return invalid("duplicate layout module %q", m.Key)
		}
		seen[m.Key] = true
		if !containsString(allowed, m.Viz) {
			return invalid("visualization %q is not allowed for module %q", m.Viz, m.Key)
		}
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// validateFilter checks the filter type and that every id is unique, exists
// and belongs to the group (links: source 'link' only).
func (s *DashboardServiceImpl) validateFilter(ctx context.Context, groupID uint64, f DashboardFilter) error {
	switch f.Type {
	case FilterTypeAll:
		if len(f.IDs) > 0 {
			return invalid("filter type all takes no ids")
		}
		return nil
	case FilterTypeLinks, FilterTypeQR:
	default:
		return invalid("unknown filter type %q", f.Type)
	}
	if len(f.IDs) == 0 || len(f.IDs) > dashboardFilterMaxIDs {
		return invalid("filter must have between 1 and %d ids", dashboardFilterMaxIDs)
	}
	want := make(map[uint64]bool, len(f.IDs))
	for _, id := range f.IDs {
		if want[id] {
			return invalid("duplicate filter id %d", id)
		}
		want[id] = true
	}
	owners, err := s.filterIDOwners(ctx, f.Type, f.IDs)
	if err != nil {
		return err
	}
	for _, id := range f.IDs {
		if g, ok := owners[id]; !ok || g != groupID {
			if f.Type == FilterTypeLinks {
				return invalid("link %d not found", id)
			}
			return invalid("QR code %d not found", id)
		}
	}
	return nil
}

// filterIDOwners returns the group of each id of a links or qr filter that
// exists; a links id must be a plain link (source 'link'), so QR-owned links
// are absent.
func (s *DashboardServiceImpl) filterIDOwners(ctx context.Context, typ string, ids []uint64) (map[uint64]uint64, error) {
	owners := make(map[uint64]uint64, len(ids))
	if len(ids) == 0 {
		return owners, nil
	}
	if typ == FilterTypeLinks {
		links, err := s.links.GetByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, l := range links {
			if l.Source == model.LinkSourceLink {
				owners[l.ID] = l.GroupID
			}
		}
		return owners, nil
	}
	qrs, err := s.qrs.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, q := range qrs {
		owners[q.ID] = q.GroupID
	}
	return owners, nil
}

// pruneFilters drops from each links/qr filter the ids that no longer exist
// in the view's group (deleted after the filter was saved), with one lookup
// per filter type. A filter whose ids are all gone keeps its type with no
// ids. Only the views change; stored rows are not rewritten.
func (s *DashboardServiceImpl) pruneFilters(ctx context.Context, views []DashboardView) error {
	idsByType := map[string][]uint64{}
	seen := map[string]map[uint64]bool{FilterTypeLinks: {}, FilterTypeQR: {}}
	for _, v := range views {
		if v.Filter.Type != FilterTypeLinks && v.Filter.Type != FilterTypeQR {
			continue
		}
		for _, id := range v.Filter.IDs {
			if !seen[v.Filter.Type][id] {
				seen[v.Filter.Type][id] = true
				idsByType[v.Filter.Type] = append(idsByType[v.Filter.Type], id)
			}
		}
	}
	owners := map[string]map[uint64]uint64{}
	for _, typ := range []string{FilterTypeLinks, FilterTypeQR} {
		if len(idsByType[typ]) == 0 {
			continue
		}
		o, err := s.filterIDOwners(ctx, typ, idsByType[typ])
		if err != nil {
			return err
		}
		owners[typ] = o
	}
	for i := range views {
		f := &views[i].Filter
		if f.Type != FilterTypeLinks && f.Type != FilterTypeQR {
			continue
		}
		kept := make([]uint64, 0, len(f.IDs))
		for _, id := range f.IDs {
			if g, ok := owners[f.Type][id]; ok && g == views[i].GroupID {
				kept = append(kept, id)
			}
		}
		f.IDs = kept
	}
	return nil
}

func validateDashboardName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > dashboardNameMax {
		return "", invalid("name must be between 1 and %d characters", dashboardNameMax)
	}
	return name, nil
}

func (s *DashboardServiceImpl) List(ctx context.Context, userID, groupID uint64, sort string) ([]DashboardView, error) {
	if sort == "" {
		sort = "created"
	}
	if !dashboardSorts[sort] {
		return nil, invalid("unknown sort %q", sort)
	}
	if _, err := s.repo.EnsureDefault(ctx, groupID, userID, toMap(DefaultLayout)); err != nil {
		return nil, err
	}
	ds, err := s.repo.ListByGroup(ctx, groupID, sort)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, ds)
}

func (s *DashboardServiceImpl) Get(ctx context.Context, groupID, id uint64) (*DashboardView, error) {
	d, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, d)
}

func (s *DashboardServiceImpl) Create(ctx context.Context, userID, groupID uint64, name string) (*DashboardView, error) {
	name, err := validateDashboardName(name)
	if err != nil {
		return nil, err
	}
	d := &model.Dashboard{
		GroupID:   groupID,
		CreatedBy: userID,
		Name:      name,
		DateRange: DefaultDashboardDateRange,
		Filter:    toMap(DashboardFilter{Type: FilterTypeAll}),
		Layout:    toMap(DefaultLayout),
	}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionDashboardCreate, EntityDashboard, d.ID, map[string]any{"name": d.Name})
	return s.view(ctx, d)
}

func (s *DashboardServiceImpl) Update(ctx context.Context, userID, groupID, id uint64, patch DashboardPatch) (*DashboardView, error) {
	d, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	if patch.Name != nil {
		name, err := validateDashboardName(*patch.Name)
		if err != nil {
			return nil, err
		}
		d.Name = name
	}
	if patch.DateRange != nil {
		if !DashboardDateRanges[*patch.DateRange] {
			return nil, invalid("unknown date range %q", *patch.DateRange)
		}
		d.DateRange = *patch.DateRange
	}
	if patch.Layout != nil {
		if err := ValidateLayout(*patch.Layout); err != nil {
			return nil, err
		}
		d.Layout = toMap(*patch.Layout)
	}
	if patch.Filter != nil {
		if err := s.validateFilter(ctx, groupID, *patch.Filter); err != nil {
			return nil, err
		}
		d.Filter = toMap(*patch.Filter)
	}
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionDashboardUpdate, EntityDashboard, d.ID, map[string]any{"name": d.Name})
	// Re-read for the database's updated_at.
	return s.Get(ctx, groupID, id)
}

func (s *DashboardServiceImpl) Duplicate(ctx context.Context, userID, groupID, id uint64) (*DashboardView, error) {
	src, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	name := "Copy of " + src.Name
	if r := []rune(name); len(r) > dashboardNameMax {
		name = string(r[:dashboardNameMax])
	}
	d := &model.Dashboard{
		GroupID:   groupID,
		CreatedBy: userID,
		Name:      name,
		DateRange: src.DateRange,
		Filter:    src.Filter,
		Layout:    src.Layout,
	}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionDashboardDuplicate, EntityDashboard, d.ID,
		map[string]any{"name": d.Name, "source_id": src.ID})
	return s.view(ctx, d)
}

func (s *DashboardServiceImpl) Delete(ctx context.Context, userID, groupID, id uint64) error {
	d, err := s.load(ctx, groupID, id)
	if err != nil {
		return err
	}
	if d.IsDefault {
		return ErrDashboardDefault
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.activity.Log(ctx, userID, groupID, ActionDashboardDelete, EntityDashboard, id, map[string]any{"name": d.Name})
	return nil
}

// load returns the dashboard when it belongs to the group; otherwise
// ErrDashboardNotFound so other groups' dashboards are not revealed.
func (s *DashboardServiceImpl) load(ctx context.Context, groupID, id uint64) (*model.Dashboard, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.GroupID != groupID {
		return nil, repository.ErrDashboardNotFound
	}
	return d, nil
}

func (s *DashboardServiceImpl) view(ctx context.Context, d *model.Dashboard) (*DashboardView, error) {
	vs, err := s.views(ctx, []model.Dashboard{*d})
	if err != nil {
		return nil, err
	}
	return &vs[0], nil
}

func (s *DashboardServiceImpl) views(ctx context.Context, ds []model.Dashboard) ([]DashboardView, error) {
	out := make([]DashboardView, 0, len(ds))
	if len(ds) == 0 {
		return out, nil
	}
	var userIDs []uint64
	seen := map[uint64]bool{}
	for _, d := range ds {
		if !seen[d.CreatedBy] {
			seen[d.CreatedBy] = true
			userIDs = append(userIDs, d.CreatedBy)
		}
	}
	names, err := s.repo.CreatorNames(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	for _, d := range ds {
		out = append(out, DashboardView{
			ID:        d.ID,
			GroupID:   d.GroupID,
			Name:      d.Name,
			IsDefault: d.IsDefault,
			DateRange: d.DateRange,
			Filter:    filterFromMap(d.Filter),
			Layout:    layoutFromMap(d.Layout),
			CreatedBy: DashboardCreator{ID: d.CreatedBy, Name: names[d.CreatedBy]},
			CreatedAt: d.CreatedAt,
			UpdatedAt: d.UpdatedAt,
		})
	}
	if err := s.pruneFilters(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// toMap converts a filter or layout to its stored JSON document.
func toMap(v any) model.JSONMap {
	b, err := json.Marshal(v)
	if err != nil {
		return model.JSONMap{}
	}
	var m model.JSONMap
	if err := json.Unmarshal(b, &m); err != nil {
		return model.JSONMap{}
	}
	return m
}

// fromMap decodes a stored JSON document into out.
func fromMap(m model.JSONMap, out any) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// filterFromMap decodes a stored filter; a malformed one reads as "all".
func filterFromMap(m model.JSONMap) DashboardFilter {
	var f DashboardFilter
	if err := fromMap(m, &f); err != nil {
		return DashboardFilter{Type: FilterTypeAll}
	}
	switch f.Type {
	case FilterTypeLinks, FilterTypeQR:
		return f
	default:
		return DashboardFilter{Type: FilterTypeAll}
	}
}

// layoutFromMap decodes a stored layout; a malformed or invalid one reads as
// DefaultLayout.
func layoutFromMap(m model.JSONMap) DashboardLayout {
	var l DashboardLayout
	if err := fromMap(m, &l); err != nil || ValidateLayout(l) != nil {
		return DashboardLayout{Modules: append([]DashboardModule(nil), DefaultLayout.Modules...)}
	}
	return l
}
