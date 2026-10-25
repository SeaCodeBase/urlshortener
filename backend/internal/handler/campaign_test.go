package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	repomocks "github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var campaignTS = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

type campaignRouter struct {
	r       *gin.Engine
	svc     *mocks.MockCampaignService
	links   *mocks.MockLinkService
	domains *repomocks.MockDomainRepository
}

func newCampaignRouter(t *testing.T) *campaignRouter {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	cr := &campaignRouter{
		svc:     mocks.NewMockCampaignService(ctrl),
		links:   mocks.NewMockLinkService(ctrl),
		domains: repomocks.NewMockDomainRepository(ctrl),
	}
	favicons := true
	cfg := &config.Config{}
	cfg.URLs.BaseURL = "https://sho.rt"
	cfg.UI.FaviconService = &favicons
	h := NewCampaignHandler(cr.svc, cr.links, cr.domains, cfg)
	h.now = func() time.Time { return campaignTS }
	cr.r = gin.New()
	RegisterCampaignRoutes(cr.r.Group("/api", withGroup), h)
	return cr
}

func (cr *campaignRouter) expectDomains(domains ...*model.Domain) {
	cr.domains.EXPECT().ListByUserID(gomock.Any(), uint64(1)).Return(domains, nil).AnyTimes()
}

func sampleCampaign(id uint64) model.Campaign {
	return model.Campaign{ID: id, UserID: 1, GroupID: 7, Name: "Launch", Description: "Q4",
		AutoUTM: true, CreatedAt: campaignTS, UpdatedAt: campaignTS}
}

func sampleCampaignDetail(id uint64) *service.CampaignDetail {
	ch := model.Channel{ID: 4, GroupID: 7, Name: "Email", CreatedAt: campaignTS}
	return &service.CampaignDetail{
		Campaign: sampleCampaign(id),
		Channels: []service.ChannelDetail{{
			Channel: ch,
			Links: []service.CampaignLinkItem{{
				LinkWithStats: model.LinkWithStats{
					Link:        model.Link{ID: 10, ShortCode: "abc", OriginalURL: "https://example.com/p", DomainID: u64p(5)},
					TotalClicks: 12,
					Tags:        []string{"x"},
				},
				ChannelID:   4,
				ChannelName: "Email",
			}},
		}},
	}
}

func TestCampaignHandler_CreateValidation400(t *testing.T) {
	cr := newCampaignRouter(t)
	cr.svc.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), service.CampaignInput{Name: ""}).
		Return(nil, &service.ValidationError{Msg: "name must be 1-120 characters"})
	w := doJSON(cr.r, http.MethodPost, "/api/campaigns", `{"name":""}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"name must be 1-120 characters"}`, w.Body.String())

	w = doJSON(cr.r, http.MethodPost, "/api/campaigns", `nope`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"invalid request body"}`, w.Body.String())
}

func TestCampaignHandler_Create201Detail(t *testing.T) {
	cr := newCampaignRouter(t)
	cr.expectDomains(&model.Domain{ID: 5, Domain: "go.brand.test"})
	in := service.CampaignInput{Name: "Launch", Description: "Q4", AutoUTM: true,
		ChannelIDs: []uint64{4}, NewChannels: []string{"SMS"}}
	cr.svc.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), in).Return(sampleCampaignDetail(3), nil)
	w := doJSON(cr.r, http.MethodPost, "/api/campaigns",
		`{"name":"Launch","description":"Q4","auto_utm":true,"channel_ids":[4],"new_channels":["SMS"]}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	body := w.Body.String()
	assert.Contains(t, body, `"name":"Launch"`)
	assert.Contains(t, body, `"auto_utm":true`)
	assert.Contains(t, body, `"channels":[{"id":4`)
	assert.Contains(t, body, `"short_url":"https://go.brand.test/abc"`)
	assert.Contains(t, body, `"favicon_url":"https://www.google.com/s2/favicons?domain=example.com`)
	assert.Contains(t, body, `"channel_id":4`)
	assert.Contains(t, body, `"channel_name":"Email"`)
	assert.Contains(t, body, `"total_clicks":12`)
	assert.Contains(t, body, `"tags":["x"]`)
}

