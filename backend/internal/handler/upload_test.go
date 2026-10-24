package handler

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUploadRouter(t *testing.T) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	svc, err := service.NewUploadService(dir)
	require.NoError(t, err)
	h := NewUploadHandler(svc)
	r := gin.New()
	r.POST("/api/uploads", func(c *gin.Context) { c.Set("user_id", uint64(1)); c.Next() }, h.Create)
	return r, dir
}

func doUpload(t *testing.T, r *gin.Engine, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "logo.png")
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestUploadHandler_201(t *testing.T) {
	r, dir := newUploadRouter(t)
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))))

	w := doUpload(t, r, buf.Bytes())
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp struct {
		URL         string `json:"url"`
		Name        string `json:"name"`
		Size        int64  `json:"size"`
		ContentType string `json:"content_type"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Regexp(t, `^[0-9a-f]{32}\.png$`, resp.Name)
	assert.Equal(t, "/uploads/"+resp.Name, resp.URL)
	assert.Equal(t, int64(buf.Len()), resp.Size)
	assert.Equal(t, "image/png", resp.ContentType)
	_, err := os.Stat(filepath.Join(dir, resp.Name))
	assert.NoError(t, err)
}

func TestUploadHandler_413(t *testing.T) {
	r, _ := newUploadRouter(t)
	w := doUpload(t, r, bytes.Repeat([]byte{0x89}, 5<<20+1))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.JSONEq(t, `{"error":"File must be 5 MB or smaller"}`, w.Body.String())
}

func TestUploadHandler_400Type(t *testing.T) {
	r, _ := newUploadRouter(t)
	w := doUpload(t, r, []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"Only PNG and JPEG images are allowed"}`, w.Body.String())
}

func TestUploadHandler_400MissingFile(t *testing.T) {
	r, _ := newUploadRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/uploads", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
