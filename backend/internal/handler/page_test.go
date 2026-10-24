package handler

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var pageNow = time.Date(2026, 10, 25, 15, 0, 0, 0, time.UTC)

func newPageRouter(t *testing.T) (*gin.Engine, *mocks.MockPageService, *mocks.MockPagePublicService) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	pages := mocks.NewMockPageService(ctrl)
	public := mocks.NewMockPagePublicService(ctrl)
	h := NewPageHandler(pages, public)
	h.now = func() time.Time { return pageNow }
	r := gin.New()
	RegisterPageRoutes(r.Group("/api/pages", withGroup), h)
	return r, pages, public
}

func pageView(id uint64, slug string) *service.PageView {
	ts := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	return &service.PageView{ID: id, GroupID: 7, Slug: slug, Title: "Jose", Status: "draft",
		HasUnpublishedChanges: true, URL: "http://localhost:8080/m/" + slug, CreatedAt: ts, UpdatedAt: ts,
		Blocks: []service.BlockView{}}
}

func TestPageHandler_CreateConflict409(t *testing.T) {
	r, pages, _ := newPageRouter(t)

	pages.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), service.CreatePageInput{Slug: "jose", Title: "Jose", Template: "classic"}).
		Return(pageView(1, "jose"), nil)
	w := doJSON(r, http.MethodPost, "/api/pages", `{"slug":"jose","title":"Jose","template":"classic"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"slug":"jose"`)
	assert.Contains(t, w.Body.String(), `"blocks":[]`)

	pages.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), service.CreatePageInput{Slug: "jose"}).
		Return(nil, repository.ErrPageSlugTaken)
	w = doJSON(r, http.MethodPost, "/api/pages", `{"slug":"jose"}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"slug is already taken"}`, w.Body.String())

	pages.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), service.CreatePageInput{Slug: "api"}).
		Return(nil, &service.ValidationError{Msg: "this slug is reserved"})
	w = doJSON(r, http.MethodPost, "/api/pages", `{"slug":"api"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"this slug is reserved"}`, w.Body.String())

	w = doJSON(r, http.MethodPost, "/api/pages", `not json`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	pages.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), gomock.Any()).Return(nil, errors.New("db down"))
	w = doJSON(r, http.MethodPost, "/api/pages", `{"slug":"x-y-z"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"failed to create page"}`, w.Body.String())
}

func TestPageHandler_List(t *testing.T) {
	r, pages, _ := newPageRouter(t)
	list := []service.PageView{*pageView(1, "jose")}
	list[0].Blocks = nil
	pages.EXPECT().List(gomock.Any(), uint64(7), repository.PageFilter{Query: "jo"}).Return(list, nil)
	w := doJSON(r, http.MethodGet, "/api/pages?q=jo", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"total":1`)
	assert.NotContains(t, w.Body.String(), `"blocks"`)

	pages.EXPECT().List(gomock.Any(), uint64(7), repository.PageFilter{Archived: true}).Return(nil, nil)
	w = doJSON(r, http.MethodGet, "/api/pages?archived=on", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"pages":[],"total":0}`, w.Body.String())

	w = doJSON(r, http.MethodGet, "/api/pages?archived=yes", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPageHandler_CheckSlug(t *testing.T) {
	r, pages, _ := newPageRouter(t)
	pages.EXPECT().CheckSlug(gomock.Any(), "Jose").Return(service.SlugCheck{Slug: "jose", Available: false, Reason: "taken"}, nil)
	w := doJSON(r, http.MethodGet, "/api/pages/check-slug?slug=Jose", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"slug":"jose","available":false,"reason":"taken"}`, w.Body.String())

	pages.EXPECT().CheckSlug(gomock.Any(), "").Return(service.SlugCheck{Slug: "", Reason: "invalid"}, nil)
	w = doJSON(r, http.MethodGet, "/api/pages/check-slug", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"slug":"","available":false,"reason":"invalid"}`, w.Body.String())
}

func TestPageHandler_Get_OtherGroup404(t *testing.T) {
	r, pages, public := newPageRouter(t)
	pages.EXPECT().Get(gomock.Any(), uint64(7), uint64(9)).Return(nil, repository.ErrPageNotFound)
	pages.EXPECT().Update(gomock.Any(), uint64(1), uint64(7), uint64(9), gomock.Any()).Return(nil, repository.ErrPageNotFound)
	pages.EXPECT().SetArchived(gomock.Any(), uint64(1), uint64(7), uint64(9), true).Return(nil, repository.ErrPageNotFound)
	pages.EXPECT().Delete(gomock.Any(), uint64(1), uint64(7), uint64(9)).Return(repository.ErrPageNotFound)
	pages.EXPECT().Publish(gomock.Any(), uint64(1), uint64(7), uint64(9)).Return(nil, repository.ErrPageNotFound)
	pages.EXPECT().AddBlock(gomock.Any(), uint64(1), uint64(7), uint64(9), gomock.Any()).Return(nil, repository.ErrPageNotFound)
	pages.EXPECT().UpdateBlock(gomock.Any(), uint64(1), uint64(7), uint64(9), uint64(3), gomock.Any()).Return(nil, repository.ErrPageNotFound)
	pages.EXPECT().DeleteBlock(gomock.Any(), uint64(1), uint64(7), uint64(9), uint64(3)).Return(repository.ErrPageNotFound)
	pages.EXPECT().Reorder(gomock.Any(), uint64(1), uint64(7), uint64(9), []uint64{3}).Return(nil, repository.ErrPageNotFound)
	public.EXPECT().Stats(gomock.Any(), uint64(7), uint64(9), gomock.Any()).Return(nil, repository.ErrPageNotFound)

	for _, rq := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/pages/9", ""},
		{http.MethodPut, "/api/pages/9", `{"title":"x"}`},
		{http.MethodPatch, "/api/pages/9", `{"archived":true}`},
		{http.MethodDelete, "/api/pages/9", ""},
		{http.MethodPost, "/api/pages/9/publish", ""},
		{http.MethodPost, "/api/pages/9/blocks", `{"type":"text","content":{"heading":"Hi"}}`},
		{http.MethodPut, "/api/pages/9/blocks/3", `{"is_active":false}`},
		{http.MethodDelete, "/api/pages/9/blocks/3", ""},
		{http.MethodPut, "/api/pages/9/blocks/reorder", `{"ids":[3]}`},
		{http.MethodGet, "/api/pages/9/stats", ""},
	} {
		w := doJSON(r, rq.method, rq.path, rq.body)
		assert.Equal(t, http.StatusNotFound, w.Code, rq.method+" "+rq.path)
		assert.JSONEq(t, `{"error":"page not found"}`, w.Body.String(), rq.method+" "+rq.path)
	}

	w := doJSON(r, http.MethodGet, "/api/pages/abc", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"invalid page ID"}`, w.Body.String())
	w = doJSON(r, http.MethodDelete, "/api/pages/9/blocks/x", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"invalid block ID"}`, w.Body.String())

	pages.EXPECT().Get(gomock.Any(), uint64(7), uint64(2)).Return(pageView(2, "jose"), nil)
	w = doJSON(r, http.MethodGet, "/api/pages/2", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":2`)
}

