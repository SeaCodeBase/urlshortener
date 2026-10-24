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

type groupRouter struct {
	r        *gin.Engine
	svc      *mocks.MockGroupService
	activity *mocks.MockActivityService
}

func newGroupRouter(t *testing.T) *groupRouter {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	gr := &groupRouter{svc: mocks.NewMockGroupService(ctrl), activity: mocks.NewMockActivityService(ctrl)}
	h := NewGroupHandler(gr.svc, gr.activity)
	gr.r = gin.New()
	// Groups use auth only: no group_id is set.
	g := gr.r.Group("/api/groups", func(c *gin.Context) { c.Set("user_id", uint64(1)); c.Next() })
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PUT("/:id", h.Update)
	return gr
}

var groupTime = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func TestGroupHandler_List(t *testing.T) {
	gr := newGroupRouter(t)
	gr.svc.EXPECT().List(gomock.Any(), uint64(1)).Return([]model.Group{
		{ID: 7, UserID: 1, Name: "Main", DefaultDomainID: u64p(3), CreatedAt: groupTime, UpdatedAt: groupTime},
	}, nil)

	w := doJSON(gr.r, http.MethodGet, "/api/groups", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"groups":[{"id":7,"name":"Main","default_domain_id":3,
		"created_at":"2026-10-24T12:00:00Z","updated_at":"2026-10-24T12:00:00Z"}]}`, w.Body.String())
}

func TestGroupHandler_List_Empty(t *testing.T) {
	gr := newGroupRouter(t)
	gr.svc.EXPECT().List(gomock.Any(), uint64(1)).Return(nil, nil)
	w := doJSON(gr.r, http.MethodGet, "/api/groups", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"groups":[]}`, w.Body.String())
}

func TestGroupHandler_Create(t *testing.T) {
	gr := newGroupRouter(t)
	gr.svc.EXPECT().Create(gomock.Any(), uint64(1), "Marketing").
		Return(&model.Group{ID: 9, UserID: 1, Name: "Marketing", CreatedAt: groupTime, UpdatedAt: groupTime}, nil)
	gr.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(9), service.ActionGroupCreate, service.EntityGroup, uint64(9), gomock.Any())

	w := doJSON(gr.r, http.MethodPost, "/api/groups", `{"name":"Marketing"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.JSONEq(t, `{"id":9,"name":"Marketing","default_domain_id":null,
		"created_at":"2026-10-24T12:00:00Z","updated_at":"2026-10-24T12:00:00Z"}`, w.Body.String())
}

func TestGroupHandler_Create_BlankName400(t *testing.T) {
	gr := newGroupRouter(t)
	gr.svc.EXPECT().Create(gomock.Any(), uint64(1), "").Return(nil, service.ErrInvalidGroupName)
	w := doJSON(gr.r, http.MethodPost, "/api/groups", `{}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGroupHandler_Get_Foreign404(t *testing.T) {
	gr := newGroupRouter(t)
	gr.svc.EXPECT().Get(gomock.Any(), uint64(1), uint64(8)).Return(nil, service.ErrNotGroupOwner)
	w := doJSON(gr.r, http.MethodGet, "/api/groups/8", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"group not found"}`, w.Body.String())
}

func TestGroupHandler_Get_InvalidID400(t *testing.T) {
	gr := newGroupRouter(t)
	w := doJSON(gr.r, http.MethodGet, "/api/groups/abc", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGroupHandler_Update(t *testing.T) {
	gr := newGroupRouter(t)
	name := "Renamed"
	gr.svc.EXPECT().Update(gomock.Any(), uint64(1), uint64(7), service.UpdateGroupInput{Name: &name, DefaultDomainID: u64p(3)}).
		Return(&model.Group{ID: 7, UserID: 1, Name: name, DefaultDomainID: u64p(3), CreatedAt: groupTime, UpdatedAt: groupTime}, nil)
	gr.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionGroupUpdate, service.EntityGroup, uint64(7), gomock.Any())

	w := doJSON(gr.r, http.MethodPut, "/api/groups/7", `{"name":"Renamed","default_domain_id":3}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"id":7,"name":"Renamed","default_domain_id":3,
		"created_at":"2026-10-24T12:00:00Z","updated_at":"2026-10-24T12:00:00Z"}`, w.Body.String())
}

func TestGroupHandler_Update_DefaultDomainMustBeVerified(t *testing.T) {
	gr := newGroupRouter(t)
	gr.svc.EXPECT().Update(gomock.Any(), uint64(1), uint64(7), service.UpdateGroupInput{DefaultDomainID: u64p(3)}).
		Return(nil, service.ErrDomainNotVerified)

	w := doJSON(gr.r, http.MethodPut, "/api/groups/7", `{"default_domain_id":3}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"domain is not verified"}`, w.Body.String())
}

func TestGroupHandler_Update_ErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
		body string
	}{
		{service.ErrNotDomainOwner, http.StatusNotFound, `{"error":"domain not found"}`},
		{service.ErrGroupNotFound, http.StatusNotFound, `{"error":"group not found"}`},
		{service.ErrNotGroupOwner, http.StatusNotFound, `{"error":"group not found"}`},
		{service.ErrInvalidGroupName, http.StatusBadRequest, `{"error":"group name must not be empty"}`},
	} {
		gr := newGroupRouter(t)
		gr.svc.EXPECT().Update(gomock.Any(), uint64(1), uint64(7), service.UpdateGroupInput{ClearDefaultDomain: true}).Return(nil, tc.err)
		w := doJSON(gr.r, http.MethodPut, "/api/groups/7", `{"clear_default_domain":true}`)
		assert.Equal(t, tc.code, w.Code, tc.err.Error())
		assert.JSONEq(t, tc.body, w.Body.String())
	}
}
