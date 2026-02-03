package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
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
