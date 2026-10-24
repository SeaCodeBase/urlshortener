package handler

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/qrimage"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const msgQRCodeNotFound = "QR code not found"

// QRHandler serves /api/qr-codes.
type QRHandler struct {
	linkPresenter
	qrService   service.QRService
	invalidator LinkCacheInvalidator
	stats       service.StatsService
	now         func() time.Time
}

func NewQRHandler(qrService service.QRService, invalidator LinkCacheInvalidator, domainRepo repository.DomainRepository, stats service.StatsService, cfg *config.Config) *QRHandler {
	return &QRHandler{
		linkPresenter: newLinkPresenter(cfg, domainRepo),
		qrService:     qrService,
		invalidator:   invalidator,
		stats:         stats,
		now:           time.Now,
	}
}

type qrResponse struct {
	ID                   uint64         `json:"id"`
	UserID               uint64         `json:"user_id"`
	GroupID              uint64         `json:"group_id"`
	LinkID               uint64         `json:"link_id"`
	Title                *string        `json:"title"`
	RenderCustomizations model.JSONMap  `json:"render_customizations"`
	Archived             bool           `json:"archived"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	Tags                 []string       `json:"tags"`
	TotalScans           int64          `json:"total_scans"`
	ShortURL             string         `json:"short_url"`
	QRURL                string         `json:"qr_url"`
	OriginalURL          string         `json:"original_url"`
	FaviconURL           string         `json:"favicon_url"`
	ExpiresAt            model.NullTime `json:"expires_at"`
	LinkSource           string         `json:"link_source"`
	ImageURL             string         `json:"image_url"`
	Link                 *linkResponse  `json:"link,omitempty"`
}

type listQRResponse struct {
	QRCodes    []qrResponse `json:"qr_codes"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	TotalPages int          `json:"total_pages"`
}

func (h *QRHandler) toQRResponse(d *service.QRCodeDetails, domainMap map[uint64]string) qrResponse {
	title := d.Title
	if title == nil {
		title = d.Link.Title
	}
	tags := d.Tags
	if tags == nil {
		tags = []string{}
	}
	rc := d.RenderCustomizations
	if rc == nil {
		rc = model.JSONMap{}
	}
	shortURL := h.buildShortURL(&d.Link, domainMap)
	return qrResponse{
		ID:                   d.ID,
		UserID:               d.UserID,
		GroupID:              d.GroupID,
		LinkID:               d.LinkID,
		Title:                title,
		RenderCustomizations: rc,
		Archived:             d.Archived,
		CreatedAt:            d.CreatedAt,
		UpdatedAt:            d.UpdatedAt,
		Tags:                 tags,
		TotalScans:           d.TotalScans,
		ShortURL:             shortURL,
		QRURL:                qrURL(shortURL),
		OriginalURL:          d.Link.OriginalURL,
		FaviconURL:           h.faviconURL(&d.Link),
		ExpiresAt:            d.Link.ExpiresAt,
		LinkSource:           d.Link.Source,
		ImageURL:             "/api/qr-codes/" + strconv.FormatUint(d.ID, 10) + "/image",
	}
}

// qrURL is the URL encoded in the QR image; ?r=qr marks scans.
func qrURL(shortURL string) string {
	return shortURL + "?r=qr"
}

// toDetailResponse adds the full link (with routing rules) to the response.
// The link's own tags and clicks are not loaded here.
func (h *QRHandler) toDetailResponse(d *service.QRCodeDetails, domainMap map[uint64]string) qrResponse {
	resp := h.toQRResponse(d, domainMap)
	link := h.toResponse(&d.Link, nil, 0, domainMap)
	rules := d.RoutingRules
	if rules == nil {
		rules = []model.RoutingRule{}
	}
	link.RoutingRules = &rules
	qrID := d.ID
	link.QRCodeID = &qrID
	resp.Link = &link
	return resp
}

