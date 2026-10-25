package handler

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var bulkTS = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

func newBulkRouter(t *testing.T) (*gin.Engine, *mocks.MockBulkService) {
	gin.SetMode(gin.TestMode)
	svc := mocks.NewMockBulkService(gomock.NewController(t))
	r := gin.New()
	RegisterBulkUploadRoutes(r.Group("/api", withGroup), NewBulkUploadHandler(svc))
	return r, svc
}

func sampleBulk(id uint64) *model.BulkUpload {
	res := "secret result"
	return &model.BulkUpload{ID: id, UserID: 1, GroupID: 7, Filename: "links.csv", Status: model.BulkStatusQueued,
		Total: 2, SourceCSV: "secret source", ResultCSV: &res, CreatedAt: bulkTS, UpdatedAt: bulkTS}
}

func postBulkFile(r *gin.Engine, field, filename string, data []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile(field, filename)
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/bulk-uploads", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestBulkHandler_Upload201(t *testing.T) {
	r, svc := newBulkRouter(t)
	csv := []byte("long_url,title,custom_back_half,tags,domain\nhttps://a.example,,,,\n")
	svc.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), "links.csv", csv).Return(sampleBulk(4), nil)
	w := postBulkFile(r, "file", "links.csv", csv)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.JSONEq(t, `{"id":4,"user_id":1,"group_id":7,"filename":"links.csv","status":"queued","total":2,
		"processed":0,"succeeded":0,"failed":0,"error":"","created_at":"2026-10-25T12:00:00Z","updated_at":"2026-10-25T12:00:00Z"}`,
		w.Body.String())
	assert.NotContains(t, w.Body.String(), "secret")
}

func TestBulkHandler_UploadErrors(t *testing.T) {
	r, svc := newBulkRouter(t)
	w := postBulkFile(r, "other", "links.csv", []byte("x"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"file is required"}`, w.Body.String())

	svc.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), "bad.csv", gomock.Any()).
		Return(nil, &service.ValidationError{Msg: "the header must be long_url,title,custom_back_half,tags,domain"})
	w = postBulkFile(r, "file", "bad.csv", []byte("nope"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "the header must be")

	// A file over 2 MiB reaches the service cut at 2 MiB + 1 byte.
	svc.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), "big.csv", gomock.Any()).DoAndReturn(
		func(_ any, _, _ uint64, _ string, data []byte) (*model.BulkUpload, error) {
			assert.Len(t, data, service.BulkMaxBytes+1)
			return nil, &service.ValidationError{Msg: "file is larger than 2 MB"}
		})
	w = postBulkFile(r, "file", "big.csv", bytes.Repeat([]byte("a"), service.BulkMaxBytes+100))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"file is larger than 2 MB"}`, w.Body.String())

	// A request far over the cap is rejected before the service.
	w = postBulkFile(r, "file", "huge.csv", bytes.Repeat([]byte("a"), 4<<20))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"file is larger than 2 MB"}`, w.Body.String())
}

func TestBulkHandler_ListGet(t *testing.T) {
	r, svc := newBulkRouter(t)
	svc.EXPECT().List(gomock.Any(), uint64(7)).Return(nil, nil)
	w := doJSON(r, http.MethodGet, "/api/bulk-uploads", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"uploads":[]}`, w.Body.String())

	svc.EXPECT().List(gomock.Any(), uint64(7)).Return([]model.BulkUpload{*sampleBulk(4)}, nil)
	w = doJSON(r, http.MethodGet, "/api/bulk-uploads", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"uploads":[{"id":4`)

	svc.EXPECT().Get(gomock.Any(), uint64(7), uint64(4)).Return(sampleBulk(4), nil)
	w = doJSON(r, http.MethodGet, "/api/bulk-uploads/4", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"filename":"links.csv"`)

	svc.EXPECT().Get(gomock.Any(), uint64(7), uint64(5)).Return(nil, repository.ErrBulkUploadNotFound)
	w = doJSON(r, http.MethodGet, "/api/bulk-uploads/5", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"upload not found"}`, w.Body.String())

	w = doJSON(r, http.MethodGet, "/api/bulk-uploads/abc", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	svc.EXPECT().List(gomock.Any(), uint64(7)).Return(nil, errors.New("db down"))
	w = doJSON(r, http.MethodGet, "/api/bulk-uploads", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestBulkHandler_Results_CSVAttachment(t *testing.T) {
	r, svc := newBulkRouter(t)
	out := "row,long_url,short_url,status,error\n1,https://a.example,http://s/x,ok,\n"
	svc.EXPECT().Results(gomock.Any(), uint64(7), uint64(4)).Return(out, nil)
	w := doJSON(r, http.MethodGet, "/api/bulk-uploads/4/results", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="bulk-4-results.csv"`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, out, w.Body.String())

	svc.EXPECT().Results(gomock.Any(), uint64(7), uint64(4)).Return("", service.ErrBulkNotFinished)
	w = doJSON(r, http.MethodGet, "/api/bulk-uploads/4/results", "")
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"upload not finished"}`, w.Body.String())
	assert.Empty(t, w.Header().Get("Content-Disposition"))

	svc.EXPECT().Results(gomock.Any(), uint64(7), uint64(9)).Return("", repository.ErrBulkUploadNotFound)
	w = doJSON(r, http.MethodGet, "/api/bulk-uploads/9/results", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, strings.HasPrefix(w.Header().Get("Content-Type"), "text/csv"))
}
