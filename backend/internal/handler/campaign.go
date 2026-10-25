package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/gin-gonic/gin"
)

// CampaignHandler serves the active group's campaigns and channels. Links in
// campaign responses are rendered through the link presenter.
type CampaignHandler struct {
	campaigns service.CampaignService
	// links renders link responses (short URL, favicon, details) exactly as
	// the link handler does; its invalidator is unused.
	links *LinkHandler
	now   func() time.Time
}

func NewCampaignHandler(campaigns service.CampaignService, linkService service.LinkService, domainRepo repository.DomainRepository, cfg *config.Config) *CampaignHandler {
	return &CampaignHandler{
		campaigns: campaigns,
		links:     &LinkHandler{linkPresenter: newLinkPresenter(cfg, domainRepo), linkService: linkService},
		now:       time.Now,
	}
}

// SetPageIDs sets the lookup used to fill page_ids of rendered links.
func (h *CampaignHandler) SetPageIDs(pages PageIDsLookup) { h.links.SetPageIDs(pages) }

// RegisterCampaignRoutes mounts /campaigns and /channels on api behind mw
// (the auth and group middleware).
func RegisterCampaignRoutes(api *gin.RouterGroup, h *CampaignHandler, mw ...gin.HandlerFunc) {
	g := api.Group("/campaigns", mw...)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/links", h.AddLinks)
	g.DELETE("/:id/links", h.RemoveLink)
	g.POST("/:id/channels/:cid/links", h.CreateLinkInChannel)
	g.GET("/:id/stats", h.Stats)

	ch := api.Group("/channels", mw...)
	ch.GET("", h.ListChannels)
	ch.POST("", h.CreateChannel)
}

// campaignLinkResponse is a link of a campaign channel: the link presenter
// response plus the channel it is listed under.
type campaignLinkResponse struct {
	linkResponse
	ChannelID   uint64 `json:"channel_id"`
	ChannelName string `json:"channel_name"`
}

type channelDetailResponse struct {
	model.Channel
	Links []campaignLinkResponse `json:"links"`
}

type campaignDetailResponse struct {
	model.Campaign
	Channels []channelDetailResponse `json:"channels"`
}

// respondDetail renders d with every link through the presenter.
func (h *CampaignHandler) respondDetail(c *gin.Context, status int, d *service.CampaignDetail) {
	ctx := c.Request.Context()
	domainMap := h.links.loadDomainMap(ctx, middleware.GetUserID(c))
	resp := campaignDetailResponse{Campaign: d.Campaign, Channels: make([]channelDetailResponse, len(d.Channels))}
	var ptrs []*linkResponse
	for i := range d.Channels {
		cd := &d.Channels[i]
		links := make([]campaignLinkResponse, len(cd.Links))
		for j := range cd.Links {
			item := &cd.Links[j]
			lr := h.links.toResponse(&item.Link, item.Tags, item.TotalClicks, domainMap)
			lr.QRCodeID = item.QRCodeID
			links[j] = campaignLinkResponse{linkResponse: lr, ChannelID: item.ChannelID, ChannelName: item.ChannelName}
		}
		for j := range links {
			ptrs = append(ptrs, &links[j].linkResponse)
		}
		resp.Channels[i] = channelDetailResponse{Channel: cd.Channel, Links: links}
	}
	h.links.attachPageIDs(ctx, ptrs...)
	c.JSON(status, resp)
}

func (h *CampaignHandler) List(c *gin.Context) {
	cs, err := h.campaigns.List(c.Request.Context(), middleware.GetGroupID(c))
	if writeCampaignError(c, "list-campaigns", err) {
		return
	}
	if cs == nil {
		cs = []model.CampaignSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"campaigns": cs})
}

func (h *CampaignHandler) Create(c *gin.Context) {
	var in service.CampaignInput
	if !bindPageJSON(c, &in) {
		return
	}
	d, err := h.campaigns.Create(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), in)
	if writeCampaignError(c, "create-campaign", err) {
		return
	}
	h.respondDetail(c, http.StatusCreated, d)
}

func (h *CampaignHandler) Get(c *gin.Context) {
	id, ok := parseCampaignID(c)
	if !ok {
		return
	}
	d, err := h.campaigns.Get(c.Request.Context(), middleware.GetGroupID(c), id)
	if writeCampaignError(c, "get-campaign", err) {
		return
	}
	h.respondDetail(c, http.StatusOK, d)
}

// Update replaces the campaign; a missing or empty channel_ids (and
// new_channels) leaves it without channels.
func (h *CampaignHandler) Update(c *gin.Context) {
	id, ok := parseCampaignID(c)
	if !ok {
		return
	}
	var in service.CampaignInput
	if !bindPageJSON(c, &in) {
		return
	}
	d, err := h.campaigns.Update(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, in)
	if writeCampaignError(c, "update-campaign", err) {
		return
	}
	h.respondDetail(c, http.StatusOK, d)
}