func TestPageHandler_UpdateAndArchive(t *testing.T) {
	r, pages, _ := newPageRouter(t)
	title := "New"
	pages.EXPECT().Update(gomock.Any(), uint64(1), uint64(7), uint64(2),
		service.UpdatePageInput{Title: &title, Design: map[string]any{"theme": "bold"}}).Return(pageView(2, "jose"), nil)
	w := doJSON(r, http.MethodPut, "/api/pages/2", `{"title":"New","design":{"theme":"bold"}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	for _, body := range []string{`{}`, `{"archived":null}`, `{"title":"x"}`} {
		w = doJSON(r, http.MethodPatch, "/api/pages/2", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.JSONEq(t, `{"error":"archived is required"}`, w.Body.String(), body)
	}
	w = doJSON(r, http.MethodPatch, "/api/pages/2", `{"archived":"yes"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	pages.EXPECT().SetArchived(gomock.Any(), uint64(1), uint64(7), uint64(2), false).Return(pageView(2, "jose"), nil)
	w = doJSON(r, http.MethodPatch, "/api/pages/2", `{"archived":false}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	pages.EXPECT().Delete(gomock.Any(), uint64(1), uint64(7), uint64(2)).Return(nil)
	w = doJSON(r, http.MethodDelete, "/api/pages/2", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestPageHandler_Blocks_CRUD(t *testing.T) {
	r, pages, _ := newPageRouter(t)
	linkID := uint64(9)
	block := &service.BlockView{ID: 5, Type: "link", Content: map[string]any{"title": "Menu"}, IsActive: true,
		LinkID: &linkID, Link: &service.BlockLink{ID: 9, Title: "Menu", ShortURL: "http://localhost:8080/abc", OriginalURL: "https://e.test"}}

	pages.EXPECT().AddBlock(gomock.Any(), uint64(1), uint64(7), uint64(2), service.BlockInput{
		Type: "link", Content: map[string]any{"title": "Menu"},
		NewLink: &service.NewLinkInput{OriginalURL: "https://e.test"}}).Return(block, nil)
	w := doJSON(r, http.MethodPost, "/api/pages/2/blocks", `{"type":"link","content":{"title":"Menu"},"new_link":{"original_url":"https://e.test"}}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"link_id":9`)
	assert.Contains(t, w.Body.String(), `"short_url":"http://localhost:8080/abc"`)

	// Link service errors of new_link are passed through.
	pages.EXPECT().AddBlock(gomock.Any(), uint64(1), uint64(7), uint64(2), gomock.Any()).Return(nil, service.ErrInvalidURL)
	w = doJSON(r, http.MethodPost, "/api/pages/2/blocks", `{"type":"link","content":{"title":"Menu"},"new_link":{"original_url":"ftp://x"}}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	pages.EXPECT().AddBlock(gomock.Any(), uint64(1), uint64(7), uint64(2), gomock.Any()).
		Return(nil, &service.ValidationError{Msg: "link_id must be a link of this group"})
	w = doJSON(r, http.MethodPost, "/api/pages/2/blocks", `{"type":"link","content":{"title":"Menu"},"link_id":77}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"link_id must be a link of this group"}`, w.Body.String())

	w = doJSON(r, http.MethodPost, "/api/pages/2/blocks", `{"type":`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	off := false
	pages.EXPECT().UpdateBlock(gomock.Any(), uint64(1), uint64(7), uint64(2), uint64(5), service.BlockPatch{IsActive: &off}).Return(block, nil)
	w = doJSON(r, http.MethodPut, "/api/pages/2/blocks/5", `{"is_active":false}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	pages.EXPECT().UpdateBlock(gomock.Any(), uint64(1), uint64(7), uint64(2), uint64(6), gomock.Any()).Return(nil, repository.ErrBlockNotFound)
	w = doJSON(r, http.MethodPut, "/api/pages/2/blocks/6", `{"is_pinned":true}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"block not found"}`, w.Body.String())

	pages.EXPECT().DeleteBlock(gomock.Any(), uint64(1), uint64(7), uint64(2), uint64(5)).Return(nil)
	w = doJSON(r, http.MethodDelete, "/api/pages/2/blocks/5", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())

	pages.EXPECT().DeleteBlock(gomock.Any(), uint64(1), uint64(7), uint64(2), uint64(6)).Return(repository.ErrBlockNotFound)
	w = doJSON(r, http.MethodDelete, "/api/pages/2/blocks/6", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPageHandler_Reorder400(t *testing.T) {
	r, pages, _ := newPageRouter(t)
	pages.EXPECT().Reorder(gomock.Any(), uint64(1), uint64(7), uint64(2), []uint64{5, 4}).
		Return(nil, &service.ValidationError{Msg: "ids must list every block of the page exactly once"})
	w := doJSON(r, http.MethodPut, "/api/pages/2/blocks/reorder", `{"ids":[5,4]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"ids must list every block of the page exactly once"}`, w.Body.String())

	w = doJSON(r, http.MethodPut, "/api/pages/2/blocks/reorder", `{"ids":"x"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	pages.EXPECT().Reorder(gomock.Any(), uint64(1), uint64(7), uint64(2), []uint64{4, 5}).
		Return([]service.BlockView{{ID: 4, Type: "text", Position: 0}, {ID: 5, Type: "text", Position: 1}}, nil)
	w = doJSON(r, http.MethodPut, "/api/pages/2/blocks/reorder", `{"ids":[4,5]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"blocks":[{"id":4`)

	pages.EXPECT().Reorder(gomock.Any(), uint64(1), uint64(7), uint64(3), gomock.Any()).Return(nil, nil)
	w = doJSON(r, http.MethodPut, "/api/pages/3/blocks/reorder", `{"ids":[]}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"blocks":[]}`, w.Body.String())
}

func TestPageHandler_Publish(t *testing.T) {
	r, pages, _ := newPageRouter(t)
	v := pageView(2, "jose")
	pub := time.Date(2026, 10, 25, 14, 0, 0, 0, time.UTC)
	v.Status, v.PublishedAt, v.HasUnpublishedChanges = "published", &pub, false
	pages.EXPECT().Publish(gomock.Any(), uint64(1), uint64(7), uint64(2)).Return(v, nil)
	w := doJSON(r, http.MethodPost, "/api/pages/2/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"status":"published"`)
	assert.Contains(t, w.Body.String(), `"published_at":"2026-10-25T14:00:00Z"`)
	assert.Contains(t, w.Body.String(), `"has_unpublished_changes":false`)
}

func TestPageHandler_Stats_BadQuery400(t *testing.T) {
	r, _, public := newPageRouter(t)
	for _, q := range []string{"from=2026-13-01", "compare=yes", "from=2026-10-24&to=2026-10-18"} {
		w := doJSON(r, http.MethodGet, "/api/pages/2/stats?"+q, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, q)
	}

	public.EXPECT().Stats(gomock.Any(), uint64(7), uint64(2), gomock.Any()).Return(nil, &service.StatsQueryError{Msg: "range too long"})
	w := doJSON(r, http.MethodGet, "/api/pages/2/stats", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"range too long"}`, w.Body.String())

	wantQ := service.StatsQuery{From: d(2026, 10, 18), To: d(2026, 10, 24), Unit: "day", Compare: true}
	public.EXPECT().Stats(gomock.Any(), uint64(7), uint64(2), wantQ).
		Return(&service.PageStats{From: "2026-10-18", To: "2026-10-24", Unit: "day", TotalViews: 3}, nil)
	w = doJSON(r, http.MethodGet, "/api/pages/2/stats?from=2026-10-18&to=2026-10-24&compare=previous&source=qr", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"total_views":3`)
}
