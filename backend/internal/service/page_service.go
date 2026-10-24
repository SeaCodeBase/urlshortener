package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

// Compile-time check: PageServiceImpl implements PageService
var _ PageService = (*PageServiceImpl)(nil)

// Page limits (characters).
const (
	pageSlugMin        = 3
	pageSlugMax        = 40
	pageTitleMax       = 255
	pageDescriptionMax = 500
	pageAvatarURLMax   = 512

	blockLinkTitleMax   = 80
	blockLinkDescMax    = 160
	blockTextHeadingMax = 120
	blockTextBodyMax    = 1000
	blockImageAltMax    = 120
	blockSocialItemsMax = 10

	// pageViewSource is appended to a link block's short URL in a published
	// snapshot so its clicks are recorded with source 'page'.
	pageViewSource = "r=page"
)

// Slug check reasons (SlugCheck.Reason).
const (
	SlugReasonInvalid  = "invalid"
	SlugReasonReserved = "reserved"
	SlugReasonTaken    = "taken"
)

var (
	pageSlugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

	// ReservedPageSlugs cannot be claimed as page slugs.
	ReservedPageSlugs = map[string]bool{
		"api": true, "admin": true, "m": true, "dashboard": true, "login": true,
		"register": true, "static": true, "assets": true, "health": true,
	}

	// PageTemplates are the templates accepted by Create ("" means scratch).
	PageTemplates = map[string]bool{"scratch": true, "classic": true, "bold": true, "minimal": true}

	designThemes     = map[string]bool{"classic": true, "bold": true, "minimal": true, "dark": true, "sunset": true}
	designBackground = map[string]bool{"solid": true, "gradient": true}
	designButtons    = map[string]bool{"fill": true, "outline": true}
	designRadii      = map[string]bool{"square": true, "rounded": true, "pill": true}
	designFonts      = map[string]bool{"inter": true, "serif": true, "mono": true}

	socialPlatforms = map[string]bool{
		"instagram": true, "x": true, "facebook": true, "tiktok": true,
		"youtube": true, "linkedin": true, "email": true, "website": true,
	}
)

// designPreset is one theme's design values.
type designPreset struct {
	bgType, bg, bg2, text, buttonStyle, button, buttonText, radius, font string
}

// designPresets are the five design themes (kept in sync with the
// frontend's designPresets.ts).
var designPresets = map[string]designPreset{
	"classic": {"solid", "#F4F6FA", "#FFFFFF", "#273144", "fill", "#2A5BD7", "#FFFFFF", "rounded", "inter"},
	"bold":    {"solid", "#111827", "#111827", "#F9FAFB", "fill", "#F59E0B", "#111827", "pill", "inter"},
	"minimal": {"solid", "#FFFFFF", "#FFFFFF", "#111827", "outline", "#111827", "#111827", "square", "serif"},
	"dark":    {"gradient", "#0F172A", "#1E293B", "#E2E8F0", "fill", "#38BDF8", "#0F172A", "rounded", "mono"},
	"sunset":  {"gradient", "#F97316", "#EC4899", "#FFFFFF", "fill", "#FFFFFF", "#9A3412", "pill", "inter"},
}

// DefaultDesign returns a fresh copy of the design preset for template
// (scratch uses classic; dark and sunset are themes too); nil when unknown.
func DefaultDesign(template string) model.JSONMap {
	theme := template
	if theme == "scratch" || theme == "" {
		theme = "classic"
	}
	p, ok := designPresets[theme]
	if !ok {
		return nil
	}
	return model.JSONMap{
		"theme":      theme,
		"background": map[string]any{"type": p.bgType, "color": p.bg, "color2": p.bg2},
		"text_color": p.text,
		"button": map[string]any{
			"style": p.buttonStyle, "radius": p.radius, "color": p.button, "text_color": p.buttonText,
		},
		"font": p.font,
	}
}

// SlugCheck is the result of CheckSlug; Reason is "" when Available, else
// one of SlugReason*.
type SlugCheck struct {
	Slug      string `json:"slug"`
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

// CreatePageInput creates a page; Template is scratch|classic|bold|minimal
// ("" = scratch).
type CreatePageInput struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Template string `json:"template"`
}

// UpdatePageInput changes the non-nil fields of a page.
type UpdatePageInput struct {
	Title       *string        `json:"title"`
	Description *string        `json:"description"`
	AvatarURL   *string        `json:"avatar_url"`
	Design      map[string]any `json:"design"`
}

