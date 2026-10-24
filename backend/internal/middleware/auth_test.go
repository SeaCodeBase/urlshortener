package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func init() { gin.SetMode(gin.TestMode) }

// newAuthRouter mounts AuthMiddleware and a handler that echoes the user ID.
func newAuthRouter(authSvc service.AuthService, tokenSvc service.TokenService) *gin.Engine {
	r := gin.New()
	r.GET("/x", middleware.AuthMiddleware(authSvc, tokenSvc), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": middleware.GetUserID(c)})
	})
	return r
}

func doAuth(r *gin.Engine, header string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthMiddleware_APIToken_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	authSvc := mocks.NewMockAuthService(ctrl)
	tokenSvc := mocks.NewMockTokenService(ctrl)
	tokenSvc.EXPECT().Validate(gomock.Any(), "us_abc").Return(uint64(7), nil)

	w := doAuth(newAuthRouter(authSvc, tokenSvc), "Bearer us_abc")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"user_id":7}`, w.Body.String())
}

func TestAuthMiddleware_APIToken_Revoked401(t *testing.T) {
	ctrl := gomock.NewController(t)
	authSvc := mocks.NewMockAuthService(ctrl)
	tokenSvc := mocks.NewMockTokenService(ctrl)
	tokenSvc.EXPECT().Validate(gomock.Any(), "us_revoked").Return(uint64(0), service.ErrInvalidAPIToken)

	w := doAuth(newAuthRouter(authSvc, tokenSvc), "Bearer us_revoked")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"error":"invalid or expired token"}`, w.Body.String())
}

func TestAuthMiddleware_JWT_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	authSvc := mocks.NewMockAuthService(ctrl)
	tokenSvc := mocks.NewMockTokenService(ctrl)
	authSvc.EXPECT().ValidateToken("jwt.token.here").Return(uint64(3), nil)

	w := doAuth(newAuthRouter(authSvc, tokenSvc), "Bearer jwt.token.here")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"user_id":3}`, w.Body.String())
}

func TestAuthMiddleware_JWT_Invalid401(t *testing.T) {
	ctrl := gomock.NewController(t)
	authSvc := mocks.NewMockAuthService(ctrl)
	tokenSvc := mocks.NewMockTokenService(ctrl)
	authSvc.EXPECT().ValidateToken("bad").Return(uint64(0), errors.New("expired"))

	w := doAuth(newAuthRouter(authSvc, tokenSvc), "Bearer bad")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_MissingHeader401(t *testing.T) {
	ctrl := gomock.NewController(t)
	w := doAuth(newAuthRouter(mocks.NewMockAuthService(ctrl), mocks.NewMockTokenService(ctrl)), "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