func (h *CampaignHandler) Delete(c *gin.Context) {
	id, ok := parseCampaignID(c)
	if !ok {
		return
	}
	err := h.campaigns.Delete(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id)
	if writeCampaignError(c, "delete-campaign", err) {
		return
	}
	c.Status(http.StatusNoContent)
}

type addCampaignLinksRequest struct {
	ChannelID uint64   `json:"channel_id"`
	LinkIDs   []uint64 `json:"link_ids"`
}

// AddLinks handles POST /campaigns/:id/links {channel_id, link_ids}.
func (h *CampaignHandler) AddLinks(c *gin.Context) {
	id, ok := parseCampaignID(c)
	if !ok {
		return
	}
	var req addCampaignLinksRequest
	if !bindPageJSON(c, &req) {
		return
	}
	err := h.campaigns.AddLinks(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, req.ChannelID, req.LinkIDs)
	if writeCampaignError(c, "add-campaign-links", err) {
		return
	}
	c.Status(http.StatusNoContent)
}

type removeCampaignLinkRequest struct {
	ChannelID uint64 `json:"channel_id"`
	LinkID    uint64 `json:"link_id"`
}

// RemoveLink handles DELETE /campaigns/:id/links {channel_id, link_id}.
func (h *CampaignHandler) RemoveLink(c *gin.Context) {
	id, ok := parseCampaignID(c)
	if !ok {
		return
	}
	var req removeCampaignLinkRequest
	if !bindPageJSON(c, &req) {
		return
	}
	err := h.campaigns.RemoveLink(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, req.ChannelID, req.LinkID)
	if writeCampaignError(c, "remove-campaign-link", err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// CreateLinkInChannel creates a link in the channel and renders it like
// POST /links (a new link has no cached redirect to invalidate).
func (h *CampaignHandler) CreateLinkInChannel(c *gin.Context) {
	id, ok := parseCampaignID(c)
	if !ok {
		return
	}
	channelID, err := strconv.ParseUint(c.Param("cid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel ID"})
		return
	}
	var in service.CreateLinkInChannelInput
	if !bindPageJSON(c, &in) {
		return
	}
	userID := middleware.GetUserID(c)
	link, err := h.campaigns.CreateLinkInChannel(c.Request.Context(), userID, middleware.GetGroupID(c), id, channelID, in)
	if writeCampaignError(c, "create-campaign-link", err) {
		return
	}
	h.links.respondDetails(c, http.StatusCreated, userID, link, nil)
}

// Stats handles GET /campaigns/:id/stats?from&to&unit.
func (h *CampaignHandler) Stats(c *gin.Context) {
	id, ok := parseCampaignID(c)
	if !ok {
		return
	}
	q, err := parseStatsQuery(c, h.now())
	if writeCampaignError(c, "campaign-stats", err) {
		return
	}
	st, err := h.campaigns.Stats(c.Request.Context(), middleware.GetGroupID(c), id, q)
	if writeCampaignError(c, "campaign-stats", err) {
		return
	}
	c.JSON(http.StatusOK, st)
}

func (h *CampaignHandler) ListChannels(c *gin.Context) {
	chs, err := h.campaigns.ListChannels(c.Request.Context(), middleware.GetGroupID(c))
	if writeCampaignError(c, "list-channels", err) {
		return
	}
	if chs == nil {
		chs = []model.Channel{}
	}
	c.JSON(http.StatusOK, gin.H{"channels": chs})
}

type createChannelRequest struct {
	Name string `json:"name"`
}

func (h *CampaignHandler) CreateChannel(c *gin.Context) {
	var req createChannelRequest
	if !bindPageJSON(c, &req) {
		return
	}
	ch, err := h.campaigns.CreateChannel(c.Request.Context(), middleware.GetGroupID(c), req.Name)
	if writeCampaignError(c, "create-channel", err) {
		return
	}
	c.JSON(http.StatusCreated, ch)
}

func parseCampaignID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign ID"})
		return 0, false
	}
	return id, true
}

// writeCampaignError maps ValidationError/StatsQueryError to 400, the
// campaign/channel/link not-found errors to 404 and ErrChannelExists to 409;
// link service errors (CreateLinkInChannel) map as in the link handler and
// anything else is a logged 500. It reports whether it wrote.
func writeCampaignError(c *gin.Context, op string, err error) bool {
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
	case errors.Is(err, repository.ErrCampaignNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
	case errors.Is(err, repository.ErrChannelNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
	case errors.Is(err, repository.ErrChannelExists):
		c.JSON(http.StatusConflict, gin.H{"error": "channel already exists"})
	case errors.Is(err, repository.ErrLinkNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": msgLinkNotFound})
	default:
		return writeServiceError(c, op, err)
	}
	return true
}
