package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
	repomocks "github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// fakeQRCounter returns count and records the filter it was asked for.
type fakeQRCounter struct {
	count   int64
	groupID uint64
	filter  repository.QRFilter
}

func (f *fakeQRCounter) CountByGroup(_ context.Context, groupID uint64, filter repository.QRFilter) (int64, error) {
	f.groupID, f.filter = groupID, filter
	return f.count, nil
}

func newSearchRouter(t *testing.T) (*gin.Engine, *repomocks.MockLinkRepository, *fakeQRCounter) {
	gin.SetMode(gin.TestMode)
	links := repomocks.NewMockLinkRepository(gomock.NewController(t))
	qrs := &fakeQRCounter{count: 3}
	r := gin.New()
	r.GET("/api/search", withGroup, NewSearchHandler(links, qrs).Search)
	return r, links, qrs
}

func TestSearchHandler_Counts(t *testing.T) {
	r, links, qrs := newSearchRouter(t)
	links.EXPECT().CountByGroup(gomock.Any(), uint64(7), repository.LinkFilter{Query: "promo"}).Return(int64(5), nil)

	w := doJSON(r, http.MethodGet, "/api/search?q=promo", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"links":5,"qr_codes":3}`, w.Body.String())
	assert.Equal(t, uint64(7), qrs.groupID)
	assert.Equal(t, repository.QRFilter{Query: "promo"}, qrs.filter)
}

func TestSearchHandler_EmptyQuery400(t *testing.T) {
	r, _, _ := newSearchRouter(t)
	for _, path := range []string{"/api/search", "/api/search?q=", "/api/search?q=%20%20"} {
		w := doJSON(r, http.MethodGet, path, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, path)
	}
}