// NewLinkInput creates a new Links-list link for a link block.
type NewLinkInput struct {
	OriginalURL string `json:"original_url"`
	Title       string `json:"title"`
}

// BlockInput adds a block. Link blocks need exactly one of LinkID and
// NewLink; other types neither.
type BlockInput struct {
	Type    string         `json:"type"`
	Content map[string]any `json:"content"`
	LinkID  *uint64        `json:"link_id"`
	NewLink *NewLinkInput  `json:"new_link"`
}

// BlockPatch changes the non-nil fields of a block; Content replaces the
// whole content when given.
type BlockPatch struct {
	Content  map[string]any `json:"content"`
	IsActive *bool          `json:"is_active"`
	IsPinned *bool          `json:"is_pinned"`
}

// BlockLink summarizes a link block's link.
type BlockLink struct {
	ID          uint64 `json:"id"`
	Title       string `json:"title"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// BlockView is a block as served to the editor.
type BlockView struct {
	ID       uint64        `json:"id"`
	Type     string        `json:"type"`
	Position int           `json:"position"`
	Content  model.JSONMap `json:"content"`
	IsActive bool          `json:"is_active"`
	IsPinned bool          `json:"is_pinned"`
	LinkID   *uint64       `json:"link_id"`
	Link     *BlockLink    `json:"link"`
}

// PageView is a page as served to the editor. Blocks is nil for list items
// (omitted from JSON) and non-nil for a single page.
type PageView struct {
	ID                    uint64        `json:"id"`
	GroupID               uint64        `json:"group_id"`
	Slug                  string        `json:"slug"`
	Title                 string        `json:"title"`
	Description           string        `json:"description"`
	AvatarURL             string        `json:"avatar_url"`
	Design                model.JSONMap `json:"design"`
	Status                string        `json:"status"`
	PublishedAt           *time.Time    `json:"published_at"`
	HasUnpublishedChanges bool          `json:"has_unpublished_changes"`
	Archived              bool          `json:"archived"`
	URL                   string        `json:"url"`
	Views                 int64         `json:"views"`
	CreatedAt             time.Time     `json:"created_at"`
	UpdatedAt             time.Time     `json:"updated_at"`
	Blocks                []BlockView   `json:"-"`
}

// MarshalJSON emits "blocks" only when Blocks is non-nil (an empty page
// still gets "blocks": []).
func (v PageView) MarshalJSON() ([]byte, error) {
	type alias PageView
	out := struct {
		alias
		Blocks any `json:"blocks,omitempty"`
	}{alias: alias(v)}
	if v.Blocks != nil {
		out.Blocks = v.Blocks
	}
	return json.Marshal(out)
}

// PageServiceDeps are the collaborators of PageServiceImpl.
type PageServiceDeps struct {
	Pages repository.PageRepository
	Views repository.PageViewRepository
	// Links validates link blocks and builds their summaries.
	Links repository.LinkRepository
	// LinkSvc creates the link of a block added with new_link.
	LinkSvc LinkService
	// Domains and BaseURL build short URLs.
	Domains  repository.DomainRepository
	Activity ActivityService
	BaseURL  string
}

type PageServiceImpl struct {
	pages    repository.PageRepository
	views    repository.PageViewRepository
	links    repository.LinkRepository
	linkSvc  LinkService
	domains  repository.DomainRepository
	activity ActivityService
	baseURL  string
	now      func() time.Time
}

func NewPageService(d PageServiceDeps) *PageServiceImpl {
	return &PageServiceImpl{
		pages: d.Pages, views: d.Views, links: d.Links, linkSvc: d.LinkSvc,
		domains: d.Domains, activity: d.Activity,
		baseURL: strings.TrimRight(d.BaseURL, "/"),
		now:     time.Now,
	}
}

// SetClock overrides the clock (tests).
func (s *PageServiceImpl) SetClock(now func() time.Time) { s.now = now }

// normalizeSlug lowercases and trims slug and reports why it cannot be
// claimed ("" when it is well-formed and not reserved).
func normalizeSlug(slug string) (string, string) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if len(slug) < pageSlugMin || len(slug) > pageSlugMax || !pageSlugRe.MatchString(slug) {
		return slug, SlugReasonInvalid
	}
	if ReservedPageSlugs[slug] {
		return slug, SlugReasonReserved
	}
	return slug, ""
}

// ValidPageSlug returns the normalized slug and whether it is well-formed
// and not reserved (no availability check).
func ValidPageSlug(slug string) (string, bool) {
	norm, reason := normalizeSlug(slug)
	return norm, reason == ""
}

func (s *PageServiceImpl) CheckSlug(ctx context.Context, slug string) (SlugCheck, error) {
	norm, reason := normalizeSlug(slug)
	out := SlugCheck{Slug: norm, Reason: reason}
	if reason == "" {
		taken, err := s.pages.SlugExists(ctx, norm)
		if err != nil {
			return SlugCheck{}, err
		}
		if taken {
			out.Reason = SlugReasonTaken
		}
	}
	out.Available = out.Reason == ""
	return out, nil
}

func (s *PageServiceImpl) Create(ctx context.Context, userID, groupID uint64, in CreatePageInput) (*PageView, error) {
	slug, reason := normalizeSlug(in.Slug)
	switch reason {
	case SlugReasonInvalid:
		return nil, invalid("slug must be %d-%d lowercase letters, digits or hyphens, not starting or ending with a hyphen", pageSlugMin, pageSlugMax)
	case SlugReasonReserved:
		return nil, invalid("slug %q is reserved", slug)
	}
	template := strings.TrimSpace(in.Template)
	if template == "" {
		template = "scratch"
	}
	if !PageTemplates[template] {
		return nil, invalid("unknown template %q", in.Template)
	}
	title := strings.TrimSpace(in.Title)
	if utf8.RuneCountInString(title) > pageTitleMax {
		return nil, invalid("title must be at most %d characters", pageTitleMax)
	}

	p := &model.Page{
		UserID: userID, GroupID: groupID, Slug: slug, Title: title,
		Design: DefaultDesign(template), Status: model.PageStatusDraft,
	}
	if err := s.pages.Create(ctx, p); err != nil {
		return nil, err
	}
	if template != "scratch" {
		seed := &model.PageBlock{
			PageID: p.ID, Type: model.PageBlockText, IsActive: true,
			Content: model.JSONMap{"heading": "Welcome", "body": "This is your new page"},
		}
		if err := s.pages.CreateBlock(ctx, seed); err != nil {
			logger.Error(ctx, "page-service: failed to seed template block", zap.Uint64("page_id", p.ID), zap.Error(err))
			return nil, err
		}
	}
	s.activity.Log(ctx, userID, groupID, ActionPageCreate, EntityPage, p.ID, map[string]any{"slug": slug, "template": template})
	return s.view(ctx, p)
}

func (s *PageServiceImpl) List(ctx context.Context, groupID uint64, f repository.PageFilter) ([]PageView, error) {
	rows, err := s.pages.ListByGroup(ctx, groupID, f)
	if err != nil {
		return nil, err
	}
	out := make([]PageView, 0, len(rows))
	for i := range rows {
		p := &rows[i].Page
		var maxBlock *time.Time
		if p.Status != model.PageStatusDraft {
			if maxBlock, err = s.pages.MaxBlockUpdatedAt(ctx, p.ID); err != nil {
				return nil, err
			}
		}
		v := s.basicView(p, rows[i].Views)
		v.HasUnpublishedChanges = HasUnpublishedChanges(p, maxBlock)
		out = append(out, v)
	}
	return out, nil
}

// getPage loads the page and checks its group; ErrPageNotFound otherwise.
func (s *PageServiceImpl) getPage(ctx context.Context, groupID, id uint64) (*model.Page, error) {
	p, err := s.pages.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.GroupID != groupID {
		return nil, repository.ErrPageNotFound
	}
	return p, nil
}

func (s *PageServiceImpl) Get(ctx context.Context, groupID, id uint64) (*PageView, error) {
	p, err := s.getPage(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, p)
}

func (s *PageServiceImpl) Update(ctx context.Context, userID, groupID, id uint64, in UpdatePageInput) (*PageView, error) {
	p, err := s.getPage(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	var fields []string
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if utf8.RuneCountInString(t) > pageTitleMax {
			return nil, invalid("title must be at most %d characters", pageTitleMax)
		}
		p.Title = t
		fields = append(fields, "title")
	}
	if in.Description != nil {
		d := strings.TrimSpace(*in.Description)
		if utf8.RuneCountInString(d) > pageDescriptionMax {
			return nil, invalid("description must be at most %d characters", pageDescriptionMax)
		}
		p.Description = d
		fields = append(fields, "description")
	}
	if in.AvatarURL != nil {
		a := strings.TrimSpace(*in.AvatarURL)
		if a != "" && (len(a) > pageAvatarURLMax || !isImageURL(a)) {
			return nil, invalid("avatar_url must be an http(s) URL or an /uploads/ path of at most %d characters", pageAvatarURLMax)
		}
		p.AvatarURL = a
		fields = append(fields, "avatar_url")
	}
	if in.Design != nil {
		d, err := normalizeDesign(in.Design)
		if err != nil {
			return nil, err
		}
		p.Design = d
		fields = append(fields, "design")
	}
	if len(fields) > 0 {
		if err := s.pages.Update(ctx, p); err != nil {
			return nil, err
		}
		s.activity.Log(ctx, userID, groupID, ActionPageUpdate, EntityPage, p.ID, map[string]any{"fields": fields})
	}
	return s.Get(ctx, groupID, id)
}

func (s *PageServiceImpl) SetArchived(ctx context.Context, userID, groupID, id uint64, archived bool) (*PageView, error) {
	p, err := s.getPage(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	if err := s.pages.SetArchived(ctx, id, archived); err != nil {
		return nil, err
	}
	action := ActionPageUnhide
	if archived {
		action = ActionPageHide
	}
	s.activity.Log(ctx, userID, groupID, action, EntityPage, id, map[string]any{"slug": p.Slug})
	p.Archived = archived
	return s.view(ctx, p)
}

func (s *PageServiceImpl) Delete(ctx context.Context, userID, groupID, id uint64) error {
	p, err := s.getPage(ctx, groupID, id)
	if err != nil {
		return err
	}
	if err := s.pages.Delete(ctx, id); err != nil {
		return err
	}
	s.activity.Log(ctx, userID, groupID, ActionPageDelete, EntityPage, id, map[string]any{"slug": p.Slug})
	return nil
}

// PageShortURL appends the page click source to a short URL.
func PageShortURL(shortURL string) string {
	if strings.Contains(shortURL, "?") {
		return shortURL + "&" + pageViewSource
	}
	return shortURL + "?" + pageViewSource
}

func (s *PageServiceImpl) Publish(ctx context.Context, userID, groupID, id uint64) (*PageView, error) {
	p, err := s.getPage(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	blocks, err := s.pages.ListBlocks(ctx, id)
	if err != nil {
		return nil, err
	}

	// published_at is never earlier than the last edit (DB clock skew), so a
	// fresh publish is never reported as having unpublished changes.
	at := s.now().UTC()
	if p.UpdatedAt.UTC().After(at) {
		at = p.UpdatedAt.UTC()
	}
	active := make([]model.PageBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.UpdatedAt.UTC().After(at) {
			at = b.UpdatedAt.UTC()
		}
		if b.IsActive {
			active = append(active, b)
		}
	}
	at = at.Truncate(time.Second)
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].IsPinned != active[j].IsPinned {
			return active[i].IsPinned
		}
		return active[i].Position < active[j].Position
	})

	links, err := s.blockLinks(ctx, active)
	if err != nil {
		return nil, err
	}
	snapBlocks := make([]any, 0, len(active))
	for _, b := range active {
		sb := map[string]any{
			"id": b.ID, "type": b.Type, "content": map[string]any(b.Content), "is_pinned": b.IsPinned,
		}
		if b.Type == model.PageBlockLink {
			if b.LinkID == nil {
				continue
			}
			l, ok := links[*b.LinkID]
			if !ok {
				continue // link deleted meanwhile; the cascade removes the block
			}
			sb["short_url"] = PageShortURL(l.ShortURL)
		}
		snapBlocks = append(snapBlocks, sb)
	}
	snapshot := model.JSONMap{
		"slug": p.Slug, "title": p.Title, "description": p.Description, "avatar_url": p.AvatarURL,
		"design": map[string]any(p.Design), "published_at": at.Format(time.RFC3339), "blocks": snapBlocks,
	}
	if err := s.pages.Publish(ctx, id, snapshot, at); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionPagePublish, EntityPage, id, map[string]any{"slug": p.Slug, "blocks": len(snapBlocks)})
	return s.Get(ctx, groupID, id)
}

func (s *PageServiceImpl) AddBlock(ctx context.Context, userID, groupID, pageID uint64, in BlockInput) (*BlockView, error) {
	p, err := s.getPage(ctx, groupID, pageID)
	if err != nil {
		return nil, err
	}
	content, err := normalizeBlockContent(in.Type, in.Content)
	if err != nil {
		return nil, err
	}
	b := &model.PageBlock{PageID: p.ID, Type: in.Type, Content: content, IsActive: true}
	var link *model.Link
	if in.Type == model.PageBlockLink {
		switch {
		case in.LinkID != nil && in.NewLink != nil:
			return nil, invalid("give either link_id or new_link, not both")
		case in.LinkID == nil && in.NewLink == nil:
			return nil, invalid("a link block needs link_id or new_link")
		case in.LinkID != nil:
			found, err := s.links.GetByIDs(ctx, []uint64{*in.LinkID})
			if err != nil {
				return nil, err
			}
			if len(found) != 1 || found[0].GroupID != p.GroupID || found[0].Source != model.LinkSourceLink {
				return nil, invalid("link %d is not a link of this group", *in.LinkID)
			}
			link = &found[0]
		default:
			dest := strings.TrimSpace(in.NewLink.OriginalURL)
			if dest == "" {
				return nil, invalid("new_link.original_url is required")
			}
			title := strings.TrimSpace(in.NewLink.Title)
			if title == "" {
				title, _ = content["title"].(string)
			}
			created, err := s.linkSvc.Create(ctx, userID, CreateLinkInput{
				OriginalURL: dest, Title: title, GroupID: p.GroupID, Source: model.LinkSourceLink,
			})
			if err != nil {
				return nil, err
			}
			link = created
		}
		b.LinkID = &link.ID
	} else if in.LinkID != nil || in.NewLink != nil {
		return nil, invalid("only link blocks take a link")
	}
	if err := s.pages.CreateBlock(ctx, b); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionPageUpdate, EntityPage, p.ID, map[string]any{"block_id": b.ID, "op": "block_add", "type": b.Type})
	bv := toBlockView(*b, nil)
	if link != nil {
		summary := s.linkSummary(ctx, link, map[uint64]string{})
		bv.Link = &summary
	}
	return &bv, nil
}

// getBlock loads the page (group-checked) and the block, which must belong
// to the page; ErrBlockNotFound otherwise.
func (s *PageServiceImpl) getBlock(ctx context.Context, groupID, pageID, blockID uint64) (*model.PageBlock, error) {
	if _, err := s.getPage(ctx, groupID, pageID); err != nil {
		return nil, err
	}
	b, err := s.pages.GetBlock(ctx, blockID)
	if err != nil {
		return nil, err
	}
	if b.PageID != pageID {
		return nil, repository.ErrBlockNotFound
	}
	return b, nil
}

func (s *PageServiceImpl) UpdateBlock(ctx context.Context, userID, groupID, pageID, blockID uint64, in BlockPatch) (*BlockView, error) {
	b, err := s.getBlock(ctx, groupID, pageID, blockID)
	if err != nil {
		return nil, err
	}
	if in.Content != nil {
		c, err := normalizeBlockContent(b.Type, in.Content)
		if err != nil {
			return nil, err
		}
		b.Content = c
	}
	if in.IsActive != nil {
		b.IsActive = *in.IsActive
	}
	if in.IsPinned != nil {
		b.IsPinned = *in.IsPinned
	}
	if err := s.pages.UpdateBlock(ctx, b); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionPageUpdate, EntityPage, pageID, map[string]any{"block_id": b.ID, "op": "block_update"})
	views, err := s.blockViews(ctx, []model.PageBlock{*b})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (s *PageServiceImpl) DeleteBlock(ctx context.Context, userID, groupID, pageID, blockID uint64) error {
	b, err := s.getBlock(ctx, groupID, pageID, blockID)
	if err != nil {
		return err
	}
	if err := s.pages.DeleteBlock(ctx, b.ID); err != nil {
		return err
	}
	s.activity.Log(ctx, userID, groupID, ActionPageUpdate, EntityPage, pageID, map[string]any{"block_id": b.ID, "op": "block_delete"})
	return nil
}

func (s *PageServiceImpl) Reorder(ctx context.Context, userID, groupID, pageID uint64, ids []uint64) ([]BlockView, error) {
	if _, err := s.getPage(ctx, groupID, pageID); err != nil {
		return nil, err
	}
	blocks, err := s.pages.ListBlocks(ctx, pageID)
	if err != nil {
		return nil, err
	}
	errOrder := invalid("ids must list each of the page's blocks exactly once")
	if len(ids) != len(blocks) {
		return nil, errOrder
	}
	want := make(map[uint64]bool, len(blocks))
	for _, b := range blocks {
		want[b.ID] = true
	}
	for _, id := range ids {
		if !want[id] {
			return nil, errOrder
		}
		delete(want, id) // a duplicate is then missing from want
	}
	if err := s.pages.ReorderBlocks(ctx, pageID, ids); err != nil {
		if errors.Is(err, repository.ErrInvalidBlockOrder) {
			return nil, errOrder
		}
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionPageUpdate, EntityPage, pageID, map[string]any{"op": "block_reorder"})
	blocks, err = s.pages.ListBlocks(ctx, pageID)
	if err != nil {
		return nil, err
	}
	return s.blockViews(ctx, blocks)
}

func (s *PageServiceImpl) PageIDsForLinks(ctx context.Context, linkIDs []uint64) (map[uint64][]uint64, error) {
	if len(linkIDs) == 0 {
		return map[uint64][]uint64{}, nil
	}
	return s.pages.PagesForLinks(ctx, linkIDs)
}

// HasUnpublishedChanges reports whether the editor state differs from the
// published snapshot: always for drafts, else when the page or a block was
// updated after published_at (UTC; equal instants are not changes).
func HasUnpublishedChanges(p *model.Page, maxBlockUpdatedAt *time.Time) bool {
	if p.Status != model.PageStatusPublished || p.PublishedAt == nil {
		return true
	}
	pub := p.PublishedAt.UTC()
	if p.UpdatedAt.UTC().After(pub) {
		return true
	}
	return maxBlockUpdatedAt != nil && maxBlockUpdatedAt.UTC().After(pub)
}

// basicView converts the page without blocks.
func (s *PageServiceImpl) basicView(p *model.Page, views int64) PageView {
	design := p.Design
	if design == nil {
		design = model.JSONMap{}
	}
	return PageView{
		ID: p.ID, GroupID: p.GroupID, Slug: p.Slug, Title: p.Title, Description: p.Description,
		AvatarURL: p.AvatarURL, Design: design, Status: p.Status, PublishedAt: p.PublishedAt,
		Archived: p.Archived, URL: s.baseURL + "/m/" + p.Slug, Views: views,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// view builds the full page view with blocks, link summaries and views.
func (s *PageServiceImpl) view(ctx context.Context, p *model.Page) (*PageView, error) {
	blocks, err := s.pages.ListBlocks(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	bvs, err := s.blockViews(ctx, blocks)
	if err != nil {
		return nil, err
	}
	views, err := s.views.CountAllTime(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	var maxBlock *time.Time
	for i := range blocks {
		if maxBlock == nil || blocks[i].UpdatedAt.After(*maxBlock) {
			maxBlock = &blocks[i].UpdatedAt
		}
	}
	v := s.basicView(p, views)
	v.HasUnpublishedChanges = HasUnpublishedChanges(p, maxBlock)
	v.Blocks = bvs
	return &v, nil
}

// blockViews converts blocks with their link summaries; never nil.
func (s *PageServiceImpl) blockViews(ctx context.Context, blocks []model.PageBlock) ([]BlockView, error) {
	links, err := s.blockLinks(ctx, blocks)
	if err != nil {
		return nil, err
	}
	out := make([]BlockView, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, toBlockView(b, links))
	}
	return out, nil
}

// toBlockView converts the block, attaching its link summary from links.
func toBlockView(b model.PageBlock, links map[uint64]BlockLink) BlockView {
	content := b.Content
	if content == nil {
		content = model.JSONMap{}
	}
	bv := BlockView{
		ID: b.ID, Type: b.Type, Position: b.Position, Content: content,
		IsActive: b.IsActive, IsPinned: b.IsPinned, LinkID: b.LinkID,
	}
	if b.LinkID != nil {
		if l, ok := links[*b.LinkID]; ok {
			bv.Link = &l
		}
	}
	return bv
}

// blockLinks loads the summaries of the blocks' links by link id.
func (s *PageServiceImpl) blockLinks(ctx context.Context, blocks []model.PageBlock) (map[uint64]BlockLink, error) {
	out := map[uint64]BlockLink{}
	var ids []uint64
	seen := map[uint64]bool{}
	for _, b := range blocks {
		if b.LinkID != nil && !seen[*b.LinkID] {
			seen[*b.LinkID] = true
			ids = append(ids, *b.LinkID)
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	links, err := s.links.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	domains := map[uint64]string{}
	for i := range links {
		out[links[i].ID] = s.linkSummary(ctx, &links[i], domains)
	}
	return out, nil
}

// linkSummary summarizes l; the title falls back to the short code.
func (s *PageServiceImpl) linkSummary(ctx context.Context, l *model.Link, domains map[uint64]string) BlockLink {
	title := l.ShortCode
	if l.Title != nil && strings.TrimSpace(*l.Title) != "" {
		title = *l.Title
	}
	return BlockLink{
		ID: l.ID, Title: title, OriginalURL: l.OriginalURL,
		ShortURL: ShortURL(s.baseURL, l, s.domainName(ctx, l, domains)),
	}
}

// domainName resolves the link's custom domain, caching lookups in cache;
// a failed lookup falls back to the base URL.
func (s *PageServiceImpl) domainName(ctx context.Context, link *model.Link, cache map[uint64]string) string {
	if link.DomainID == nil {
		return ""
	}
	name, ok := cache[*link.DomainID]
	if !ok {
		d, err := s.domains.GetByID(ctx, *link.DomainID)
		if err != nil {
			logger.Warn(ctx, "page-service: failed to load link domain",
				zap.Uint64("domain_id", *link.DomainID), zap.Error(err))
		} else {
			name = d.Domain
		}
		cache[*link.DomainID] = name
	}
	return name
}

// --- validation ---

// ValidateDesign checks a page design (Shared contracts: theme, background,
// text_color, button, font); *ValidationError when invalid.
func ValidateDesign(d map[string]any) error {
	_, err := normalizeDesign(d)
	return err
}

// normalizeDesign validates d and returns it with only the known keys.
func normalizeDesign(d map[string]any) (model.JSONMap, error) {
	if d == nil {
		return nil, invalid("design is required")
	}
	theme, err := enumField(d, "design.theme", "theme", designThemes)
	if err != nil {
		return nil, err
	}
	bg, ok := d["background"].(map[string]any)
	if !ok {
		return nil, invalid("design.background must be an object")
	}
	bgType, err := enumField(bg, "design.background.type", "type", designBackground)
	if err != nil {
		return nil, err
	}
	bgColor, err := hexField(bg, "design.background.color", "color")
	if err != nil {
		return nil, err
	}
	outBG := map[string]any{"type": bgType, "color": bgColor}
	if _, has := bg["color2"]; has || bgType == "gradient" {
		c2, err := hexField(bg, "design.background.color2", "color2")
		if err != nil {
			return nil, err
		}
		outBG["color2"] = c2
	}
	text, err := hexField(d, "design.text_color", "text_color")
	if err != nil {
		return nil, err
	}
	btn, ok := d["button"].(map[string]any)
	if !ok {
		return nil, invalid("design.button must be an object")
	}
	style, err := enumField(btn, "design.button.style", "style", designButtons)
	if err != nil {
		return nil, err
	}
	radius, err := enumField(btn, "design.button.radius", "radius", designRadii)
	if err != nil {
		return nil, err
	}
	btnColor, err := hexField(btn, "design.button.color", "color")
	if err != nil {
		return nil, err
	}
	btnText, err := hexField(btn, "design.button.text_color", "text_color")
	if err != nil {
		return nil, err
	}
	font, err := enumField(d, "design.font", "font", designFonts)
	if err != nil {
		return nil, err
	}
	return model.JSONMap{
		"theme": theme, "background": outBG, "text_color": text,
		"button": map[string]any{"style": style, "radius": radius, "color": btnColor, "text_color": btnText},
		"font":   font,
	}, nil
}

func enumField(m map[string]any, name, key string, allowed map[string]bool) (string, error) {
	v, ok := m[key].(string)
	if !ok || !allowed[v] {
		keys := make([]string, 0, len(allowed))
		for k := range allowed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return "", invalid("%s must be one of %s", name, strings.Join(keys, ", "))
	}
	return v, nil
}

func hexField(m map[string]any, name, key string) (string, error) {
	v, ok := m[key].(string)
	if !ok || !hexColorRe.MatchString(v) {
		return "", invalid("%s must be a #RRGGBB colour", name)
	}
	return v, nil
}

// ValidateBlockContent checks a block's content for its type (Shared
// contracts); *ValidationError when invalid.
func ValidateBlockContent(blockType string, content map[string]any) error {
	_, err := normalizeBlockContent(blockType, content)
	return err
}

// normalizeBlockContent validates content and returns it trimmed, with only
// the known keys of the type.
func normalizeBlockContent(blockType string, content map[string]any) (model.JSONMap, error) {
	switch blockType {
	case model.PageBlockLink, model.PageBlockText, model.PageBlockImage, model.PageBlockSocial:
	default:
		return nil, invalid("block type must be one of link, text, image, social")
	}
	if content == nil {
		return nil, invalid("content is required")
	}
	switch blockType {
	case model.PageBlockLink:
		title, err := textField(content, "title", blockLinkTitleMax)
		if err != nil {
			return nil, err
		}
		if title == "" {
			return nil, invalid("title is required")
		}
		desc, err := textField(content, "description", blockLinkDescMax)
		if err != nil {
			return nil, err
		}
		thumb, err := textField(content, "thumbnail_url", pageAvatarURLMax)
		if err != nil {
			return nil, err
		}
		if thumb != "" && !isHTTPURL(thumb) {
			return nil, invalid("thumbnail_url must be an http(s) URL")
		}
		return model.JSONMap{"title": title, "description": desc, "thumbnail_url": thumb}, nil
	case model.PageBlockText:
		heading, err := textField(content, "heading", blockTextHeadingMax)
		if err != nil {
			return nil, err
		}
		body, err := textField(content, "body", blockTextBodyMax)
		if err != nil {
			return nil, err
		}
		if heading == "" && body == "" {
			return nil, invalid("a text block needs a heading or a body")
		}
		return model.JSONMap{"heading": heading, "body": body}, nil
	case model.PageBlockImage:
		u, err := textField(content, "url", pageAvatarURLMax)
		if err != nil {
			return nil, err
		}
		if !isImageURL(u) {
			return nil, invalid("url must be an http(s) URL or an /uploads/ path")
		}
		alt, err := textField(content, "alt", blockImageAltMax)
		if err != nil {
			return nil, err
		}
		return model.JSONMap{"url": u, "alt": alt}, nil
	default: // social
		var raw []map[string]any
		switch items := content["items"].(type) {
		case []any:
			for _, it := range items {
				m, ok := it.(map[string]any)
				if !ok {
					return nil, invalid("items must be objects with platform and url")
				}
				raw = append(raw, m)
			}
		case []map[string]any:
			raw = items
		default:
			return nil, invalid("items must be a list")
		}
		if len(raw) < 1 || len(raw) > blockSocialItemsMax {
			return nil, invalid("a social block needs 1-%d items", blockSocialItemsMax)
		}
		out := make([]any, 0, len(raw))
		for _, it := range raw {
			platform, ok := it["platform"].(string)
			if !ok || !socialPlatforms[platform] {
				return nil, invalid("platform must be one of instagram, x, facebook, tiktok, youtube, linkedin, email, website")
			}
			u, err := textField(it, "url", pageAvatarURLMax)
			if err != nil {
				return nil, err
			}
			if !isHTTPURL(u) && (platform != "email" || !isMailto(u)) {
				return nil, invalid("%s url must be an http(s) URL", platform)
			}
			out = append(out, map[string]any{"platform": platform, "url": u})
		}
		return model.JSONMap{"items": out}, nil
	}
}

// textField returns the trimmed string at key ("" when absent) of at most
// max characters.
func textField(m map[string]any, key string, max int) (string, error) {
	raw, has := m[key]
	if !has || raw == nil {
		return "", nil
	}
	v, ok := raw.(string)
	if !ok {
		return "", invalid("%s must be a string", key)
	}
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > max {
		return "", invalid("%s must be at most %d characters", key, max)
	}
	return v, nil
}

func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// isImageURL accepts http(s) URLs and paths served by the uploads endpoint.
func isImageURL(s string) bool {
	if strings.HasPrefix(s, "/uploads/") {
		return !strings.Contains(s, "..") && len(s) > len("/uploads/")
	}
	return isHTTPURL(s)
}

func isMailto(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "mailto" && strings.Contains(u.Opaque, "@")
}
