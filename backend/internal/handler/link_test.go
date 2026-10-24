package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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

// fakeInvalidator records InvalidateLink calls as "domain/code".
type fakeInvalidator struct{ calls []string }

func (f *fakeInvalidator) InvalidateLink(_ context.Context, link *model.Link, domainName string) error {
	f.calls = append(f.calls, domainName+"/"+link.ShortCode)
	return nil
}

type linkRouter struct {
	r       *gin.Engine
	svc     *mocks.MockLinkService
	domains *repomocks.MockDomainRepository
	inv     *fakeInvalidator
}

func newLinkRouter(t *testing.T, favicons bool) *linkRouter {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	lr := &linkRouter{
		svc:     mocks.NewMockLinkService(ctrl),
		domains: repomocks.NewMockDomainRepository(ctrl),
		inv:     &fakeInvalidator{},
	}
	cfg := &config.Config{}
	cfg.URLs.BaseURL = "https://sho.rt"
	cfg.UI.FaviconService = &favicons
	h := NewLinkHandler(lr.svc, lr.inv, lr.domains, cfg)
	lr.r = gin.New()
	links := lr.r.Group("/api/links", withGroup)
	links.POST("", h.Create)
	links.GET("", h.List)
	links.GET("/export", h.Export)
	links.PATCH("/bulk", h.Bulk)
	links.GET("/:id", h.Get)
	links.PUT("/:id", h.Update)
	links.DELETE("/:id", h.Delete)
	links.POST("/:id/back-half", h.EditBackHalf)
	links.PUT("/:id/tags", h.SetTags)
	links.GET("/:id/routing-rules", h.GetRoutingRules)
	links.PUT("/:id/routing-rules", h.SetRoutingRules)
	return lr
}

func (lr *linkRouter) expectDomains(domains ...*model.Domain) {
	lr.domains.EXPECT().ListByUserID(gomock.Any(), uint64(1)).Return(domains, nil).AnyTimes()
}

var linkCreated = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func TestLinkHandler_Bulk_Archive(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.expectDomains(&model.Domain{ID: 5, Domain: "go.brand.test"})
	lr.svc.EXPECT().SetArchived(gomock.Any(), uint64(1), uint64(7), []uint64{3, 4}, true).
		Return([]model.Link{{ID: 3, ShortCode: "a"}, {ID: 4, ShortCode: "b", DomainID: u64p(5)}}, nil)

	w := doJSON(lr.r, http.MethodPatch, "/api/links/bulk", `{"action":"archive","ids":[3,4]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"updated":2}`, w.Body.String())
	assert.Equal(t, []string{"/a", "go.brand.test/b"}, lr.inv.calls)
}

