// backend/internal/handler/link.go
package handler

import (
	"context"
	"encoding/csv"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Error messages shown verbatim by the UI (spec §5.2, §5.3).
const (
	msgLinkNotFound      = "link not found"
	msgLinkNotDeletable  = "This link can't be deleted. You can hide it instead."
	msgBackHalfTaken     = "That back-half is already taken"
	msgInvalidBackHalf   = "Back-halves can only contain letters, numbers, hyphens and underscores (1-255 characters)"
	maxBulkIDs           = 1000
	faviconServiceFormat = "https://www.google.com/s2/favicons?domain=%s&sz=32"
)

// LinkCacheInvalidator drops cached redirects of a link
// (implemented by *service.RedirectService).
type LinkCacheInvalidator interface {
	InvalidateLink(ctx context.Context, link *model.Link, domainName string) error
}

type LinkHandler struct {
	linkService service.LinkService
	invalidator LinkCacheInvalidator
	domainRepo  repository.DomainRepository
	baseURL     string
	favicons    bool
}

func NewLinkHandler(linkService service.LinkService, invalidator LinkCacheInvalidator, domainRepo repository.DomainRepository, cfg *config.Config) *LinkHandler {
	return &LinkHandler{
		linkService: linkService,
		invalidator: invalidator,
		domainRepo:  domainRepo,
		baseURL:     cfg.URLs.BaseURL,
		favicons:    cfg.UI.FaviconService == nil || *cfg.UI.FaviconService,
	}
}

type historyEntry struct {
	ID        uint64    `json:"id"`
	ShortURL  string    `json:"short_url"`
	CreatedAt time.Time `json:"created_at"`
}

type linkResponse struct {
	*model.Link
	ShortURL     string               `json:"short_url"`
	Tags         []string             `json:"tags"`
	TotalClicks  int64                `json:"total_clicks"`
	FaviconURL   string               `json:"favicon_url"`
	RoutingRules *[]model.RoutingRule `json:"routing_rules,omitempty"`
	History      *[]historyEntry      `json:"history,omitempty"`
}

type listLinksResponse struct {
	Links      []linkResponse `json:"links"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	TotalPages int            `json:"total_pages"`
}

func (h *LinkHandler) buildShortURL(link *model.Link, domainMap map[uint64]string) string {
	domainName := ""
	if link.DomainID != nil {
		domainName = domainMap[*link.DomainID]
	}
	return service.ShortURL(h.baseURL, link, domainName)
}

// faviconURL returns the Google favicon URL for the destination host, or ""
// when the favicon service is disabled.
func (h *LinkHandler) faviconURL(link *model.Link) string {
	if !h.favicons {
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

func (h *LinkHandler) toResponse(link *model.Link, tags []string, clicks int64, domainMap map[uint64]string) linkResponse {
	if tags == nil {
		tags = []string{}
	}
	return linkResponse{
		Link:        link,
		ShortURL:    h.buildShortURL(link, domainMap),
		Tags:        tags,
		TotalClicks: clicks,
		FaviconURL:  h.faviconURL(link),
	}
}

func (h *LinkHandler) toDetailResponse(d *service.LinkDetails, domainMap map[uint64]string) linkResponse {
	resp := h.toResponse(&d.Link, d.Tags, d.TotalClicks, domainMap)
	rules := d.RoutingRules
	if rules == nil {
		rules = []model.RoutingRule{}
	}
	history := make([]historyEntry, len(d.History))
	for i := range d.History {
		p := &d.History[i]
		history[i] = historyEntry{ID: p.ID, ShortURL: h.buildShortURL(p, domainMap), CreatedAt: p.CreatedAt}
	}
	resp.RoutingRules = &rules
	resp.History = &history
	return resp
}

func (h *LinkHandler) toListResponse(result *service.ListLinksResult, domainMap map[uint64]string) listLinksResponse {
	links := make([]linkResponse, len(result.Links))
	for i := range result.Links {
		l := &result.Links[i]
		links[i] = h.toResponse(&l.Link, l.Tags, l.TotalClicks, domainMap)
	}
	return listLinksResponse{
		Links:      links,
		Total:      result.Total,
		Page:       result.Page,
		TotalPages: result.TotalPages,
	}
}

func (h *LinkHandler) loadDomainMap(ctx context.Context, userID uint64) map[uint64]string {
	domains, err := h.domainRepo.ListByUserID(ctx, userID)
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

// invalidate drops the cached redirects of the links (best effort).
func (h *LinkHandler) invalidate(ctx context.Context, domainMap map[uint64]string, links ...*model.Link) {
	for _, link := range links {
		domainName := ""
		if link.DomainID != nil {
			domainName = domainMap[*link.DomainID]
		}
		if err := h.invalidator.InvalidateLink(ctx, link, domainName); err != nil {
			logger.Warn(ctx, "link-handler: failed to invalidate cache",
				zap.Uint64("link_id", link.ID),
				zap.String("short_code", link.ShortCode),
				zap.String("domain", domainName),
				zap.Error(err),
			)
		}
	}
}

// respondDetails renders link with its details; if they cannot be loaded
// the plain link is returned.
func (h *LinkHandler) respondDetails(c *gin.Context, status int, userID uint64, link *model.Link, domainMap map[uint64]string) {
	ctx := c.Request.Context()
	if domainMap == nil {
		domainMap = h.loadDomainMap(ctx, userID)
	}
	d, err := h.linkService.GetDetails(ctx, userID, link.ID)
	if err != nil {
		logger.Warn(ctx, "link-handler: failed to load link details",
			zap.Uint64("link_id", link.ID),
			zap.Error(err),
		)
		c.JSON(status, h.toResponse(link, nil, 0, domainMap))
		return
	}
	c.JSON(status, h.toDetailResponse(d, domainMap))
}

// writeServiceError maps link service errors to responses. It reports
// whether err was handled (non-nil).
func writeServiceError(c *gin.Context, op string, err error) bool {
	if err == nil {
		return false
	}
	ctx := c.Request.Context()
	switch {
	case errors.Is(err, service.ErrLinkNotFound), errors.Is(err, service.ErrNotLinkOwner):
		c.JSON(http.StatusNotFound, gin.H{"error": msgLinkNotFound})
	case errors.Is(err, service.ErrLinkNotDeletable):
		c.JSON(http.StatusConflict, gin.H{"error": msgLinkNotDeletable})
	case errors.Is(err, service.ErrShortCodeTaken):
		c.JSON(http.StatusConflict, gin.H{"error": msgBackHalfTaken})
	case errors.Is(err, service.ErrInvalidShortCode):
		c.JSON(http.StatusBadRequest, gin.H{"error": msgInvalidBackHalf})
	case errors.Is(err, service.ErrInvalidURL):
		c.JSON(http.StatusBadRequest, gin.H{"error": "destination must be a valid http(s) URL of at most 2048 characters"})
	case errors.Is(err, service.ErrInvalidUTMParam):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidRoutingRule):
		c.JSON(http.StatusBadRequest, gin.H{"error": "each routing rule needs a valid http(s) destination URL"})
	case errors.Is(err, service.ErrDomainNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain not found"})
	case errors.Is(err, service.ErrDomainNotVerified):
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is not verified"})
	case errors.Is(err, service.ErrGroupRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "no active group"})
	default:
		logger.Error(ctx, op+": failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to " + strings.ReplaceAll(op, "-", " ")})
		return true
	}
	logger.Warn(ctx, op+": rejected", zap.Error(err))
	return true
}

func parseLinkID(c *gin.Context) (uint64, bool) {
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return 0, false
	}
	return linkID, true
}

func (h *LinkHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	var input service.CreateLinkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "create-link: invalid request body",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.GroupID = middleware.GetGroupID(c)

	link, err := h.linkService.Create(ctx, userID, input)
	if writeServiceError(c, "create-link", err) {
		return
	}
	h.respondDetails(c, http.StatusCreated, userID, link, nil)
}

func (h *LinkHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	linkID, ok := parseLinkID(c)
	if !ok {
		return
	}

	d, err := h.linkService.GetDetails(ctx, userID, linkID)
	if writeServiceError(c, "get-link", err) {
		return
	}
	c.JSON(http.StatusOK, h.toDetailResponse(d, h.loadDomainMap(ctx, userID)))
}

// parseOnOff parses an on|off(|both) query value into a *bool (nil = both).
func parseOnOff(c *gin.Context, name, def string, allowBoth bool) (*bool, bool) {
	switch c.DefaultQuery(name, def) {
	case "on":
		v := true
		return &v, true
	case "off":
		v := false
		return &v, true
	case "both":
		return nil, allowBoth
	}
	return nil, false
}

func parseDay(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// parseLinkFilter reads the list/export query parameters (spec §6.2). It
// writes a 400 response and returns false on invalid input.
func parseLinkFilter(c *gin.Context) (repository.LinkFilter, bool) {
	var f repository.LinkFilter
	bad := func(msg string) (repository.LinkFilter, bool) {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return repository.LinkFilter{}, false
	}

	f.Query = strings.TrimSpace(c.Query("q"))
	for _, raw := range append(c.QueryArray("tags"), c.QueryArray("tags[]")...) {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				f.Tags = append(f.Tags, t)
			}
		}
	}

	var ok bool
	if f.Archived, ok = parseOnOff(c, "archived", "off", true); !ok {
		return bad("archived must be on or off")
	}
	if f.CustomBackHalf, ok = parseOnOff(c, "custom_back_half", "both", true); !ok {
		return bad("custom_back_half must be on, off or both")
	}
	if f.HasQR, ok = parseOnOff(c, "has_qr", "both", true); !ok {
		return bad("has_qr must be on, off or both")
	}

	switch e := c.Query("expiration"); e {
	case "", repository.ExpirationExpired, repository.ExpirationExpiring, repository.ExpirationNone:
		f.Expiration = e
	default:
		return bad("expiration must be expired, expiring or none")
	}

	if raw := c.Query("campaign_id"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return bad("invalid campaign_id")
		}
		f.CampaignID = &id
	}

	after, err := parseDay(c.Query("created_after"))
	if err != nil {
		return bad("created_after must be YYYY-MM-DD")
	}
	before, err := parseDay(c.Query("created_before"))
	if err != nil {
		return bad("created_before must be YYYY-MM-DD")
	}
	if before != nil {
		// created_before is inclusive of the whole day.
		next := before.AddDate(0, 0, 1)
		before = &next
	}
	if after != nil && before != nil && !after.Before(*before) {
		return bad("created_after must not be after created_before")
	}
	f.CreatedAfter, f.CreatedBefore = after, before

	switch s := c.Query("sort"); s {
	case "", "created":
	case repository.SortClicks:
		f.Sort = s
	default:
		return bad("sort must be created or clicks")
	}
	return f, true
}

func (h *LinkHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	groupID := middleware.GetGroupID(c)

	filter, ok := parseLinkFilter(c)
	if !ok {
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page"})
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
		return
	}

	result, err := h.linkService.List(ctx, groupID, service.ListLinksParams{Filter: filter, Page: page, Limit: limit})
	if writeServiceError(c, "list-links", err) {
		return
	}
	c.JSON(http.StatusOK, h.toListResponse(result, h.loadDomainMap(ctx, userID)))
}

// Export handles GET /links/export: the filtered links as a CSV download.
func (h *LinkHandler) Export(c *gin.Context) {
	ctx := c.Request.Context()
	groupID := middleware.GetGroupID(c)

	filter, ok := parseLinkFilter(c)
	if !ok {
		return
	}
	rows, err := h.linkService.Export(ctx, groupID, filter)
	if writeServiceError(c, "export-links", err) {
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="links.csv"`)
	c.Status(http.StatusOK)
	w := csv.NewWriter(c.Writer)
	_ = w.Write(service.LinkExportHeader)
	_ = w.WriteAll(rows)
	if err := w.Error(); err != nil {
		logger.Error(ctx, "export-links: failed to write CSV",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
	}
}

func (h *LinkHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	linkID, ok := parseLinkID(c)
	if !ok {
		return
	}

	var input service.UpdateLinkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "update-link: invalid request body",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	link, prev, err := h.linkService.Update(ctx, userID, linkID, input)
	if writeServiceError(c, "update-link", err) {
		return
	}

	domainMap := h.loadDomainMap(ctx, userID)
	h.invalidate(ctx, domainMap, link)
	// A domain change leaves the old custom-domain key behind.
	if prev != nil && prev.DomainID != nil && (link.DomainID == nil || *link.DomainID != *prev.DomainID) {
		h.invalidate(ctx, domainMap, prev)
	}
	h.respondDetails(c, http.StatusOK, userID, link, domainMap)
}

func (h *LinkHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	linkID, ok := parseLinkID(c)
	if !ok {
		return
	}

	link, err := h.linkService.Delete(ctx, userID, linkID)
	if writeServiceError(c, "delete-link", err) {
		return
	}
	h.invalidate(ctx, h.loadDomainMap(ctx, userID), link)
	c.Status(http.StatusNoContent)
}

type bulkRequest struct {
	Action     string   `json:"action"`
	IDs        []uint64 `json:"ids"`
	AddTags    []string `json:"add_tags"`
	RemoveTags []string `json:"remove_tags"`
}

// Bulk handles PATCH /links/bulk {action: archive|unarchive|edit_tags, ids,
// add_tags, remove_tags}. Every id must belong to the active group.
func (h *LinkHandler) Bulk(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	groupID := middleware.GetGroupID(c)

	var req bulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.IDs) == 0 || len(req.IDs) > maxBulkIDs {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids must contain 1 to 1000 link IDs"})
		return
	}

	switch req.Action {
	case "archive", "unarchive":
		links, err := h.linkService.SetArchived(ctx, userID, groupID, req.IDs, req.Action == "archive")
		if writeServiceError(c, "bulk-links", err) {
			return
		}
		domainMap := h.loadDomainMap(ctx, userID)
		for i := range links {
			h.invalidate(ctx, domainMap, &links[i])
		}
		c.JSON(http.StatusOK, gin.H{"updated": len(links)})
	case "edit_tags":
		if err := h.linkService.BulkTags(ctx, groupID, req.IDs, req.AddTags, req.RemoveTags); writeServiceError(c, "bulk-links", err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"updated": countDistinct(req.IDs)})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be archive, unarchive or edit_tags"})
	}
}