// respondDetails reloads the QR code (for routing rules and scans) and
// renders it; if that fails, d is rendered as is.
func (h *QRHandler) respondDetails(c *gin.Context, status int, userID uint64, d *service.QRCodeDetails, domainMap map[uint64]string) {
	ctx := c.Request.Context()
	if domainMap == nil {
		domainMap = h.loadDomainMap(ctx, userID)
	}
	full, err := h.qrService.Get(ctx, userID, d.ID)
	if err != nil {
		logger.Warn(ctx, "qr-handler: failed to reload qr code",
			zap.Uint64("qr_code_id", d.ID),
			zap.Error(err),
		)
		full = d
	}
	c.JSON(status, h.toDetailResponse(full, domainMap))
}

// invalidate drops the cached redirects of the links (best effort).
func (h *QRHandler) invalidate(c *gin.Context, domainMap map[uint64]string, links ...*model.Link) {
	ctx := c.Request.Context()
	for _, link := range links {
		domainName := ""
		if link.DomainID != nil {
			domainName = domainMap[*link.DomainID]
		}
		if err := h.invalidator.InvalidateLink(ctx, link, domainName); err != nil {
			logger.Warn(ctx, "qr-handler: failed to invalidate cache",
				zap.Uint64("link_id", link.ID),
				zap.String("short_code", link.ShortCode),
				zap.String("domain", domainName),
				zap.Error(err),
			)
		}
	}
}

// writeQRServiceError maps QR service errors to responses and falls back to
// writeServiceError for link errors. It reports whether err was handled.
func writeQRServiceError(c *gin.Context, op string, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, service.ErrQRCodeNotFound), errors.Is(err, service.ErrLinkNotFound), errors.Is(err, service.ErrNotLinkOwner):
		c.JSON(http.StatusNotFound, gin.H{"error": msgQRCodeNotFound})
	case errors.Is(err, service.ErrQRCodeExistsForLink):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrQRLinkRequired), errors.Is(err, service.ErrInvalidRenderCustomizations):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		return writeServiceError(c, op, err)
	}
	logger.Warn(c.Request.Context(), op+": rejected", zap.Error(err))
	return true
}

func parseQRID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid QR code ID"})
		return 0, false
	}
	return id, true
}

// parseQRFilter reads the list/export query parameters. It writes a 400
// response and returns false on invalid input.
func parseQRFilter(c *gin.Context) (repository.QRFilter, bool) {
	var f repository.QRFilter
	bad := func(msg string) (repository.QRFilter, bool) {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return repository.QRFilter{}, false
	}

	f.Query = strings.TrimSpace(c.Query("q"))
	f.Tags = parseTagsQuery(c)

	var ok bool
	if f.Archived, ok = parseOnOff(c, "archived", "off", true); !ok {
		return bad("archived must be on or off")
	}
	if f.HasLink, ok = parseOnOff(c, "has_link", "both", true); !ok {
		return bad("has_link must be on, off or both")
	}

	switch e := c.Query("expiration"); e {
	case "", repository.ExpirationExpired, repository.ExpirationExpiring, repository.ExpirationNone:
		f.Expiration = e
	default:
		return bad("expiration must be expired, expiring or none")
	}

	var msg string
	if f.CreatedAfter, f.CreatedBefore, msg = parseCreatedRange(c); msg != "" {
		return bad(msg)
	}

	switch s := c.Query("sort"); s {
	case "", "created":
	case repository.SortScans:
		f.Sort = s
	default:
		return bad("sort must be created or scans")
	}
	return f, true
}

// List handles GET /qr-codes.
func (h *QRHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	groupID := middleware.GetGroupID(c)

	filter, ok := parseQRFilter(c)
	if !ok {
		return
	}
	page, limit, ok := parsePaging(c)
	if !ok {
		return
	}

	result, err := h.qrService.List(ctx, groupID, service.ListQRParams{Filter: filter, Page: page, Limit: limit})
	if writeQRServiceError(c, "list-qr-codes", err) {
		return
	}
	domainMap := h.loadDomainMap(ctx, userID)
	items := make([]qrResponse, len(result.QRCodes))
	for i := range result.QRCodes {
		items[i] = h.toQRResponse(&result.QRCodes[i], domainMap)
	}
	c.JSON(http.StatusOK, listQRResponse{
		QRCodes:    items,
		Total:      result.Total,
		Page:       result.Page,
		TotalPages: result.TotalPages,
	})
}