func TestLinkHandler_Bulk_Unarchive(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.expectDomains()
	lr.svc.EXPECT().SetArchived(gomock.Any(), uint64(1), uint64(7), []uint64{3}, false).
		Return([]model.Link{{ID: 3, ShortCode: "a"}}, nil)
	w := doJSON(lr.r, http.MethodPatch, "/api/links/bulk", `{"action":"unarchive","ids":[3]}`)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestLinkHandler_Bulk_EditTags(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().BulkTags(gomock.Any(), uint64(7), []uint64{3, 4}, []string{"x"}, []string{"y"}).Return(nil)
	w := doJSON(lr.r, http.MethodPatch, "/api/links/bulk", `{"action":"edit_tags","ids":[3,4],"add_tags":["x"],"remove_tags":["y"]}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"updated":2}`, w.Body.String())
}

func TestLinkHandler_Bulk_ForeignIDs404(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().BulkTags(gomock.Any(), uint64(7), []uint64{3, 99}, []string{"x"}, nil).Return(service.ErrLinkNotFound)
	w := doJSON(lr.r, http.MethodPatch, "/api/links/bulk", `{"action":"edit_tags","ids":[3,99],"add_tags":["x"]}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"link not found"}`, w.Body.String())
}

func TestLinkHandler_Bulk_BadRequest(t *testing.T) {
	lr := newLinkRouter(t, true)
	for _, body := range []string{`{"action":"explode","ids":[1]}`, `{"action":"archive","ids":[]}`, `nope`} {
		w := doJSON(lr.r, http.MethodPatch, "/api/links/bulk", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
	}
}

func TestLinkHandler_Bulk_EditTagsCapped(t *testing.T) {
	lr := newLinkRouter(t, true)
	tags := make([]string, 0, 51)
	for i := 0; i < 51; i++ {
		tags = append(tags, fmt.Sprintf("%q", fmt.Sprintf("t%d", i)))
	}
	body := `{"action":"edit_tags","ids":[1],"add_tags":[` + strings.Join(tags[:30], ",") +
		`],"remove_tags":[` + strings.Join(tags[30:], ",") + `]}`
	w := doJSON(lr.r, http.MethodPatch, "/api/links/bulk", body)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"edit_tags accepts at most 50 tags per request"}`, w.Body.String())
}

func TestLinkHandler_Create_ExpirationTooSoon400(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).Return(nil, service.ErrExpirationTooSoon)
	w := doJSON(lr.r, http.MethodPost, "/api/links", `{"original_url":"https://example.com","expires_at":"2020-01-01T00:00:00Z"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"expiration must be at least 5 minutes in the future"}`, w.Body.String())
}

func TestLinkHandler_Export_CSVHeader(t *testing.T) {
	lr := newLinkRouter(t, true)
	archived := false
	lr.svc.EXPECT().Export(gomock.Any(), uint64(7), repository.LinkFilter{Archived: &archived, Tags: []string{"a", "b"}}).
		Return([][]string{{"https://sho.rt/abc", "", "2026-10-24T12:00:00Z", "Hi, there", "https://e.com", "3", "active"}}, nil)

	w := doJSON(lr.r, http.MethodGet, "/api/links/export?tags=a,b", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, `attachment; filename="links.csv"`, w.Header().Get("Content-Disposition"))
	assert.True(t, strings.HasPrefix(w.Header().Get("Content-Type"), "text/csv"))
	lines := strings.Split(strings.TrimRight(w.Body.String(), "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "Link,Custom Link,Date created,Title,Destination URL,Engagements,Status", lines[0])
	assert.Equal(t, `https://sho.rt/abc,,2026-10-24T12:00:00Z,"Hi, there",https://e.com,3,active`, lines[1])
}

func TestLinkHandler_Delete_409(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().Delete(gomock.Any(), uint64(1), uint64(3)).Return(nil, service.ErrLinkNotDeletable)
	w := doJSON(lr.r, http.MethodDelete, "/api/links/3", "")
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"This link can't be deleted. You can hide it instead."}`, w.Body.String())
	assert.Empty(t, lr.inv.calls)
}

func TestLinkHandler_Delete_InvalidatesCache(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.expectDomains()
	lr.svc.EXPECT().Delete(gomock.Any(), uint64(1), uint64(3)).Return(&model.Link{ID: 3, ShortCode: "abc"}, nil)
	w := doJSON(lr.r, http.MethodDelete, "/api/links/3", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, []string{"/abc"}, lr.inv.calls)
}

func TestLinkHandler_List_ParsesFilters(t *testing.T) {
	lr := newLinkRouter(t, false)
	lr.expectDomains()
	archived, custom := true, false
	after := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC) // created_before is inclusive of the day
	lr.svc.EXPECT().List(gomock.Any(), uint64(7), service.ListLinksParams{
		Filter: repository.LinkFilter{
			Query: "sale", Tags: []string{"a", "b", "c"}, Archived: &archived, CustomBackHalf: &custom,
			Expiration: "expiring", CampaignID: u64p(4), CreatedAfter: &after, CreatedBefore: &before, Sort: "clicks",
		},
		Page: 2, Limit: 10,
	}).Return(&service.ListLinksResult{Links: []model.LinkWithStats{}, Page: 2}, nil)

	w := doJSON(lr.r, http.MethodGet, "/api/links?q=sale&tags=a,b&tags=c&archived=on&custom_back_half=off&expiration=expiring"+
		"&has_qr=both&campaign_id=4&created_after=2026-10-01&created_before=2026-10-24&sort=clicks&page=2&limit=10", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"links":[],"total":0,"page":2,"total_pages":0}`, w.Body.String())
}

func TestLinkHandler_List_BadParams400(t *testing.T) {
	lr := newLinkRouter(t, true)
	for _, q := range []string{"archived=maybe", "custom_back_half=x", "expiration=soon", "has_qr=1", "sort=name",
		"created_after=2026-13-01", "created_before=yesterday", "page=abc", "page=10001", "limit=x", "campaign_id=z",
		"created_after=2026-10-05&created_before=2026-10-01"} {
		w := doJSON(lr.r, http.MethodGet, "/api/links?"+q, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, q)
	}
}

