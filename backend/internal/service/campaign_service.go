package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
)

// Compile-time check: CampaignServiceImpl implements CampaignService
var _ CampaignService = (*CampaignServiceImpl)(nil)

const (
	campaignNameMax        = 120
	campaignDescriptionMax = 500
	channelNameMax         = 80
	// campaignAddLinksMax caps the link ids of one AddLinks request.
	campaignAddLinksMax = 500
	// campaignUTMMedium is utm_medium of links created in an AutoUTM campaign.
	campaignUTMMedium = "bitly"
)

// CampaignInput is the body of POST/PUT /campaigns. NewChannels are created
// (or matched to existing channels of the group by name) and attached along
// with ChannelIDs.
type CampaignInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AutoUTM     bool     `json:"auto_utm"`
	ChannelIDs  []uint64 `json:"channel_ids"`
	NewChannels []string `json:"new_channels"`
}

// CampaignDetail is a campaign with its channels and their links.
type CampaignDetail struct {
	model.Campaign
	Channels []ChannelDetail `json:"channels"`
}

// ChannelDetail is a channel of a campaign with the links in it.
type ChannelDetail struct {
	model.Channel
	Links []CampaignLinkItem `json:"links"`
}

// CampaignLinkItem is a link in a campaign channel. The handler renders the
// embedded link through the link presenter.
type CampaignLinkItem struct {
	model.LinkWithStats
	ChannelID   uint64 `json:"channel_id"`
	ChannelName string `json:"channel_name"`
}

// CreateLinkInChannelInput is the body of POST /campaigns/:id/channels/:cid/links.
type CreateLinkInChannelInput struct {
	OriginalURL string   `json:"original_url"`
	Title       string   `json:"title"`
	CustomCode  string   `json:"custom_code"`
	DomainID    *uint64  `json:"domain_id"`
	Tags        []string `json:"tags"`
}

// CampaignTotals are the campaign-wide figures of CampaignStats; a link in
// several channels counts once.
type CampaignTotals struct {
	Links       int   `json:"links"`
	Engagements int64 `json:"engagements"`
}

// CampaignStats is GET /campaigns/:id/stats.
type CampaignStats struct {
	Totals   CampaignTotals `json:"totals"`
	Channels []ChannelStats `json:"channels"`
}

// ChannelStats is one channel's engagements over the stats window.
type ChannelStats struct {
	ChannelID   uint64        `json:"channel_id"`
	Name        string        `json:"name"`
	Links       int           `json:"links"`
	Engagements int64         `json:"engagements"`
	Series      []SeriesPoint `json:"series"`
}

// LinkMembership is a link's place in a campaign channel.
type LinkMembership struct {
	CampaignID   uint64 `json:"campaign_id"`
	CampaignName string `json:"campaign_name"`
	ChannelID    uint64 `json:"channel_id"`
	ChannelName  string `json:"channel_name"`
}

type CampaignServiceDeps struct {
	Campaigns repository.CampaignRepository
	Links     repository.LinkRepository
	LinkSvc   LinkService
	Clicks    repository.ClickRepository
	Activity  ActivityService
}

type CampaignServiceImpl struct {
	campaigns repository.CampaignRepository
	links     repository.LinkRepository
	linkSvc   LinkService
	clicks    repository.ClickRepository
	activity  ActivityService
	now       func() time.Time
}

func NewCampaignService(d CampaignServiceDeps) *CampaignServiceImpl {
	return &CampaignServiceImpl{
		campaigns: d.Campaigns,
		links:     d.Links,
		linkSvc:   d.LinkSvc,
		clicks:    d.Clicks,
		activity:  d.Activity,
		now:       time.Now,
	}
}

// SetClock overrides the time source (tests).
func (s *CampaignServiceImpl) SetClock(now func() time.Time) { s.now = now }

// load returns the campaign when it belongs to groupID, else ErrCampaignNotFound.
func (s *CampaignServiceImpl) load(ctx context.Context, groupID, id uint64) (*model.Campaign, error) {
	c, err := s.campaigns.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.GroupID != groupID {
		return nil, repository.ErrCampaignNotFound
	}
	return c, nil
}

func validChannelName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > channelNameMax {
		return "", invalid("channel name must be 1-%d characters", channelNameMax)
	}
	return name, nil
}

