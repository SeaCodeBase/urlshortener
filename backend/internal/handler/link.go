// backend/internal/handler/link.go
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/service"
)

type LinkHandler struct {
	linkService service.LinkService
	baseURL     string
}

func NewLinkHandler(linkService service.LinkService, cfg *config.Config) *LinkHandler {
	return &LinkHandler{
		linkService: linkService,
		baseURL:     cfg.BaseURL,
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

func (h *LinkHandler) toResponse(link *model.Link) linkResponse {
	return linkResponse{
		Link:     link,
		ShortURL: h.baseURL + "/" + link.ShortCode,
	}
}

func (h *LinkHandler) toListResponse(result *service.ListLinksResult) listLinksResponse {
	links := make([]linkResponse, len(result.Links))
	for i := range result.Links {
		links[i] = h.toResponse(&result.Links[i])
	}
	return listLinksResponse{
		Links:      links,
		Total:      result.Total,
		Page:       result.Page,
		TotalPages: result.TotalPages,
	}
}

func (h *LinkHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var input service.CreateLinkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	link, err := h.linkService.Create(c.Request.Context(), userID, input)
	if errors.Is(err, service.ErrInvalidShortCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid custom code"})
		return
	}
	if errors.Is(err, service.ErrShortCodeTaken) {
		c.JSON(http.StatusConflict, gin.H{"error": "short code already taken"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create link"})
		return
	}

	c.JSON(http.StatusCreated, h.toResponse(link))
}

func (h *LinkHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	link, err := h.linkService.GetByID(c.Request.Context(), userID, linkID)
	if errors.Is(err, service.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get link"})
		return
	}

	c.JSON(http.StatusOK, h.toResponse(link))
}

func (h *LinkHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	params := service.ListLinksParams{
		Page:  page,
		Limit: limit,
	}

	result, err := h.linkService.List(c.Request.Context(), userID, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list links"})
		return
	}

	c.JSON(http.StatusOK, h.toListResponse(result))
}

func (h *LinkHandler) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	var input service.UpdateLinkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	link, err := h.linkService.Update(c.Request.Context(), userID, linkID, input)
	if errors.Is(err, service.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update link"})
		return
	}

	c.JSON(http.StatusOK, h.toResponse(link))
}

func (h *LinkHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	err = h.linkService.Delete(c.Request.Context(), userID, linkID)
	if errors.Is(err, service.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete link"})
		return
	}

	c.Status(http.StatusNoContent)
}
