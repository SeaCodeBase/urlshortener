package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DomainHandler struct {
	domainRepo repository.DomainRepository
}

func NewDomainHandler(domainRepo repository.DomainRepository) *DomainHandler {
	return &DomainHandler{domainRepo: domainRepo}
}

type CreateDomainRequest struct {
	Domain string `json:"domain" binding:"required"`
}

func (h *DomainHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	var req CreateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn(ctx, "domain-handler: invalid request body",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	domain := &model.Domain{
		UserID: userID,
		Domain: req.Domain,
	}

	if err := h.domainRepo.Create(ctx, domain); err != nil {
		if errors.Is(err, repository.ErrDomainExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Domain already in use"})
			return
		}
		logger.Error(ctx, "domain-handler: failed to create domain",
			zap.String("domain", req.Domain),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create domain"})
		return
	}

	c.JSON(http.StatusCreated, domain)
}

func (h *DomainHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	domains, err := h.domainRepo.ListByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "domain-handler: failed to list domains",
			zap.Uint64("userID", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list domains"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"domains": domains})
}

func (h *DomainHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	// Check ownership
	domain, err := h.domainRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrDomainNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
			return
		}
		logger.Error(ctx, "domain-handler: failed to get domain",
			zap.Uint64("id", id),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get domain"})
		return
	}

	if domain.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't own this domain"})
		return
	}

	if err := h.domainRepo.Delete(ctx, id); err != nil {
		logger.Error(ctx, "domain-handler: failed to delete domain",
			zap.Uint64("id", id),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete domain"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Domain deleted"})
}