func TestLinkHandler_List_ResponseShape(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.expectDomains()
	host := "example.com"
	title := "Example"
	lr.svc.EXPECT().List(gomock.Any(), uint64(7), gomock.Any()).Return(&service.ListLinksResult{
		Links: []model.LinkWithStats{{
			Link: model.Link{ID: 3, UserID: 1, GroupID: 7, ShortCode: "abc", OriginalURL: "https://example.com/x",
				LongURLHost: &host, Title: &title, IsActive: true, Source: "link", CreatedAt: linkCreated, UpdatedAt: linkCreated},
			TotalClicks: 5, Tags: []string{"news"},
		}},
		Total: 1, Page: 1, TotalPages: 1,
	}, nil)

	w := doJSON(lr.r, http.MethodGet, "/api/links", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"links":[{"id":3,"user_id":1,"group_id":7,"short_code":"abc","original_url":"https://example.com/x",
		"title":"Example","expires_at":null,"is_active":true,"archived":false,"has_custom_back_half":false,"redirect_count":0,
		"source":"link","created_at":"2026-10-24T12:00:00Z","updated_at":"2026-10-24T12:00:00Z",
		"short_url":"https://sho.rt/abc","tags":["news"],"total_clicks":5,
		"favicon_url":"https://www.google.com/s2/favicons?domain=example.com&sz=32"}],
		"total":1,"page":1,"total_pages":1}`, w.Body.String())
}

func TestLinkHandler_Get_Details(t *testing.T) {
	lr := newLinkRouter(t, false)
	lr.expectDomains()
	lr.svc.EXPECT().GetDetails(gomock.Any(), uint64(1), uint64(30)).Return(&service.LinkDetails{
		Link:         model.Link{ID: 30, UserID: 1, GroupID: 7, ShortCode: "new", OriginalURL: "https://e.com", Source: "link", CreatedAt: linkCreated, UpdatedAt: linkCreated},
		Tags:         []string{},
		TotalClicks:  2,
		RoutingRules: []model.RoutingRule{},
		History:      []model.Link{{ID: 20, ShortCode: "old", CreatedAt: linkCreated}},
	}, nil)

	w := doJSON(lr.r, http.MethodGet, "/api/links/30", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"id":30,"user_id":1,"group_id":7,"short_code":"new","original_url":"https://e.com",
		"expires_at":null,"is_active":false,"archived":false,"has_custom_back_half":false,"redirect_count":0,
		"source":"link","created_at":"2026-10-24T12:00:00Z","updated_at":"2026-10-24T12:00:00Z",
		"short_url":"https://sho.rt/new","tags":[],"total_clicks":2,"favicon_url":"",
		"routing_rules":[],"history":[{"id":20,"short_url":"https://sho.rt/old","created_at":"2026-10-24T12:00:00Z"}]}`, w.Body.String())
}

func TestLinkHandler_Get_NotFound(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().GetDetails(gomock.Any(), uint64(1), uint64(30)).Return(nil, service.ErrNotLinkOwner)
	w := doJSON(lr.r, http.MethodGet, "/api/links/30", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestLinkHandler_Create_PassesGroup(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.expectDomains()
	lr.svc.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ uint64, in service.CreateLinkInput) (*model.Link, error) {
			assert.Equal(t, uint64(7), in.GroupID)
			assert.Equal(t, []string{"a"}, in.Tags)
			require.NotNil(t, in.UTM)
			assert.Equal(t, "news", in.UTM.Source)
			require.Len(t, in.RoutingRules, 1)
			return &model.Link{ID: 9, ShortCode: "abc"}, nil
		})
	lr.svc.EXPECT().GetDetails(gomock.Any(), uint64(1), uint64(9)).Return(&service.LinkDetails{
		Link: model.Link{ID: 9, ShortCode: "abc"}, Tags: []string{"a"}, RoutingRules: []model.RoutingRule{}, History: []model.Link{},
	}, nil)

	w := doJSON(lr.r, http.MethodPost, "/api/links", `{"original_url":"https://e.com","tags":["a"],"utm":{"source":"news"},
		"routing_rules":[{"long_url":"https://m.e.com","device_match":["mobile"]}],"group_id":99}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"short_url":"https://sho.rt/abc"`)
}

func TestLinkHandler_Create_BackHalfTaken409(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).Return(nil, service.ErrShortCodeTaken)
	w := doJSON(lr.r, http.MethodPost, "/api/links", `{"original_url":"https://e.com","custom_code":"taken"}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"That back-half is already taken"}`, w.Body.String())
}