func countDistinct(ids []uint64) int {
	seen := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return len(seen)
}

type backHalfRequest struct {
	// DomainID nil keeps the link's domain; 0 selects the default domain.
	DomainID *uint64 `json:"domain_id"`
	BackHalf string  `json:"back_half" binding:"required"`
}

// EditBackHalf handles POST /links/:id/back-half: it creates a new link with
// the back-half (the old link stays) and returns it with 201.
func (h *LinkHandler) EditBackHalf(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	linkID, ok := parseLinkID(c)
	if !ok {
		return
	}
	var req backHalfRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	link, err := h.linkService.EditBackHalf(ctx, userID, linkID, req.DomainID, req.BackHalf)
	if writeServiceError(c, "edit-back-half", err) {
		return
	}
	domainMap := h.loadDomainMap(ctx, userID)
	h.invalidate(ctx, domainMap, link)
	h.respondDetails(c, http.StatusCreated, userID, link, domainMap)
}

type setTagsRequest struct {
	Tags []string `json:"tags"`
}

// SetTags handles PUT /links/:id/tags {tags}.
func (h *LinkHandler) SetTags(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	linkID, ok := parseLinkID(c)
	if !ok {
		return
	}
	var req setTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tags, err := h.linkService.SetTags(ctx, userID, linkID, req.Tags)
	if writeServiceError(c, "set-link-tags", err) {
		return
	}
	if tags == nil {
		tags = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}