func TestCampaignHandler_ListGetUpdateDelete(t *testing.T) {
	cr := newCampaignRouter(t)
	cr.expectDomains()
	cr.svc.EXPECT().List(gomock.Any(), uint64(7)).Return([]model.CampaignSummary{
		{Campaign: sampleCampaign(3), ChannelCount: 2, LinkCount: 5, Engagements: 40}}, nil)
	w := doJSON(cr.r, http.MethodGet, "/api/campaigns", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"campaigns":[{"id":3`)
	assert.Contains(t, w.Body.String(), `"engagements":40`)

	cr.svc.EXPECT().List(gomock.Any(), uint64(7)).Return(nil, nil)
	w = doJSON(cr.r, http.MethodGet, "/api/campaigns", "")
	assert.JSONEq(t, `{"campaigns":[]}`, w.Body.String())

	cr.svc.EXPECT().Get(gomock.Any(), uint64(7), uint64(3)).Return(sampleCampaignDetail(3), nil)
	w = doJSON(cr.r, http.MethodGet, "/api/campaigns/3", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"short_url":"https://sho.rt/abc"`)

	// Empty channel list renders [] (not null), and missing channel_ids is nil.
	empty := &service.CampaignDetail{Campaign: sampleCampaign(3)}
	cr.svc.EXPECT().Update(gomock.Any(), uint64(1), uint64(7), uint64(3), service.CampaignInput{Name: "New"}).
		Return(empty, nil)
	w = doJSON(cr.r, http.MethodPut, "/api/campaigns/3", `{"name":"New"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"channels":[]`)

	cr.svc.EXPECT().Delete(gomock.Any(), uint64(1), uint64(7), uint64(3)).Return(nil)
	w = doJSON(cr.r, http.MethodDelete, "/api/campaigns/3", "")
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = doJSON(cr.r, http.MethodGet, "/api/campaigns/abc", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	cr.svc.EXPECT().List(gomock.Any(), uint64(7)).Return(nil, errors.New("db down"))
	w = doJSON(cr.r, http.MethodGet, "/api/campaigns", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"failed to list campaigns"}`, w.Body.String())
}

func TestCampaignHandler_NotFound404(t *testing.T) {
	cr := newCampaignRouter(t)
	cr.svc.EXPECT().Get(gomock.Any(), uint64(7), uint64(99)).Return(nil, repository.ErrCampaignNotFound)
	w := doJSON(cr.r, http.MethodGet, "/api/campaigns/99", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"campaign not found"}`, w.Body.String())

	cr.svc.EXPECT().AddLinks(gomock.Any(), uint64(1), uint64(7), uint64(3), uint64(44), []uint64{10}).
		Return(repository.ErrChannelNotFound)
	w = doJSON(cr.r, http.MethodPost, "/api/campaigns/3/links", `{"channel_id":44,"link_ids":[10]}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"channel not found"}`, w.Body.String())
}

func TestCampaignHandler_AddLinks204(t *testing.T) {
	cr := newCampaignRouter(t)
	cr.svc.EXPECT().AddLinks(gomock.Any(), uint64(1), uint64(7), uint64(3), uint64(4), []uint64{10, 11}).Return(nil)
	w := doJSON(cr.r, http.MethodPost, "/api/campaigns/3/links", `{"channel_id":4,"link_ids":[10,11]}`)
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	cr.svc.EXPECT().RemoveLink(gomock.Any(), uint64(1), uint64(7), uint64(3), uint64(4), uint64(10)).Return(nil)
	w = doJSON(cr.r, http.MethodDelete, "/api/campaigns/3/links", `{"channel_id":4,"link_id":10}`)
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	cr.svc.EXPECT().AddLinks(gomock.Any(), uint64(1), uint64(7), uint64(3), uint64(4), []uint64{10}).
		Return(&service.ValidationError{Msg: "link 10 cannot be added"})
	w = doJSON(cr.r, http.MethodPost, "/api/campaigns/3/links", `{"channel_id":4,"link_ids":[10]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"link 10 cannot be added"}`, w.Body.String())
}

func TestCampaignHandler_CreateLinkInChannel201(t *testing.T) {
	cr := newCampaignRouter(t)
	cr.expectDomains()
	in := service.CreateLinkInChannelInput{OriginalURL: "https://example.com/p", Title: "P", Tags: []string{"a"}}
	link := &model.Link{ID: 9, ShortCode: "xyz", OriginalURL: "https://example.com/p?utm_source=Email"}
	cr.svc.EXPECT().CreateLinkInChannel(gomock.Any(), uint64(1), uint64(7), uint64(3), uint64(4), in).Return(link, nil)
	cr.links.EXPECT().GetDetails(gomock.Any(), uint64(1), uint64(9)).Return(&service.LinkDetails{
		Link: *link, Tags: []string{"a"}, RoutingRules: []model.RoutingRule{}, History: []model.Link{},
	}, nil)
	w := doJSON(cr.r, http.MethodPost, "/api/campaigns/3/channels/4/links",
		`{"original_url":"https://example.com/p","title":"P","tags":["a"]}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"short_url":"https://sho.rt/xyz"`)
	assert.Contains(t, w.Body.String(), `"tags":["a"]`)
	assert.Contains(t, w.Body.String(), `"routing_rules":[]`)

	cr.svc.EXPECT().CreateLinkInChannel(gomock.Any(), uint64(1), uint64(7), uint64(3), uint64(4), gomock.Any()).
		Return(nil, service.ErrShortCodeTaken)
	w = doJSON(cr.r, http.MethodPost, "/api/campaigns/3/channels/4/links", `{"original_url":"https://e.com","custom_code":"x"}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"That back-half is already taken"}`, w.Body.String())

	w = doJSON(cr.r, http.MethodPost, "/api/campaigns/3/channels/zz/links", `{}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCampaignHandler_Stats(t *testing.T) {
	cr := newCampaignRouter(t)
	stats := &service.CampaignStats{
		Totals: service.CampaignTotals{Links: 1, Engagements: 3},
		Channels: []service.ChannelStats{{ChannelID: 4, Name: "Email", Links: 1, Engagements: 3,
			Series: []service.SeriesPoint{{Date: "2026-10-24", Clicks: 3}}}},
	}
	cr.svc.EXPECT().Stats(gomock.Any(), uint64(7), uint64(3), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ uint64, _ uint64, q service.StatsQuery) (*service.CampaignStats, error) {
			assert.Equal(t, "2026-10-20", q.From.Format("2006-01-02"))
			assert.Equal(t, "2026-10-24", q.To.Format("2006-01-02"))
			assert.Equal(t, "day", q.Unit)
			return stats, nil
		})
	w := doJSON(cr.r, http.MethodGet, "/api/campaigns/3/stats?from=2026-10-20&to=2026-10-24&unit=day", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"totals":{"links":1,"engagements":3},"channels":[{"channel_id":4,"name":"Email","links":1,
		"engagements":3,"series":[{"date":"2026-10-24","clicks":3}]}]}`, w.Body.String())
}

func TestCampaignHandler_Stats_BadRange400(t *testing.T) {
	cr := newCampaignRouter(t)
	w := doJSON(cr.r, http.MethodGet, "/api/campaigns/3/stats?from=2026-10-24&to=2026-10-20", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"from must not be after to"}`, w.Body.String())

	w = doJSON(cr.r, http.MethodGet, "/api/campaigns/3/stats?from=yesterday", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChannelHandler_ListCreate(t *testing.T) {
	cr := newCampaignRouter(t)
	cr.svc.EXPECT().ListChannels(gomock.Any(), uint64(7)).Return(nil, nil)
	w := doJSON(cr.r, http.MethodGet, "/api/channels", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"channels":[]}`, w.Body.String())

	cr.svc.EXPECT().CreateChannel(gomock.Any(), uint64(7), "SMS").
		Return(&model.Channel{ID: 8, GroupID: 7, Name: "SMS", CreatedAt: campaignTS}, nil)
	w = doJSON(cr.r, http.MethodPost, "/api/channels", `{"name":"SMS"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"name":"SMS"`)
}

func TestChannelHandler_Duplicate409(t *testing.T) {
	cr := newCampaignRouter(t)
	cr.svc.EXPECT().CreateChannel(gomock.Any(), uint64(7), "Email").Return(nil, repository.ErrChannelExists)
	w := doJSON(cr.r, http.MethodPost, "/api/channels", `{"name":"Email"}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"channel already exists"}`, w.Body.String())
}