func TestLinkHandler_EditBackHalf(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.expectDomains()
	lr.svc.EXPECT().EditBackHalf(gomock.Any(), uint64(1), uint64(3), u64p(5), "New-Half").
		Return(&model.Link{ID: 20, ShortCode: "New-Half"}, nil)
	lr.svc.EXPECT().GetDetails(gomock.Any(), uint64(1), uint64(20)).Return(&service.LinkDetails{
		Link: model.Link{ID: 20, ShortCode: "New-Half"}, Tags: []string{}, RoutingRules: []model.RoutingRule{}, History: []model.Link{},
	}, nil)
	w := doJSON(lr.r, http.MethodPost, "/api/links/3/back-half", `{"domain_id":5,"back_half":"New-Half"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"id":20`)
}

func TestLinkHandler_EditBackHalf_Taken409(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().EditBackHalf(gomock.Any(), uint64(1), uint64(3), (*uint64)(nil), "dup").Return(nil, service.ErrShortCodeTaken)
	w := doJSON(lr.r, http.MethodPost, "/api/links/3/back-half", `{"back_half":"dup"}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"That back-half is already taken"}`, w.Body.String())
}

func TestLinkHandler_Update_InvalidatesCache(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.expectDomains()
	lr.svc.EXPECT().Update(gomock.Any(), uint64(1), uint64(3), gomock.Any()).
		Return(&model.Link{ID: 3, ShortCode: "abc"}, &model.Link{ID: 3, ShortCode: "abc"}, nil)
	lr.svc.EXPECT().GetDetails(gomock.Any(), uint64(1), uint64(3)).Return(&service.LinkDetails{
		Link: model.Link{ID: 3, ShortCode: "abc", RedirectCount: 1}, Tags: []string{}, RoutingRules: []model.RoutingRule{}, History: []model.Link{},
	}, nil)
	w := doJSON(lr.r, http.MethodPut, "/api/links/3", `{"original_url":"https://new.example.com"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []string{"/abc"}, lr.inv.calls)
	assert.Contains(t, w.Body.String(), `"redirect_count":1`)
}

func TestLinkHandler_SetTags(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().SetTags(gomock.Any(), uint64(1), uint64(3), []string{"a", "b"}).Return([]string{"a", "b"}, nil)
	w := doJSON(lr.r, http.MethodPut, "/api/links/3/tags", `{"tags":["a","b"]}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"tags":["a","b"]}`, w.Body.String())
}

func TestLinkHandler_Update_InvalidatesOldDomain(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.expectDomains(&model.Domain{ID: 5, Domain: "old.test"}, &model.Domain{ID: 6, Domain: "new.test"})
	lr.svc.EXPECT().Update(gomock.Any(), uint64(1), uint64(3), gomock.Any()).
		Return(&model.Link{ID: 3, ShortCode: "abc", DomainID: u64p(6)}, &model.Link{ID: 3, ShortCode: "abc", DomainID: u64p(5)}, nil)
	lr.svc.EXPECT().GetDetails(gomock.Any(), uint64(1), uint64(3)).Return(&service.LinkDetails{
		Link: model.Link{ID: 3, ShortCode: "abc", DomainID: u64p(6)}, Tags: []string{}, RoutingRules: []model.RoutingRule{}, History: []model.Link{},
	}, nil)
	w := doJSON(lr.r, http.MethodPut, "/api/links/3", `{"domain_id":6}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.ElementsMatch(t, []string{"new.test/abc", "old.test/abc"}, lr.inv.calls)
}

func TestLinkHandler_Create_UnverifiedDomain400(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).Return(nil, service.ErrDomainNotVerified)
	w := doJSON(lr.r, http.MethodPost, "/api/links", `{"original_url":"https://e.com","domain_id":5}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"domain is not verified"}`, w.Body.String())
}