// validateInput trims in and checks name and description.
func validateCampaignInput(in *CampaignInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > campaignNameMax {
		return invalid("name must be 1-%d characters", campaignNameMax)
	}
	if utf8.RuneCountInString(in.Description) > campaignDescriptionMax {
		return invalid("description must be at most %d characters", campaignDescriptionMax)
	}
	return nil
}

// resolveChannels validates in's channels against the group and returns the
// channels to attach, creating the new ones. Nothing is created unless the
// whole input is valid.
func (s *CampaignServiceImpl) resolveChannels(ctx context.Context, groupID uint64, in CampaignInput) ([]model.Channel, error) {
	if len(in.ChannelIDs) == 0 && len(in.NewChannels) == 0 {
		return []model.Channel{}, nil
	}
	existing, err := s.campaigns.ListChannels(ctx, groupID)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint64]model.Channel, len(existing))
	byName := make(map[string]model.Channel, len(existing))
	for _, ch := range existing {
		byID[ch.ID] = ch
		byName[strings.ToLower(ch.Name)] = ch
	}

	out := []model.Channel{}
	seen := map[uint64]bool{}
	add := func(ch model.Channel) {
		if !seen[ch.ID] {
			seen[ch.ID] = true
			out = append(out, ch)
		}
	}
	for _, id := range in.ChannelIDs {
		ch, ok := byID[id]
		if !ok {
			return nil, invalid("unknown channel %d", id)
		}
		add(ch)
	}

	var create []string
	pending := map[string]bool{}
	for _, raw := range in.NewChannels {
		name, err := validChannelName(raw)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(name)
		if ch, ok := byName[key]; ok {
			add(ch)
			continue
		}
		if !pending[key] {
			pending[key] = true
			create = append(create, name)
		}
	}
	for _, name := range create {
		ch, err := s.createOrFindChannel(ctx, groupID, name)
		if err != nil {
			return nil, err
		}
		add(*ch)
	}
	return out, nil
}

// createOrFindChannel creates the channel, or returns the group's channel of
// that name when it was created concurrently.
func (s *CampaignServiceImpl) createOrFindChannel(ctx context.Context, groupID uint64, name string) (*model.Channel, error) {
	ch := &model.Channel{GroupID: groupID, Name: name}
	err := s.campaigns.CreateChannel(ctx, ch)
	if err == nil {
		return ch, nil
	}
	if !errors.Is(err, repository.ErrChannelExists) {
		return nil, err
	}
	all, lerr := s.campaigns.ListChannels(ctx, groupID)
	if lerr != nil {
		return nil, lerr
	}
	for i := range all {
		if strings.EqualFold(all[i].Name, name) {
			return &all[i], nil
		}
	}
	return nil, err
}

func channelIDs(chs []model.Channel) []uint64 {
	ids := make([]uint64, len(chs))
	for i, ch := range chs {
		ids[i] = ch.ID
	}
	return ids
}

func sortChannels(chs []model.Channel) {
	sort.SliceStable(chs, func(i, j int) bool {
		if chs[i].Name != chs[j].Name {
			return chs[i].Name < chs[j].Name
		}
		return chs[i].ID < chs[j].ID
	})
}

func (s *CampaignServiceImpl) Create(ctx context.Context, userID, groupID uint64, in CampaignInput) (*CampaignDetail, error) {
	if err := validateCampaignInput(&in); err != nil {
		return nil, err
	}
	chs, err := s.resolveChannels(ctx, groupID, in)
	if err != nil {
		return nil, err
	}
	c := &model.Campaign{UserID: userID, GroupID: groupID, Name: in.Name, Description: in.Description, AutoUTM: in.AutoUTM}
	if err := s.campaigns.Create(ctx, c); err != nil {
		return nil, err
	}
	if err := s.campaigns.SetCampaignChannels(ctx, c.ID, channelIDs(chs)); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionCampaignCreate, EntityCampaign, c.ID, map[string]any{"name": c.Name})

	sortChannels(chs)
	d := &CampaignDetail{Campaign: *c, Channels: make([]ChannelDetail, len(chs))}
	for i, ch := range chs {
		d.Channels[i] = ChannelDetail{Channel: ch, Links: []CampaignLinkItem{}}
	}
	return d, nil
}

func (s *CampaignServiceImpl) Get(ctx context.Context, groupID, id uint64) (*CampaignDetail, error) {
	c, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, c)
}

