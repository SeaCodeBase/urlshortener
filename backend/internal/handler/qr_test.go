package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"

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

type qrRouter struct {
	r       *gin.Engine
	svc     *mocks.MockQRService
	domains *repomocks.MockDomainRepository
	inv     *fakeInvalidator
}

func newQRRouter(t *testing.T) *qrRouter {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	qr := &qrRouter{
		svc:     mocks.NewMockQRService(ctrl),
		domains: repomocks.NewMockDomainRepository(ctrl),
		inv:     &fakeInvalidator{},
	}
	favicons := false
	cfg := &config.Config{}
	cfg.URLs.BaseURL = "https://sho.rt"
	cfg.UI.FaviconService = &favicons
	h := NewQRHandler(qr.svc, qr.inv, qr.domains, cfg)
	qr.r = gin.New()
	g := qr.r.Group("/api/qr-codes", withGroup)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/export", h.Export)
	g.PATCH("/bulk", h.Bulk)
	g.GET("/:id", h.Get)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.PUT("/:id/tags", h.SetTags)
	return qr
}

func (qr *qrRouter) expectDomains(domains ...*model.Domain) {
	qr.domains.EXPECT().ListByUserID(gomock.Any(), uint64(1)).Return(domains, nil).AnyTimes()
}

func qrDetails(id uint64, link model.Link) *service.QRCodeDetails {
	return &service.QRCodeDetails{
		QRCode: model.QRCode{ID: id, UserID: 1, GroupID: 7, LinkID: link.ID,
			RenderCustomizations: model.JSONMap{}, CreatedAt: linkCreated, UpdatedAt: linkCreated},
		Link: link,
		Tags: []string{},
	}
}

func TestQRHandler_Create_201_WithQRURL(t *testing.T) {
	qr := newQRRouter(t)
	qr.expectDomains()
	linkTitle := "Example"
	link := model.Link{ID: 3, UserID: 1, GroupID: 7, ShortCode: "abc", OriginalURL: "https://example.com",
		Title: &linkTitle, Source: model.LinkSourceQR, CreatedAt: linkCreated, UpdatedAt: linkCreated}
	created := qrDetails(9, link)
	qr.svc.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ uint64, in service.CreateQRCodeInput) (*service.QRCodeDetails, error) {
			assert.Equal(t, uint64(7), in.GroupID)
			assert.Equal(t, "https://example.com", in.LongURL)
			return created, nil
		})
	full := qrDetails(9, link)
	full.RoutingRules = []model.RoutingRule{}
	full.TotalScans = 4
	qr.svc.EXPECT().Get(gomock.Any(), uint64(1), uint64(9)).Return(full, nil)

	w := doJSON(qr.r, http.MethodPost, "/api/qr-codes", `{"long_url":"https://example.com"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.JSONEq(t, `{"id":9,"user_id":1,"group_id":7,"link_id":3,"title":"Example","render_customizations":{},
		"archived":false,"created_at":"2026-10-24T12:00:00Z","updated_at":"2026-10-24T12:00:00Z","tags":[],
		"total_scans":4,"short_url":"https://sho.rt/abc","qr_url":"https://sho.rt/abc?r=qr",
		"original_url":"https://example.com","favicon_url":"","expires_at":null,"link_source":"qr",
		"image_url":"/api/qr-codes/9/image",
		"link":{"id":3,"user_id":1,"group_id":7,"short_code":"abc","original_url":"https://example.com","title":"Example",
			"expires_at":null,"is_active":false,"archived":false,"has_custom_back_half":false,"redirect_count":0,
			"source":"qr","created_at":"2026-10-24T12:00:00Z","updated_at":"2026-10-24T12:00:00Z",
			"short_url":"https://sho.rt/abc","tags":[],"total_clicks":0,"favicon_url":"","qr_code_id":9,"routing_rules":[]}}`,
		w.Body.String())
}

func TestQRHandler_Create_409WhenLinkHasQR(t *testing.T) {
	qr := newQRRouter(t)
	qr.svc.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).Return(nil, service.ErrQRCodeExistsForLink)
	w := doJSON(qr.r, http.MethodPost, "/api/qr-codes", `{"link_id":3}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"This link already has a QR Code"}`, w.Body.String())
}

func TestQRHandler_Create_400s(t *testing.T) {
	qr := newQRRouter(t)
	for _, err := range []error{service.ErrQRLinkRequired, service.ErrInvalidRenderCustomizations,
		service.ErrInvalidURL, service.ErrInvalidShortCode, service.ErrExpirationTooSoon,
		service.ErrInvalidRoutingRule, service.ErrDomainNotFound} {
		qr.svc.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).Return(nil, err)
		w := doJSON(qr.r, http.MethodPost, "/api/qr-codes", `{}`)
		assert.Equal(t, http.StatusBadRequest, w.Code, err.Error())
	}
	qr.svc.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).Return(nil, service.ErrShortCodeTaken)
	assert.Equal(t, http.StatusConflict, doJSON(qr.r, http.MethodPost, "/api/qr-codes", `{}`).Code)
}

func TestQRHandler_Get_NotFound(t *testing.T) {
	qr := newQRRouter(t)
	qr.svc.EXPECT().Get(gomock.Any(), uint64(1), uint64(9)).Return(nil, service.ErrQRCodeNotFound)
	w := doJSON(qr.r, http.MethodGet, "/api/qr-codes/9", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"QR code not found"}`, w.Body.String())
	assert.Equal(t, http.StatusBadRequest, doJSON(qr.r, http.MethodGet, "/api/qr-codes/x", "").Code)
}