// Create handles POST /qr-codes.
func (h *QRHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	var input service.CreateQRCodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "create-qr-code: invalid request body",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.GroupID = middleware.GetGroupID(c)

	d, err := h.qrService.Create(ctx, userID, input)
	if writeQRServiceError(c, "create-qr-code", err) {
		return
	}
	h.respondDetails(c, http.StatusCreated, userID, d, nil)
}

// Get handles GET /qr-codes/:id.
func (h *QRHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	id, ok := parseQRID(c)
	if !ok {
		return
	}
	d, err := h.qrService.Get(ctx, userID, id)
	if writeQRServiceError(c, "get-qr-code", err) {
		return
	}
	c.JSON(http.StatusOK, h.toDetailResponse(d, h.loadDomainMap(ctx, userID)))
}

// Update handles PUT /qr-codes/:id and drops the link's cached redirect.
func (h *QRHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	id, ok := parseQRID(c)
	if !ok {
		return
	}
	var input service.UpdateQRCodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "update-qr-code: invalid request body",
			zap.Uint64("qr_code_id", id),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	d, prev, err := h.qrService.Update(ctx, userID, id, input)
	if err != nil {
		// The link may have changed before a later step failed.
		if prev != nil {
			h.invalidate(c, h.loadDomainMap(ctx, userID), prev)
		}
		writeQRServiceError(c, "update-qr-code", err)
		return
	}

	domainMap := h.loadDomainMap(ctx, userID)
	h.invalidate(c, domainMap, &d.Link)
	// A domain change leaves the old custom-domain key behind.
	if prev != nil && prev.DomainID != nil && (d.Link.DomainID == nil || *d.Link.DomainID != *prev.DomainID) {
		h.invalidate(c, domainMap, prev)
	}
	h.respondDetails(c, http.StatusOK, userID, d, domainMap)
}

// Delete handles DELETE /qr-codes/:id. The link's cached redirect is dropped
// whether the link was deleted with the QR code or kept.
func (h *QRHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	id, ok := parseQRID(c)
	if !ok {
		return
	}
	d, _, err := h.qrService.Delete(ctx, userID, id)
	if writeQRServiceError(c, "delete-qr-code", err) {
		return
	}
	h.invalidate(c, h.loadDomainMap(ctx, userID), &d.Link)
	c.Status(http.StatusNoContent)
}

// Bulk handles PATCH /qr-codes/bulk {action: archive|unarchive|edit_tags,
// ids, add_tags, remove_tags}. Every id must belong to the active group.
func (h *QRHandler) Bulk(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	groupID := middleware.GetGroupID(c)

	var req bulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.IDs) == 0 || len(req.IDs) > maxBulkIDs {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids must contain 1 to 1000 QR code IDs"})
		return
	}

	switch req.Action {
	case "archive", "unarchive":
		n, err := h.qrService.SetArchived(ctx, userID, groupID, req.IDs, req.Action == "archive")
		if writeQRServiceError(c, "bulk-qr-codes", err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"updated": n})
	case "edit_tags":
		if len(req.AddTags)+len(req.RemoveTags) > maxBulkTags {
			c.JSON(http.StatusBadRequest, gin.H{"error": "edit_tags accepts at most 50 tags per request"})
			return
		}
		if err := h.qrService.BulkTags(ctx, groupID, req.IDs, req.AddTags, req.RemoveTags); writeQRServiceError(c, "bulk-qr-codes", err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"updated": countDistinct(req.IDs)})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be archive, unarchive or edit_tags"})
	}
}