// detail loads the campaign's channels and the links in each.
func (s *CampaignServiceImpl) detail(ctx context.Context, c *model.Campaign) (*CampaignDetail, error) {
	chs, err := s.campaigns.CampaignChannels(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	members, err := s.campaigns.Links(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	links, err := s.memberLinks(ctx, c, len(members))
	if err != nil {
		return nil, err
	}

	d := &CampaignDetail{Campaign: *c, Channels: make([]ChannelDetail, len(chs))}
	index := make(map[uint64]int, len(chs))
	for i, ch := range chs {
		d.Channels[i] = ChannelDetail{Channel: ch, Links: []CampaignLinkItem{}}
		index[ch.ID] = i
	}
	for _, m := range members {
		i, ok := index[m.ChannelID]
		if !ok {
			continue
		}
		l, ok := links[m.LinkID]
		if !ok {
			continue
		}
		d.Channels[i].Links = append(d.Channels[i].Links, CampaignLinkItem{
			LinkWithStats: l, ChannelID: m.ChannelID, ChannelName: d.Channels[i].Name,
		})
	}
	return d, nil
}

// memberLinks returns the campaign's links with tags and click totals, keyed
// by id, through the Links-list query filtered by campaign.
func (s *CampaignServiceImpl) memberLinks(ctx context.Context, c *model.Campaign, memberships int) (map[uint64]model.LinkWithStats, error) {
	out := map[uint64]model.LinkWithStats{}
	if memberships == 0 {
		return out, nil
	}
	campaignID := c.ID
	for page := 1; ; page++ {
		res, err := s.linkSvc.List(ctx, c.GroupID, ListLinksParams{
			Filter: repository.LinkFilter{CampaignID: &campaignID},
			Page:   page,
			Limit:  maxPageSize,
		})
		if err != nil {
			return nil, err
		}
		for _, l := range res.Links {
			out[l.ID] = l
		}
		if len(res.Links) == 0 || page >= res.TotalPages {
			return out, nil
		}
	}
}

// distinctLinkIDs returns the memberships' link ids in first-seen order.
func distinctLinkIDs(members []model.CampaignLink) []uint64 {
	ids := []uint64{}
	seen := map[uint64]bool{}
	for _, m := range members {
		if !seen[m.LinkID] {
			seen[m.LinkID] = true
			ids = append(ids, m.LinkID)
		}
	}
	return ids
}

func (s *CampaignServiceImpl) List(ctx context.Context, groupID uint64) ([]model.CampaignSummary, error) {
	rows, err := s.campaigns.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.CampaignSummary{}
	}
	from := time.Unix(0, 0).UTC()
	to := s.now().UTC().Add(24 * time.Hour)
	for i := range rows {
		if rows[i].LinkCount == 0 {
			continue
		}
		members, err := s.campaigns.Links(ctx, rows[i].ID)
		if err != nil {
			return nil, err
		}
		ids := distinctLinkIDs(members)
		if len(ids) == 0 {
			continue
		}
		n, err := s.clicks.CountScope(ctx, repository.ClickScope{GroupID: groupID, LinkIDs: ids}, from, to)
		if err != nil {
			return nil, err
		}
		rows[i].Engagements = n
	}
	return rows, nil
}

func (s *CampaignServiceImpl) Update(ctx context.Context, userID, groupID, id uint64, in CampaignInput) (*CampaignDetail, error) {
	c, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	if err := validateCampaignInput(&in); err != nil {
		return nil, err
	}
	chs, err := s.resolveChannels(ctx, groupID, in)
	if err != nil {
		return nil, err
	}
	c.Name, c.Description, c.AutoUTM = in.Name, in.Description, in.AutoUTM
	if err := s.campaigns.Update(ctx, c); err != nil {
		return nil, err
	}
	if err := s.campaigns.SetCampaignChannels(ctx, c.ID, channelIDs(chs)); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionCampaignUpdate, EntityCampaign, c.ID, map[string]any{"name": c.Name})
	return s.detail(ctx, c)
}

func (s *CampaignServiceImpl) Delete(ctx context.Context, userID, groupID, id uint64) error {
	c, err := s.load(ctx, groupID, id)
	if err != nil {
		return err
	}
	if err := s.campaigns.Delete(ctx, c.ID); err != nil {
		return err
	}
	s.activity.Log(ctx, userID, groupID, ActionCampaignDelete, EntityCampaign, c.ID, map[string]any{"name": c.Name})
	return nil
}

