package handler

import (
	"errors"
	"net/http"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// maxUploadRequestBytes caps the multipart request body (file plus overhead).
const maxUploadRequestBytes = 6 << 20

const (
	msgUploadTooLarge = "File must be 5 MB or smaller"
	msgUploadType     = "Only PNG and JPEG images are allowed"
)

// UploadHandler serves POST /api/uploads.
type UploadHandler struct {
	uploads service.UploadService
}

func NewUploadHandler(uploads service.UploadService) *UploadHandler {
	return &UploadHandler{uploads: uploads}
}

type uploadResponse struct {
	URL         string `json:"url"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// Create handles POST /uploads with a multipart "file" field (PNG or JPEG,
// at most 5 MB).
func (h *UploadHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadRequestBytes)

	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": msgUploadTooLarge})
			return
		}
		logger.Warn(ctx, "upload: missing file", zap.Uint64("user_id", userID), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	f, err := fh.Open()
	if err != nil {
		logger.Error(ctx, "upload: failed to open file", zap.Uint64("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read upload"})
		return
	}
	defer f.Close()

	up, err := h.uploads.Save(ctx, userID, f, fh.Size)
	switch {
	case errors.Is(err, service.ErrUploadTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": msgUploadTooLarge})
	case errors.Is(err, service.ErrUploadType):
		c.JSON(http.StatusBadRequest, gin.H{"error": msgUploadType})
	case err != nil:
		logger.Error(ctx, "upload: failed to save file", zap.Uint64("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload"})
	default:
		c.JSON(http.StatusCreated, uploadResponse{URL: up.URL, Name: up.Name, Size: up.Size, ContentType: up.ContentType})
	}
}