func TestQRHandler_List_ParsesFilters(t *testing.T) {
	qr := newQRRouter(t)
	qr.expectDomains()
	archived, hasLink := false, true
	qrTitle := "Poster"
	link := model.Link{ID: 3, ShortCode: "abc", OriginalURL: "https://e.com", Source: model.LinkSourceLink}
	item := qrDetails(9, link)
	item.Title = &qrTitle
	item.Tags = []string{"x"}
	item.TotalScans = 2
	qr.svc.EXPECT().List(gomock.Any(), uint64(7), service.ListQRParams{
		Filter: repository.QRFilter{Query: "sale", Tags: []string{"a", "b"}, Archived: &archived,
			Expiration: "expired", HasLink: &hasLink, Sort: "scans"},
		Page: 2, Limit: 10,
	}).Return(&service.ListQRResult{QRCodes: []service.QRCodeDetails{*item}, Total: 11, Page: 2, TotalPages: 2}, nil)

	w := doJSON(qr.r, http.MethodGet, "/api/qr-codes?q=sale&tags[]=a&tags[]=b&expiration=expired&has_link=on&sort=scans&page=2&limit=10", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"qr_codes":[{"id":9,"user_id":1,"group_id":7,"link_id":3,"title":"Poster","render_customizations":{},
		"archived":false,"created_at":"2026-10-24T12:00:00Z","updated_at":"2026-10-24T12:00:00Z","tags":["x"],
		"total_scans":2,"short_url":"https://sho.rt/abc","qr_url":"https://sho.rt/abc?r=qr",
		"original_url":"https://e.com","favicon_url":"","expires_at":null,"link_source":"link",
		"image_url":"/api/qr-codes/9/image"}],"total":11,"page":2,"total_pages":2}`, w.Body.String())
}

func TestQRHandler_List_BadParams400(t *testing.T) {
	qr := newQRRouter(t)
	for _, q := range []string{"archived=maybe", "has_link=1", "expiration=soon", "sort=clicks",
		"created_after=nope", "created_before=2026-13-01", "page=x", "page=10001", "limit=x"} {
		w := doJSON(qr.r, http.MethodGet, "/api/qr-codes?"+q, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, q)
	}
}

func TestQRHandler_Bulk_Archive(t *testing.T) {
	qr := newQRRouter(t)
	qr.svc.EXPECT().SetArchived(gomock.Any(), uint64(1), uint64(7), []uint64{9, 10}, true).Return(2, nil)
	w := doJSON(qr.r, http.MethodPatch, "/api/qr-codes/bulk", `{"action":"archive","ids":[9,10]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"updated":2}`, w.Body.String())
	assert.Empty(t, qr.inv.calls)
}

func TestQRHandler_Bulk_EditTagsAndErrors(t *testing.T) {
	qr := newQRRouter(t)
	qr.svc.EXPECT().BulkTags(gomock.Any(), uint64(7), []uint64{9, 9, 10}, []string{"x"}, []string{"y"}).Return(nil)
	w := doJSON(qr.r, http.MethodPatch, "/api/qr-codes/bulk", `{"action":"edit_tags","ids":[9,9,10],"add_tags":["x"],"remove_tags":["y"]}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"updated":2}`, w.Body.String())

	qr.svc.EXPECT().SetArchived(gomock.Any(), uint64(1), uint64(7), []uint64{99}, false).Return(0, service.ErrQRCodeNotFound)
	w = doJSON(qr.r, http.MethodPatch, "/api/qr-codes/bulk", `{"action":"unarchive","ids":[99]}`)
	assert.Equal(t, http.StatusNotFound, w.Code)

	for _, body := range []string{`{"action":"explode","ids":[1]}`, `{"action":"archive","ids":[]}`, `nope`} {
		assert.Equal(t, http.StatusBadRequest, doJSON(qr.r, http.MethodPatch, "/api/qr-codes/bulk", body).Code, body)
	}
}

func TestQRHandler_Delete_InvalidatesLink(t *testing.T) {
	qr := newQRRouter(t)
	qr.expectDomains(&model.Domain{ID: 5, Domain: "go.brand.test"})
	link := model.Link{ID: 3, ShortCode: "abc", DomainID: u64p(5), Source: model.LinkSourceQR}
	qr.svc.EXPECT().Delete(gomock.Any(), uint64(1), uint64(9)).Return(qrDetails(9, link), true, nil)
	w := doJSON(qr.r, http.MethodDelete, "/api/qr-codes/9", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, []string{"go.brand.test/abc"}, qr.inv.calls)
}

func TestQRHandler_Delete_NotFound(t *testing.T) {
	qr := newQRRouter(t)
	qr.svc.EXPECT().Delete(gomock.Any(), uint64(1), uint64(9)).Return(nil, false, service.ErrQRCodeNotFound)
	assert.Equal(t, http.StatusNotFound, doJSON(qr.r, http.MethodDelete, "/api/qr-codes/9", "").Code)
	assert.Empty(t, qr.inv.calls)
}

func TestQRHandler_Update_InvalidatesLink(t *testing.T) {
	qr := newQRRouter(t)
	qr.expectDomains()
	link := model.Link{ID: 3, ShortCode: "abc", OriginalURL: "https://new.example", Source: model.LinkSourceQR}
	prev := model.Link{ID: 3, ShortCode: "abc", OriginalURL: "https://old.example", Source: model.LinkSourceQR}
	qr.svc.EXPECT().Update(gomock.Any(), uint64(1), uint64(9), gomock.Any()).DoAndReturn(
		func(_ context.Context, _, _ uint64, in service.UpdateQRCodeInput) (*service.QRCodeDetails, *model.Link, error) {
			assert.Equal(t, "https://new.example", in.LongURL)
			return qrDetails(9, link), &prev, nil
		})
	qr.svc.EXPECT().Get(gomock.Any(), uint64(1), uint64(9)).Return(qrDetails(9, link), nil)
	w := doJSON(qr.r, http.MethodPut, "/api/qr-codes/9", `{"long_url":"https://new.example"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"original_url":"https://new.example"`)
	assert.Equal(t, []string{"/abc"}, qr.inv.calls)
}

func TestQRHandler_Update_PartialFailureInvalidatesPrev(t *testing.T) {
	qr := newQRRouter(t)
	qr.expectDomains()
	prev := model.Link{ID: 3, ShortCode: "abc"}
	qr.svc.EXPECT().Update(gomock.Any(), uint64(1), uint64(9), gomock.Any()).Return(nil, &prev, service.ErrInvalidRoutingRule)
	w := doJSON(qr.r, http.MethodPut, "/api/qr-codes/9", `{"long_url":"https://new.example","routing_rules":[]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, []string{"/abc"}, qr.inv.calls)
}