func (s *CampaignServiceImpl) ListChannels(ctx context.Context, groupID uint64) ([]model.Channel, error) {
	chs, err := s.campaigns.ListChannels(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if chs == nil {
		chs = []model.Channel{}
	}
	return chs, nil
}

func (s *CampaignServiceImpl) CreateChannel(ctx context.Context, groupID uint64, name string) (*model.Channel, error) {
	name, err := validChannelName(name)
	if err != nil {
		return nil, err
	}
	ch := &model.Channel{GroupID: groupID, Name: name}
	if err := s.campaigns.CreateChannel(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

// campaignChannel returns the campaign's channel channelID, or a
// ValidationError when it is not attached.
func (s *CampaignServiceImpl) campaignChannel(ctx context.Context, campaignID, channelID uint64) (*model.Channel, error) {
	chs, err := s.campaigns.CampaignChannels(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	for i := range chs {
		if chs[i].ID == channelID {
			return &chs[i], nil
		}
	}
	return nil, invalid("channel is not part of the campaign")
}

func (s *CampaignServiceImpl) AddLinks(ctx context.Context, userID, groupID, campaignID, channelID uint64, linkIDs []uint64) error {
	c, err := s.load(ctx, groupID, campaignID)
	if err != nil {
		return err
	}
	ids := make([]uint64, 0, len(linkIDs))
	seen := make(map[uint64]bool, len(linkIDs))
	for _, id := range linkIDs {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return invalid("at least one link is required")
	}
	if len(ids) > campaignAddLinksMax {
		return invalid("at most %d links per request", campaignAddLinksMax)
	}
	if _, err := s.campaignChannel(ctx, c.ID, channelID); err != nil {
		return err
	}
	links, err := s.links.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}
	ok := make(map[uint64]bool, len(links))
	for _, l := range links {
		if l.GroupID == groupID && l.Source == model.LinkSourceLink {
			ok[l.ID] = true
		}
	}
	for _, id := range ids {
		if !ok[id] {
			return invalid("link %d cannot be added to the campaign", id)
		}
	}
	if err := s.campaigns.AddLinks(ctx, c.ID, channelID, ids); err != nil {
		return err
	}
	s.activity.Log(ctx, userID, groupID, ActionCampaignUpdate, EntityCampaign, c.ID,
		map[string]any{"name": c.Name, "links": len(ids)})
	return nil
}

func (s *CampaignServiceImpl) RemoveLink(ctx context.Context, userID, groupID, campaignID, channelID, linkID uint64) error {
	c, err := s.load(ctx, groupID, campaignID)
	if err != nil {
		return err
	}
	if err := s.campaigns.RemoveLink(ctx, c.ID, channelID, linkID); err != nil {
		return err
	}
	s.activity.Log(ctx, userID, groupID, ActionCampaignUpdate, EntityCampaign, c.ID,
		map[string]any{"name": c.Name, "link_id": linkID})
	return nil
}

func (s *CampaignServiceImpl) CreateLinkInChannel(ctx context.Context, userID, groupID, campaignID, channelID uint64, in CreateLinkInChannelInput) (*model.Link, error) {
	c, err := s.load(ctx, groupID, campaignID)
	if err != nil {
		return nil, err
	}
	ch, err := s.campaignChannel(ctx, c.ID, channelID)
	if err != nil {
		return nil, err
	}
	dest := in.OriginalURL
	if c.AutoUTM {
		dest, err = ApplyUTM(dest, UTMParams{Source: ch.Name, Medium: campaignUTMMedium, Campaign: c.Name})
		if err != nil {
			return nil, invalid("%s", err.Error())
		}
	}
	link, err := s.linkSvc.Create(ctx, userID, CreateLinkInput{
		OriginalURL: dest,
		Title:       in.Title,
		CustomCode:  in.CustomCode,
		DomainID:    in.DomainID,
		Tags:        in.Tags,
		GroupID:     groupID,
	})
	if err != nil {
		return nil, err
	}
	if err := s.campaigns.AddLinks(ctx, c.ID, ch.ID, []uint64{link.ID}); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionCampaignUpdate, EntityCampaign, c.ID,
		map[string]any{"name": c.Name, "links": 1})
	return link, nil
}

// zeroSeries returns the zero-filled points SeriesScope would return for a
// scope without clicks: one per unit step over the inclusive UTC date range,
// keyed like the repository's buckets.
func zeroSeries(from, to time.Time, unit string) []SeriesPoint {
	layout, step := "2006-01-02", 24*time.Hour
	if unit == "hour" {
		layout, step = "2006-01-02T15:00", time.Hour
	}
	start, end := dateOnly(from), dateOnly(to).AddDate(0, 0, 1)
	out := []SeriesPoint{}
	for t := start; t.Before(end); t = t.Add(step) {
		out = append(out, SeriesPoint{Date: t.Format(layout)})
	}
	return out
}

func (s *CampaignServiceImpl) Stats(ctx context.Context, groupID, id uint64, query StatsQuery) (*CampaignStats, error) {
	c, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	q, err := NormalizeStatsQuery(query, s.now())
	if err != nil {
		return nil, invalid("%s", err.Error())
	}
	chs, err := s.campaigns.CampaignChannels(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	members, err := s.campaigns.Links(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	byChannel := map[uint64][]uint64{}
	for _, m := range members {
		byChannel[m.ChannelID] = append(byChannel[m.ChannelID], m.LinkID)
	}

	st := &CampaignStats{Channels: make([]ChannelStats, 0, len(chs))}
	channelLinks := 0
	for _, ch := range chs {
		ids := byChannel[ch.ID]
		cs := ChannelStats{ChannelID: ch.ID, Name: ch.Name, Links: len(ids)}
		if len(ids) == 0 {
			// An empty LinkIDs still queries; skip it.
			cs.Series = zeroSeries(q.From, q.To, q.Unit)
		} else {
			buckets, err := s.clicks.SeriesScope(ctx,
				repository.ClickScope{GroupID: groupID, LinkIDs: ids, Source: q.Source}, q.From, q.To, q.Unit)
			if err != nil {
				return nil, err
			}
			cs.Series = toSeries(buckets)
			for _, p := range cs.Series {
				cs.Engagements += p.Clicks
			}
		}
		channelLinks += cs.Links
		st.Totals.Engagements += cs.Engagements
		st.Channels = append(st.Channels, cs)
	}

	all := distinctLinkIDs(members)
	st.Totals.Links = len(all)
	if channelLinks != len(all) && len(all) > 0 {
		// A link in several channels: count its clicks once.
		n, err := s.clicks.CountScope(ctx,
			repository.ClickScope{GroupID: groupID, LinkIDs: all, Source: q.Source}, q.From, q.To)
		if err != nil {
			return nil, err
		}
		st.Totals.Engagements = n
	}
	return st, nil
}

func (s *CampaignServiceImpl) LinkMemberships(ctx context.Context, groupID, linkID uint64) ([]LinkMembership, error) {
	link, err := s.links.GetByID(ctx, linkID)
	if err != nil {
		return nil, err
	}
	if link.GroupID != groupID {
		return nil, repository.ErrLinkNotFound
	}
	rows, err := s.campaigns.LinkCampaigns(ctx, linkID)
	if err != nil {
		return nil, err
	}
	type campaignNames struct {
		name     string
		channels map[uint64]string
	}
	cache := map[uint64]*campaignNames{}
	out := []LinkMembership{}
	for _, r := range rows {
		cn, ok := cache[r.CampaignID]
		if !ok {
			c, err := s.campaigns.GetByID(ctx, r.CampaignID)
			if errors.Is(err, repository.ErrCampaignNotFound) {
				cache[r.CampaignID] = nil
				continue
			}
			if err != nil {
				return nil, err
			}
			if c.GroupID != groupID {
				cache[r.CampaignID] = nil
				continue
			}
			chs, err := s.campaigns.CampaignChannels(ctx, c.ID)
			if err != nil {
				return nil, err
			}
			cn = &campaignNames{name: c.Name, channels: make(map[uint64]string, len(chs))}
			for _, ch := range chs {
				cn.channels[ch.ID] = ch.Name
			}
			cache[r.CampaignID] = cn
		}
		if cn == nil {
			continue
		}
		out = append(out, LinkMembership{
			CampaignID: r.CampaignID, CampaignName: cn.name,
			ChannelID: r.ChannelID, ChannelName: cn.channels[r.ChannelID],
		})
	}
	return out, nil
}