func TestLinkHandler_Export_FormulaTitleEscaped(t *testing.T) {
	// End-to-end through the real service: a scraped title starting with "="
	// is exported with a leading apostrophe.
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	links := repomocks.NewMockLinkRepository(ctrl)
	title := `=HYPERLINK("x")`
	links.EXPECT().ListByGroup(gomock.Any(), uint64(7), gomock.Any(), gomock.Any(), 0).Return([]model.LinkWithStats{
		{Link: model.Link{ID: 1, ShortCode: "abc", OriginalURL: "https://e.com", Title: &title, CreatedAt: linkCreated}},
	}, int64(1), nil)
	svc := service.NewLinkService(service.LinkServiceDeps{Links: links, BaseURL: "https://sho.rt"})
	cfg := &config.Config{}
	cfg.URLs.BaseURL = "https://sho.rt"
	h := NewLinkHandler(svc, &fakeInvalidator{}, repomocks.NewMockDomainRepository(ctrl), cfg)
	r := gin.New()
	r.GET("/api/links/export", withGroup, h.Export)

	w := doJSON(r, http.MethodGet, "/api/links/export", "")
	require.Equal(t, http.StatusOK, w.Code)
	lines := strings.Split(strings.TrimRight(w.Body.String(), "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, `https://sho.rt/abc,,2026-10-24T12:00:00Z,"'=HYPERLINK(""x"")",https://e.com,0,active`, lines[1])
}

func u64p(v uint64) *uint64 { return &v }

func TestLinkHandler_GetRoutingRules(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().GetRoutingRules(gomock.Any(), uint64(1), uint64(3)).Return([]model.RoutingRule{
		{ID: 9, LinkID: 3, Position: 0, LongURL: "https://m.e.com", DeviceMatch: model.StringList{"mobile"},
			OSMatch: model.StringList{}, CountryMatch: model.StringList{}, CountryExclude: model.StringList{}},
	}, nil)
	w := doJSON(lr.r, http.MethodGet, "/api/links/3/routing-rules", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"routing_rules":[{"id":9,"link_id":3,"position":0,"long_url":"https://m.e.com",
		"device_match":["mobile"],"os_match":[],"country_match":[],"country_exclude":[]}]}`, w.Body.String())

	lr.svc.EXPECT().GetRoutingRules(gomock.Any(), uint64(1), uint64(4)).Return(nil, service.ErrNotLinkOwner)
	w = doJSON(lr.r, http.MethodGet, "/api/links/4/routing-rules", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestLinkHandler_SetRoutingRules_InvalidatesCache(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.expectDomains(&model.Domain{ID: 5, Domain: "go.brand.test"})
	lr.svc.EXPECT().SetRoutingRules(gomock.Any(), uint64(1), uint64(3), []service.RoutingRuleInput{
		{LongURL: "https://m.e.com", DeviceMatch: []string{"mobile"}},
	}).Return([]model.RoutingRule{
		{ID: 9, LinkID: 3, Position: 0, LongURL: "https://m.e.com", DeviceMatch: model.StringList{"mobile"},
			OSMatch: model.StringList{}, CountryMatch: model.StringList{}, CountryExclude: model.StringList{}},
	}, nil)
	lr.svc.EXPECT().GetByID(gomock.Any(), uint64(1), uint64(3)).Return(&model.Link{ID: 3, ShortCode: "abc", DomainID: u64p(5)}, nil)

	w := doJSON(lr.r, http.MethodPut, "/api/links/3/routing-rules",
		`{"routing_rules":[{"long_url":"https://m.e.com","device_match":["mobile"]}]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"routing_rules":[{"id":9,"link_id":3,"position":0,"long_url":"https://m.e.com",
		"device_match":["mobile"],"os_match":[],"country_match":[],"country_exclude":[]}]}`, w.Body.String())
	assert.Equal(t, []string{"go.brand.test/abc"}, lr.inv.calls)
}

func TestLinkHandler_SetRoutingRules_Invalid400(t *testing.T) {
	lr := newLinkRouter(t, true)
	lr.svc.EXPECT().SetRoutingRules(gomock.Any(), uint64(1), uint64(3), gomock.Any()).
		Return(nil, fmt.Errorf("%w: rule 1: device_match value \"phone\" must be one of mobile, tablet, desktop", service.ErrInvalidRoutingRule))
	w := doJSON(lr.r, http.MethodPut, "/api/links/3/routing-rules", `{"routing_rules":[{"long_url":"https://e.com","device_match":["phone"]}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "device_match")
	assert.Empty(t, lr.inv.calls)

	w = doJSON(lr.r, http.MethodPut, "/api/links/3/routing-rules", `nope`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLinkHandler_Get_IncludesQRCodeID(t *testing.T) {
	lr := newLinkRouter(t, false)
	lr.expectDomains()
	lr.svc.EXPECT().GetDetails(gomock.Any(), uint64(1), uint64(30)).Return(&service.LinkDetails{
		Link:     model.Link{ID: 30, ShortCode: "new", OriginalURL: "https://e.com", Source: "link"},
		QRCodeID: u64p(9),
	}, nil)
	w := doJSON(lr.r, http.MethodGet, "/api/links/30", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"qr_code_id":9`)

	lr.svc.EXPECT().List(gomock.Any(), uint64(7), gomock.Any()).Return(&service.ListLinksResult{
		Links: []model.LinkWithStats{{Link: model.Link{ID: 30, ShortCode: "new"}, QRCodeID: u64p(9)}},
	}, nil)
	w = doJSON(lr.r, http.MethodGet, "/api/links", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"qr_code_id":9`)
}