// SetTags handles PUT /qr-codes/:id/tags {tags}.
func (h *QRHandler) SetTags(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	id, ok := parseQRID(c)
	if !ok {
		return
	}
	var req setTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	d, _, err := h.qrService.Update(ctx, userID, id, service.UpdateQRCodeInput{Tags: &tags})
	if writeQRServiceError(c, "set-qr-tags", err) {
		return
	}
	out := d.Tags
	if out == nil {
		out = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"tags": out})
}

// Export handles GET /qr-codes/export: the filtered QR codes as a CSV
// download.
func (h *QRHandler) Export(c *gin.Context) {
	ctx := c.Request.Context()
	groupID := middleware.GetGroupID(c)

	filter, ok := parseQRFilter(c)
	if !ok {
		return
	}
	rows, err := h.qrService.Export(ctx, groupID, filter)
	if writeQRServiceError(c, "export-qr-codes", err) {
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="qr-codes.csv"`)
	c.Status(http.StatusOK)
	w := csv.NewWriter(c.Writer)
	_ = w.Write(service.QRExportHeader)
	_ = w.WriteAll(rows)
	if err := w.Error(); err != nil {
		logger.Error(ctx, "export-qr-codes: failed to write CSV",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
	}
}

// renderString returns rc[key] if it is a non-empty string, else def.
func renderString(rc model.JSONMap, key, def string) string {
	if v, ok := rc[key].(string); ok && v != "" {
		return v
	}
	return def
}

// Image handles GET /qr-codes/:id/image?format=png|svg&size=: the plain QR
// code (colors and error correction from render_customizations) for qr_url.
func (h *QRHandler) Image(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	id, ok := parseQRID(c)
	if !ok {
		return
	}
	format := c.DefaultQuery("format", "png")
	if format != "png" && format != "svg" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be png or svg"})
		return
	}
	size := qrimage.DefaultSize
	if raw := c.Query("size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < qrimage.MinSize || n > qrimage.MaxSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": qrimage.ErrInvalidSize.Error()})
			return
		}
		size = n
	}

	d, err := h.qrService.Get(ctx, userID, id)
	if writeQRServiceError(c, "qr-image", err) {
		return
	}
	content := qrURL(h.buildShortURL(&d.Link, h.loadDomainMap(ctx, userID)))
	opts := qrimage.Options{
		Size:       size,
		Foreground: renderString(d.RenderCustomizations, "color", "#000000"),
		Background: renderString(d.RenderCustomizations, "background", "#FFFFFF"),
		Level:      renderString(d.RenderCustomizations, "error_correction", "H"),
	}

	render, contentType := qrimage.PNG, "image/png"
	if format == "svg" {
		render, contentType = qrimage.SVG, "image/svg+xml"
	}
	data, err := render(content, opts)
	if err != nil {
		logger.Error(ctx, "qr-image: failed to render",
			zap.Uint64("qr_code_id", id),
			zap.String("format", format),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to render QR code"})
		return
	}
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("Content-Disposition", `inline; filename="qr-`+strconv.FormatUint(id, 10)+"."+format+`"`)
	c.Data(http.StatusOK, contentType, data)
}

// Stats handles GET /qr-codes/:id/stats: the link stats of the QR code's
// link restricted to QR scans (source is always qr).
func (h *QRHandler) Stats(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	id, ok := parseQRID(c)
	if !ok {
		return
	}
	q, err := parseStatsQuery(c, h.now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	q.Source = "qr"

	d, err := h.qrService.Get(ctx, userID, id)
	if writeQRServiceError(c, "qr-stats", err) {
		return
	}
	stats, err := h.stats.GetLinkStats(ctx, userID, d.Link.ID, q)
	var qe *service.StatsQueryError
	if errors.As(err, &qe) {
		c.JSON(http.StatusBadRequest, gin.H{"error": qe.Msg})
		return
	}
	if writeQRServiceError(c, "qr-stats", err) {
		return
	}
	c.JSON(http.StatusOK, stats)
}
