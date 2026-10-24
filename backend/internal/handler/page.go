package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/gin-gonic/gin"
)

// PageHandler serves the link-in-bio pages of the active group.
type PageHandler struct {
	pages  service.PageService
	public service.PagePublicService
	now    func() time.Time
}

func NewPageHandler(pages service.PageService, public service.PagePublicService) *PageHandler {
	return &PageHandler{pages: pages, public: public, now: time.Now}
}

// RegisterPageRoutes mounts the /pages routes on g (which must carry the
// auth and group middleware).
func RegisterPageRoutes(g *gin.RouterGroup, h *PageHandler) {
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/check-slug", h.CheckSlug)
	g.GET("/:id", h.Get)
	g.PUT("/:id", h.Update)
	g.PATCH("/:id", h.SetArchived)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/publish", h.Publish)
	g.GET("/:id/stats", h.Stats)
	g.POST("/:id/blocks", h.AddBlock)
	g.PUT("/:id/blocks/reorder", h.Reorder)
	g.PUT("/:id/blocks/:bid", h.UpdateBlock)
	g.DELETE("/:id/blocks/:bid", h.DeleteBlock)
}

type setArchivedRequest struct {
	Archived *bool `json:"archived"`
}

type reorderRequest struct {
	IDs []uint64 `json:"ids"`
}

func (h *PageHandler) List(c *gin.Context) {
	f := repository.PageFilter{Query: c.Query("q")}
	switch c.DefaultQuery("archived", "off") {
	case "off":
	case "on":
		f.Archived = true
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "archived must be on or off"})
		return
	}
	views, err := h.pages.List(c.Request.Context(), middleware.GetGroupID(c), f)
	if writePageError(c, "list-pages", err) {
		return
	}
	if views == nil {
		views = []service.PageView{}
	}
	c.JSON(http.StatusOK, gin.H{"pages": views, "total": len(views)})
}

func (h *PageHandler) Create(c *gin.Context) {
	var in service.CreatePageInput
	if !bindPageJSON(c, &in) {
		return
	}
	view, err := h.pages.Create(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), in)
	if writePageError(c, "create-page", err) {
		return
	}
	c.JSON(http.StatusCreated, view)
}

func (h *PageHandler) CheckSlug(c *gin.Context) {
	res, err := h.pages.CheckSlug(c.Request.Context(), c.Query("slug"))
	if writePageError(c, "check-slug", err) {
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *PageHandler) Get(c *gin.Context) {
	id, ok := parsePageID(c)
	if !ok {
		return
	}
	view, err := h.pages.Get(c.Request.Context(), middleware.GetGroupID(c), id)
	if writePageError(c, "get-page", err) {
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *PageHandler) Update(c *gin.Context) {
	id, ok := parsePageID(c)
	if !ok {
		return
	}
	var in service.UpdatePageInput
	if !bindPageJSON(c, &in) {
		return
	}
	view, err := h.pages.Update(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, in)
	if writePageError(c, "update-page", err) {
		return
	}
	c.JSON(http.StatusOK, view)
}

// SetArchived handles PATCH /pages/:id {archived}: hide or unhide the page.
func (h *PageHandler) SetArchived(c *gin.Context) {
	id, ok := parsePageID(c)
	if !ok {
		return
	}
	var req setArchivedRequest
	if !bindPageJSON(c, &req) {
		return
	}
	if req.Archived == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "archived is required"})
		return
	}
	view, err := h.pages.SetArchived(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, *req.Archived)
	if writePageError(c, "update-page", err) {
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *PageHandler) Delete(c *gin.Context) {
	id, ok := parsePageID(c)
	if !ok {
		return
	}
	err := h.pages.Delete(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id)
	if writePageError(c, "delete-page", err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PageHandler) Publish(c *gin.Context) {
	id, ok := parsePageID(c)
	if !ok {
		return
	}
	view, err := h.pages.Publish(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id)
	if writePageError(c, "publish-page", err) {
		return
	}
	c.JSON(http.StatusOK, view)
}

// Stats handles GET /pages/:id/stats?from&to&unit&compare (source ignored).
func (h *PageHandler) Stats(c *gin.Context) {
	id, ok := parsePageID(c)
	if !ok {
		return
	}
	q, err := parseStatsQuery(c, h.now())
	if writePageError(c, "get-page-stats", err) {
		return
	}
	q.Source = ""
	stats, err := h.public.Stats(c.Request.Context(), middleware.GetGroupID(c), id, q)
	if writePageError(c, "get-page-stats", err) {
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *PageHandler) AddBlock(c *gin.Context) {
	id, ok := parsePageID(c)
	if !ok {
		return
	}
	var in service.BlockInput
	if !bindPageJSON(c, &in) {
		return
	}
	block, err := h.pages.AddBlock(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, in)
	if writePageError(c, "add-block", err) {
		return
	}
	c.JSON(http.StatusCreated, block)
}

func (h *PageHandler) UpdateBlock(c *gin.Context) {
	id, bid, ok := parsePageAndBlockID(c)
	if !ok {
		return
	}
	var in service.BlockPatch
	if !bindPageJSON(c, &in) {
		return
	}
	block, err := h.pages.UpdateBlock(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, bid, in)
	if writePageError(c, "update-block", err) {
		return
	}
	c.JSON(http.StatusOK, block)
}

func (h *PageHandler) DeleteBlock(c *gin.Context) {
	id, bid, ok := parsePageAndBlockID(c)
	if !ok {
		return
	}
	err := h.pages.DeleteBlock(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, bid)
	if writePageError(c, "delete-block", err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PageHandler) Reorder(c *gin.Context) {
	id, ok := parsePageID(c)
	if !ok {
		return
	}
	var req reorderRequest
	if !bindPageJSON(c, &req) {
		return
	}
	blocks, err := h.pages.Reorder(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, req.IDs)
	if writePageError(c, "reorder-blocks", err) {
		return
	}
	if blocks == nil {
		blocks = []service.BlockView{}
	}
	c.JSON(http.StatusOK, gin.H{"blocks": blocks})
}

func bindPageJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	return true
}

func parsePageID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page ID"})
		return 0, false
	}
	return id, true
}

func parsePageAndBlockID(c *gin.Context) (uint64, uint64, bool) {
	id, ok := parsePageID(c)
	if !ok {
		return 0, 0, false
	}
	bid, err := strconv.ParseUint(c.Param("bid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid block ID"})
		return 0, 0, false
	}
	return id, bid, true
}

// writePageError writes the response for a non-nil err and reports whether
// it did. Errors it does not know (e.g. from LinkService.Create for a
// new_link block) go through writeServiceError, which also logs and turns
// anything unknown into a 500 "failed to <op>".
func writePageError(c *gin.Context, op string, err error) bool {
	if err == nil {
		return false
	}
	var ve *service.ValidationError
	var qe *service.StatsQueryError
	switch {
	case errors.As(err, &ve):
		c.JSON(http.StatusBadRequest, gin.H{"error": ve.Msg})
	case errors.As(err, &qe):
		c.JSON(http.StatusBadRequest, gin.H{"error": qe.Msg})
	case errors.Is(err, repository.ErrPageSlugTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "slug is already taken"})
	case errors.Is(err, repository.ErrPageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
	case errors.Is(err, repository.ErrBlockNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "block not found"})
	default:
		return writeServiceError(c, op, err)
	}
	return true
}
