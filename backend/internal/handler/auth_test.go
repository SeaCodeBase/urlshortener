package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// MockAuthService implements service.AuthService for testing
type MockAuthService struct {
	RegisterFunc func(ctx context.Context, input service.RegisterInput) (*service.AuthResponse, error)
}

func (m *MockAuthService) Register(ctx context.Context, input service.RegisterInput) (*service.AuthResponse, error) {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, input)
	}
	return &service.AuthResponse{Token: "test-token"}, nil
}

func (m *MockAuthService) Login(ctx context.Context, input service.LoginInput) (*service.AuthResponse, error) {
	return nil, nil
}

func (m *MockAuthService) GetUserByID(ctx context.Context, userID uint64) (*model.User, error) {
	return nil, nil
}

func (m *MockAuthService) ValidateToken(tokenString string) (uint64, error) {
	return 0, nil
}

func (m *MockAuthService) ChangePassword(ctx context.Context, userID uint64, input service.ChangePasswordInput) error {
	return nil
}

func (m *MockAuthService) UpdateDisplayName(ctx context.Context, userID uint64, displayName string) error {
	return nil
}

func (m *MockAuthService) GenerateToken(userID uint64) (string, error) {
	return "", nil
}

func (m *MockAuthService) UpdateEmail(ctx context.Context, userID uint64, password, newEmail string) error {
	return nil
}

func (m *MockAuthService) UpdateAccount(ctx context.Context, userID uint64, input service.UpdateAccountInput) error {
	return nil
}

// MockPasskeyService implements service.PasskeyService for testing
type MockPasskeyService struct{}

func (m *MockPasskeyService) BeginRegistration(ctx context.Context, userID uint64) (*protocol.CredentialCreation, string, error) {
	return nil, "", nil
}

func (m *MockPasskeyService) FinishRegistration(ctx context.Context, userID uint64, sessionData string, credentialJSON []byte, name string) (*model.Passkey, error) {
	return nil, nil
}

func (m *MockPasskeyService) BeginLogin(ctx context.Context, userID uint64) (*protocol.CredentialAssertion, string, error) {
	return nil, "", nil
}

func (m *MockPasskeyService) FinishLogin(ctx context.Context, userID uint64, sessionData string, credentialJSON []byte) error {
	return nil
}

func (m *MockPasskeyService) List(ctx context.Context, userID uint64) ([]model.Passkey, error) {
	return nil, nil
}

func (m *MockPasskeyService) Rename(ctx context.Context, userID uint64, passkeyID uint64, name string) error {
	return nil
}

func (m *MockPasskeyService) Delete(ctx context.Context, userID uint64, passkeyID uint64) error {
	return nil
}

func (m *MockPasskeyService) HasPasskeys(ctx context.Context, userID uint64) (bool, error) {
	return false, nil
}

func TestRegisterDisabled(t *testing.T) {
	allowReg := false
	cfg := &config.Config{
		Server: config.ServerConfig{
			AllowRegistration: &allowReg,
		},
	}

	mockAuthService := &MockAuthService{}
	mockPasskeyService := &MockPasskeyService{}
	handler := NewAuthHandler(mockAuthService, mockPasskeyService, cfg)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", handler.Register)

	body := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response["error"] != "Registration is disabled" {
		t.Errorf("expected 'Registration is disabled' error, got %s", response["error"])
	}
}

func TestRegisterEnabled(t *testing.T) {
	allowReg := true
	cfg := &config.Config{
		Server: config.ServerConfig{
			AllowRegistration: &allowReg,
		},
	}

	registerCalled := false
	mockAuthService := &MockAuthService{
		RegisterFunc: func(ctx context.Context, input service.RegisterInput) (*service.AuthResponse, error) {
			registerCalled = true
			return &service.AuthResponse{
				Token: "test-token",
				User:  &model.User{ID: 1, Email: input.Email},
			}, nil
		},
	}
	mockPasskeyService := &MockPasskeyService{}
	handler := NewAuthHandler(mockAuthService, mockPasskeyService, cfg)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", handler.Register)

	body := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	if !registerCalled {
		t.Error("expected Register to be called on auth service")
	}
}

func newAccountRouter(t *testing.T) (*gin.Engine, *mocks.MockAuthService) {
	gin.SetMode(gin.TestMode)
	svc := mocks.NewMockAuthService(gomock.NewController(t))
	h := NewAuthHandler(svc, &MockPasskeyService{}, &config.Config{})
	r := gin.New()
	auth := r.Group("/api/auth", func(c *gin.Context) { c.Set("user_id", uint64(1)); c.Next() })
	auth.GET("/me", h.Me)
	auth.PUT("/me", h.UpdateMe)
	auth.PUT("/email", h.UpdateEmail)
	return r, svc
}

