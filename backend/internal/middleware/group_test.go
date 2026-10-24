package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// newGroupRouter fakes an authenticated user 1 and mounts GroupMiddleware.
func newGroupRouter(groupSvc service.GroupService) *gin.Engine {
	r := gin.New()
	r.GET("/x",
		func(c *gin.Context) { c.Set("user_id", uint64(1)); c.Next() },
		middleware.GroupMiddleware(groupSvc),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"group_id": middleware.GetGroupID(c)})
		})
	return r
}

func doGroup(r *gin.Engine, groupHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if groupHeader != "" {
		req.Header.Set("X-Group-ID", groupHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func u64(v uint64) *uint64 { return &v }

func TestGroupMiddleware_Header403(t *testing.T) {
	ctrl := gomock.NewController(t)
	groupSvc := mocks.NewMockGroupService(ctrl)
	groupSvc.EXPECT().ResolveActive(gomock.Any(), uint64(1), u64(99)).Return(nil, service.ErrNotGroupOwner)

	w := doGroup(newGroupRouter(groupSvc), "99")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.JSONEq(t, `{"error":"group not found"}`, w.Body.String())
}

func TestGroupMiddleware_HeaderUnknownGroup403(t *testing.T) {
	ctrl := gomock.NewController(t)
	groupSvc := mocks.NewMockGroupService(ctrl)
	groupSvc.EXPECT().ResolveActive(gomock.Any(), uint64(1), u64(98)).Return(nil, service.ErrGroupNotFound)

	w := doGroup(newGroupRouter(groupSvc), "98")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.JSONEq(t, `{"error":"group not found"}`, w.Body.String())
}

func TestGroupMiddleware_HeaderOwned(t *testing.T) {
	ctrl := gomock.NewController(t)
	groupSvc := mocks.NewMockGroupService(ctrl)
	groupSvc.EXPECT().ResolveActive(gomock.Any(), uint64(1), u64(5)).Return(&model.Group{ID: 5, UserID: 1}, nil)

	w := doGroup(newGroupRouter(groupSvc), "5")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"group_id":5}`, w.Body.String())
}

func TestGroupMiddleware_DefaultWhenMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	groupSvc := mocks.NewMockGroupService(ctrl)
	groupSvc.EXPECT().ResolveActive(gomock.Any(), uint64(1), (*uint64)(nil)).Return(&model.Group{ID: 11, UserID: 1}, nil)

	w := doGroup(newGroupRouter(groupSvc), "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"group_id":11}`, w.Body.String())
}

func TestGroupMiddleware_InvalidHeader400(t *testing.T) {
	ctrl := gomock.NewController(t)
	groupSvc := mocks.NewMockGroupService(ctrl)

	w := doGroup(newGroupRouter(groupSvc), "abc")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"invalid group id"}`, w.Body.String())
}

func TestGroupMiddleware_InternalError500(t *testing.T) {
	ctrl := gomock.NewController(t)
	groupSvc := mocks.NewMockGroupService(ctrl)
	groupSvc.EXPECT().ResolveActive(gomock.Any(), uint64(1), (*uint64)(nil)).Return(nil, errors.New("db down"))

	w := doGroup(newGroupRouter(groupSvc), "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