func TestQRHandler_SetTags(t *testing.T) {
	qr := newQRRouter(t)
	d := qrDetails(9, model.Link{ID: 3, ShortCode: "abc"})
	d.Tags = []string{"a", "b"}
	qr.svc.EXPECT().Update(gomock.Any(), uint64(1), uint64(9), gomock.Any()).DoAndReturn(
		func(_ context.Context, _, _ uint64, in service.UpdateQRCodeInput) (*service.QRCodeDetails, *model.Link, error) {
			require.NotNil(t, in.Tags)
			assert.Equal(t, []string{"a", "b"}, *in.Tags)
			assert.Nil(t, in.Title)
			assert.Nil(t, in.RoutingRules)
			return d, nil, nil
		})
	w := doJSON(qr.r, http.MethodPut, "/api/qr-codes/9/tags", `{"tags":["a","b"]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"tags":["a","b"]}`, w.Body.String())
}

func TestQRHandler_Export_CSVHeader(t *testing.T) {
	qr := newQRRouter(t)
	archived := false
	qr.svc.EXPECT().Export(gomock.Any(), uint64(7), repository.QRFilter{Archived: &archived, Tags: []string{"a"}}).
		Return([][]string{{"Poster, A", "https://sho.rt/abc", "https://e.com", "2026-10-24T12:00:00Z", "3", "Active"}}, nil)

	w := doJSON(qr.r, http.MethodGet, "/api/qr-codes/export?tags=a", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, `attachment; filename="qr-codes.csv"`, w.Header().Get("Content-Disposition"))
	assert.True(t, strings.HasPrefix(w.Header().Get("Content-Type"), "text/csv"))
	lines := strings.Split(strings.TrimRight(w.Body.String(), "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "Title,Short Link,Destination URL,Date created,Scans,Status", lines[0])
	assert.Equal(t, `"Poster, A",https://sho.rt/abc,https://e.com,2026-10-24T12:00:00Z,3,Active`, lines[1])
}
