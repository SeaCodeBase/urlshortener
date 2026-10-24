package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func withGroup(c *gin.Context) {
	c.Set("user_id", uint64(1))
	c.Set("group_id", uint64(7))
	c.Next()
}

func newActivityRouter(t *testing.T) (*gin.Engine, *mocks.MockActivityService) {
	gin.SetMode(gin.TestMode)
	svc := mocks.NewMockActivityService(gomock.NewController(t))
	r := gin.New()
	r.GET("/api/activity-log", withGroup, NewActivityHandler(svc).List)
	return r, svc
}

func TestActivityHandler_List(t *testing.T) {
	r, svc := newActivityRouter(t)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	toExcl := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC) // "to" is inclusive of the whole day
	created := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

	svc.EXPECT().List(gomock.Any(), uint64(7), &from, &toExcl, 2, 10).Return(&service.ActivityListResult{
		Entries: []model.ActivityEntry{{ID: 3, UserID: 1, GroupID: 7, Action: "link.create", EntityType: "link", EntityID: 4,
			Metadata: model.JSONMap{"short_code": "abc"}, CreatedAt: created}},
		Total: 11, Page: 2, TotalPages: 2,
	}, nil)

	w := doJSON(r, http.MethodGet, "/api/activity-log?from=2026-10-01&to=2026-10-24&page=2&limit=10", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"entries":[{"id":3,"user_id":1,"group_id":7,"action":"link.create","entity_type":"link","entity_id":4,
		"metadata":{"short_code":"abc"},"created_at":"2026-10-24T12:00:00Z"}],"total":11,"page":2,"total_pages":2}`, w.Body.String())
}

func TestActivityHandler_List_Defaults(t *testing.T) {
	r, svc := newActivityRouter(t)
	svc.EXPECT().List(gomock.Any(), uint64(7), nil, nil, 1, 20).
		Return(&service.ActivityListResult{Entries: []model.ActivityEntry{}, Page: 1}, nil)

	w := doJSON(r, http.MethodGet, "/api/activity-log", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"entries":[],"total":0,"page":1,"total_pages":0}`, w.Body.String())
}

func TestActivityHandler_List_LimitCapped(t *testing.T) {
	r, svc := newActivityRouter(t)
	svc.EXPECT().List(gomock.Any(), uint64(7), nil, nil, 1, 100).
		Return(&service.ActivityListResult{Entries: []model.ActivityEntry{}, Page: 1}, nil)

	w := doJSON(r, http.MethodGet, "/api/activity-log?limit=1000", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestActivityHandler_List_BadParams400(t *testing.T) {
	r, _ := newActivityRouter(t)
	for _, q := range []string{"from=2026-13-01", "to=yesterday", "page=abc", "limit=x", "from=2026-10-05&to=2026-10-01"} {
		w := doJSON(r, http.MethodGet, "/api/activity-log?"+q, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, q)
	}
}

func TestTagHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := mocks.NewMockTagService(gomock.NewController(t))
	r := gin.New()
	r.GET("/api/tags", withGroup, NewTagHandler(svc).List)

	svc.EXPECT().ListByGroup(gomock.Any(), uint64(7), "link").Return([]string{"alpha", "beta"}, nil)
	w := doJSON(r, http.MethodGet, "/api/tags?type=link", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"tags":["alpha","beta"]}`, w.Body.String())

	svc.EXPECT().ListByGroup(gomock.Any(), uint64(7), "link").Return(nil, nil)
	w = doJSON(r, http.MethodGet, "/api/tags", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"tags":[]}`, w.Body.String())

	svc.EXPECT().ListByGroup(gomock.Any(), uint64(7), "bogus").Return(nil, service.ErrInvalidTagType)
	w = doJSON(r, http.MethodGet, "/api/tags?type=bogus", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
