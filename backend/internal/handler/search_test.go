package handler

import (
	"net/http"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
	repomocks "github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newSearchRouter(t *testing.T) (*gin.Engine, *repomocks.MockLinkRepository) {
	gin.SetMode(gin.TestMode)
	links := repomocks.NewMockLinkRepository(gomock.NewController(t))
	r := gin.New()
	r.GET("/api/search", withGroup, NewSearchHandler(links).Search)
	return r, links
}

func TestSearchHandler_Counts(t *testing.T) {
	r, links := newSearchRouter(t)
	links.EXPECT().CountByGroup(gomock.Any(), uint64(7), repository.LinkFilter{Query: "promo"}).Return(int64(5), nil)

	w := doJSON(r, http.MethodGet, "/api/search?q=promo", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"links":5,"qr_codes":0}`, w.Body.String())
}

func TestSearchHandler_EmptyQuery400(t *testing.T) {
	r, _ := newSearchRouter(t)
	for _, path := range []string{"/api/search", "/api/search?q=", "/api/search?q=%20%20"} {
		w := doJSON(r, http.MethodGet, path, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, path)
	}
}