func accountUser() *model.User {
	name, acct := "Jose", "Acme"
	created := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	return &model.User{ID: 1, Email: "me@example.com", DisplayName: &name, PasswordHash: "secret-hash",
		DefaultGroupID: u64p(7), AccountName: &acct, HomeChecklistDismissed: true, CreatedAt: created, UpdatedAt: created}
}

const accountUserJSON = `{"id":1,"email":"me@example.com","display_name":"Jose","default_group_id":7,
	"account_name":"Acme","home_checklist_dismissed":true,
	"created_at":"2026-10-24T12:00:00Z","updated_at":"2026-10-24T12:00:00Z"}`

func TestAuthHandler_Me_ReturnsAccountFields(t *testing.T) {
	r, svc := newAccountRouter(t)
	svc.EXPECT().GetUserByID(gomock.Any(), uint64(1)).Return(accountUser(), nil)
	w := doJSON(r, http.MethodGet, "/api/auth/me", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, accountUserJSON, w.Body.String())
}

func TestAuthHandler_UpdateMe_AllFields(t *testing.T) {
	r, svc := newAccountRouter(t)
	name, acct, dismissed := "Jose", "Acme", true
	svc.EXPECT().UpdateAccount(gomock.Any(), uint64(1), service.UpdateAccountInput{
		DisplayName: &name, AccountName: &acct, DefaultGroupID: u64p(7), HomeChecklistDismissed: &dismissed,
	}).Return(nil)
	svc.EXPECT().GetUserByID(gomock.Any(), uint64(1)).Return(accountUser(), nil)

	w := doJSON(r, http.MethodPut, "/api/auth/me",
		`{"display_name":"Jose","account_name":"Acme","default_group_id":7,"home_checklist_dismissed":true}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, accountUserJSON, w.Body.String())
}

func TestAuthHandler_UpdateMe_ForeignDefaultGroup404(t *testing.T) {
	r, svc := newAccountRouter(t)
	svc.EXPECT().UpdateAccount(gomock.Any(), uint64(1), service.UpdateAccountInput{DefaultGroupID: u64p(9)}).
		Return(service.ErrGroupNotFound)
	w := doJSON(r, http.MethodPut, "/api/auth/me", `{"default_group_id":9}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"group not found"}`, w.Body.String())
}

func TestAuthHandler_UpdateMe_BlankAccountName400(t *testing.T) {
	r, svc := newAccountRouter(t)
	blank := " "
	svc.EXPECT().UpdateAccount(gomock.Any(), uint64(1), service.UpdateAccountInput{AccountName: &blank}).
		Return(service.ErrInvalidAccountName)
	w := doJSON(r, http.MethodPut, "/api/auth/me", `{"account_name":" "}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_UpdateEmail_OK(t *testing.T) {
	r, svc := newAccountRouter(t)
	svc.EXPECT().UpdateEmail(gomock.Any(), uint64(1), "password123", "new@example.com").Return(nil)
	svc.EXPECT().GetUserByID(gomock.Any(), uint64(1)).Return(accountUser(), nil)
	w := doJSON(r, http.MethodPut, "/api/auth/email", `{"password":"password123","new_email":"new@example.com"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "secret-hash")
}

func TestAuthHandler_UpdateEmail_WrongPassword401(t *testing.T) {
	r, svc := newAccountRouter(t)
	svc.EXPECT().UpdateEmail(gomock.Any(), uint64(1), "bad", "new@example.com").Return(service.ErrWrongPassword)
	w := doJSON(r, http.MethodPut, "/api/auth/email", `{"password":"bad","new_email":"new@example.com"}`)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"error":"password is incorrect"}`, w.Body.String())
}

func TestAuthHandler_UpdateEmail_Taken409(t *testing.T) {
	r, svc := newAccountRouter(t)
	svc.EXPECT().UpdateEmail(gomock.Any(), uint64(1), "pw", "taken@example.com").Return(service.ErrEmailTaken)
	w := doJSON(r, http.MethodPut, "/api/auth/email", `{"password":"pw","new_email":"taken@example.com"}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"email already in use"}`, w.Body.String())
}

func TestAuthHandler_UpdateEmail_Invalid400(t *testing.T) {
	r, svc := newAccountRouter(t)
	svc.EXPECT().UpdateEmail(gomock.Any(), uint64(1), "pw", "nope").Return(service.ErrInvalidEmail)
	w := doJSON(r, http.MethodPut, "/api/auth/email", `{"password":"pw","new_email":"nope"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Missing fields never reach the service.
	w = doJSON(r, http.MethodPut, "/api/auth/email", `{"new_email":"a@b.com"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_UpdateEmail_SameEmail400(t *testing.T) {
	r, svc := newAccountRouter(t)
	svc.EXPECT().UpdateEmail(gomock.Any(), uint64(1), "pw", "Me@example.com").Return(service.ErrSameEmail)
	w := doJSON(r, http.MethodPut, "/api/auth/email", `{"password":"pw","new_email":"Me@example.com"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"new email is the same as the current email"}`, w.Body.String())
}
