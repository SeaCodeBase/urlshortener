package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// maxBulkRequestBytes caps the multipart request (2 MiB file plus
	// form overhead); the file itself is checked by the service.
	maxBulkRequestBytes = 3 << 20
	msgBulkTooLarge     = "file is larger than 2 MB"
)

// BulkUploadHandler serves the active group's CSV bulk uploads.
type BulkUploadHandler struct {
	bulk service.BulkService
}

func NewBulkUploadHandler(bulk service.BulkService) *BulkUploadHandler {
	return &BulkUploadHandler{bulk: bulk}
}

// RegisterBulkUploadRoutes mounts /bulk-uploads on api behind mw (the auth
// and group middleware).
func RegisterBulkUploadRoutes(api *gin.RouterGroup, h *BulkUploadHandler, mw ...gin.HandlerFunc) {
	g := api.Group("/bulk-uploads", mw...)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.GET("/:id/results", h.Results)
}

// Create queues the CSV in multipart field "file" → 201 BulkUpload.
func (h *BulkUploadHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBulkRequestBytes)
	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusBadRequest, gin.H{"error": msgBulkTooLarge})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	f, err := fh.Open()
	if err != nil {
		logger.Error(ctx, "bulk-upload: failed to open file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read upload"})
		return
	}
	defer f.Close()
	// One byte past the limit is enough for the service to reject it.
	data, err := io.ReadAll(io.LimitReader(f, service.BulkMaxBytes+1))
	if err != nil {
		logger.Error(ctx, "bulk-upload: failed to read file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read upload"})
		return
	}
	b, err := h.bulk.Create(ctx, middleware.GetUserID(c), middleware.GetGroupID(c), fh.Filename, data)
	if writeBulkError(c, "create-bulk-upload", err) {
		return
	}
	c.JSON(http.StatusCreated, b)
}

func (h *BulkUploadHandler) List(c *gin.Context) {
	list, err := h.bulk.List(c.Request.Context(), middleware.GetGroupID(c))
	if writeBulkError(c, "list-bulk-uploads", err) {
		return
	}
	if list == nil {
		list = []model.BulkUpload{}
	}
	c.JSON(http.StatusOK, gin.H{"uploads": list})
}

func (h *BulkUploadHandler) Get(c *gin.Context) {
	id, ok := parseBulkID(c)
	if !ok {
		return
	}
	b, err := h.bulk.Get(c.Request.Context(), middleware.GetGroupID(c), id)
	if writeBulkError(c, "get-bulk-upload", err) {
		return
	}
	c.JSON(http.StatusOK, b)
}

// Results downloads the per-row results CSV; 409 until the upload finished.
func (h *BulkUploadHandler) Results(c *gin.Context) {
	id, ok := parseBulkID(c)
	if !ok {
		return
	}
	out, err := h.bulk.Results(c.Request.Context(), middleware.GetGroupID(c), id)
	if writeBulkError(c, "get-bulk-upload-results", err) {
		return
	}
	c.Header("Content-Disposition", `attachment; filename="bulk-`+strconv.FormatUint(id, 10)+`-results.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", []byte(out))
}

func parseBulkID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload ID"})
		return 0, false
	}
	return id, true
}

// writeBulkError maps ValidationError to 400, ErrBulkUploadNotFound to 404
// and ErrBulkNotFinished to 409; anything else is a logged 500. It reports
// whether it wrote.
func writeBulkError(c *gin.Context, op string, err error) bool {
	if err == nil {
		return false
	}
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		c.JSON(http.StatusBadRequest, gin.H{"error": ve.Msg})
	case errors.Is(err, repository.ErrBulkUploadNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
	case errors.Is(err, service.ErrBulkNotFinished):
		c.JSON(http.StatusConflict, gin.H{"error": "upload not finished"})
	default:
		return writeServiceError(c, op, err)
	}
	return true
}
