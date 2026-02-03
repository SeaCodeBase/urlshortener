// backend/internal/handler/link.go
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type LinkHandler struct {
	linkService service.LinkService
	domainRepo  repository.DomainRepository
	baseURL     string
}

func NewLinkHandler(linkService service.LinkService, domainRepo repository.DomainRepository, cfg *config.Config) *LinkHandler {
	return &LinkHandler{
		linkService: linkService,
		domainRepo:  domainRepo,
		baseURL:     cfg.URLs.BaseURL,
	}
}

type linkResponse struct {
	*model.Link
	ShortURL string `json:"short_url"`
}

type listLinksResponse struct {
	Links      []linkResponse `json:"links"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	TotalPages int            `json:"total_pages"`
}

func (h *LinkHandler) buildShortURL(link *model.Link, domainMap map[uint64]string) string {
	if link.DomainID != nil {
		if domain, ok := domainMap[*link.DomainID]; ok {
			return "http://" + domain + "/" + link.ShortCode
		}
	}
	return h.baseURL + "/" + link.ShortCode
}

func (h *LinkHandler) toResponse(link *model.Link, domainMap map[uint64]string) linkResponse {
	return linkResponse{
		Link:     link,
		ShortURL: h.buildShortURL(link, domainMap),
	}
}

func (h *LinkHandler) toListResponse(result *service.ListLinksResult, domainMap map[uint64]string) listLinksResponse {
	links := make([]linkResponse, len(result.Links))
	for i := range result.Links {
		links[i] = h.toResponse(&result.Links[i], domainMap)
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

	link, err := h.linkService.Create(ctx, userID, input)
	if errors.Is(err, service.ErrInvalidShortCode) {
		logger.Warn(ctx, "create-link: invalid custom code",
			zap.Uint64("user_id", userID),
			zap.String("custom_code", input.CustomCode),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid custom code"})
		return
	}
	if errors.Is(err, service.ErrShortCodeTaken) {
		logger.Warn(ctx, "create-link: short code already taken",
			zap.Uint64("user_id", userID),
			zap.String("custom_code", input.CustomCode),
		)
		c.JSON(http.StatusConflict, gin.H{"error": "short code already taken"})
		return
	}
	if err != nil {
		logger.Error(ctx, "create-link: failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create link"})
		return
	}

	domainMap := h.loadDomainMap(ctx, userID)
	c.JSON(http.StatusCreated, h.toResponse(link, domainMap))
}

func (h *LinkHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn(ctx, "get-link: invalid link ID",
			zap.String("link_id_param", c.Param("id")),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	link, err := h.linkService.GetByID(ctx, userID, linkID)
	if errors.Is(err, service.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
		logger.Warn(ctx, "get-link: not found",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		logger.Error(ctx, "get-link: failed",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get link"})
		return
	}

	domainMap := h.loadDomainMap(ctx, userID)
	c.JSON(http.StatusOK, h.toResponse(link, domainMap))
}

func (h *LinkHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	params := service.ListLinksParams{
		Page:  page,
		Limit: limit,
	}

	result, err := h.linkService.List(ctx, userID, params)
	if err != nil {
		logger.Error(ctx, "list-links: failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list links"})
		return
	}

	domainMap := h.loadDomainMap(ctx, userID)
	c.JSON(http.StatusOK, h.toListResponse(result, domainMap))
}

func (h *LinkHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn(ctx, "update-link: invalid link ID",
			zap.String("link_id_param", c.Param("id")),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
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

	link, err := h.linkService.Update(ctx, userID, linkID, input)
	if errors.Is(err, service.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
		logger.Warn(ctx, "update-link: not found",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		logger.Error(ctx, "update-link: failed",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update link"})
		return
	}

	domainMap := h.loadDomainMap(ctx, userID)
	c.JSON(http.StatusOK, h.toResponse(link, domainMap))
}

func (h *LinkHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn(ctx, "delete-link: invalid link ID",
			zap.String("link_id_param", c.Param("id")),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	err = h.linkService.Delete(ctx, userID, linkID)
	if errors.Is(err, service.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
		logger.Warn(ctx, "delete-link: not found",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		logger.Error(ctx, "delete-link: failed",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete link"})
		return
	}

	c.Status(http.StatusNoContent)
}
