package service_test

import (
	"context"
	"testing"

	"github.com/jose/urlshortener/internal/repository"
	"github.com/jose/urlshortener/internal/service"
	"github.com/jose/urlshortener/internal/testutil"
)

func TestAuthService_Register(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, "test-secret")

	ctx := context.Background()
	input := service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}

	resp, err := authService.Register(ctx, input)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if resp.Token == "" {
		t.Error("Expected token to be set")
	}

	if resp.User.Email != input.Email {
		t.Errorf("Expected email %s, got %s", input.Email, resp.User.Email)
	}
}

func TestAuthService_Login(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, "test-secret")

	ctx := context.Background()

	// Register first
	registerInput := service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}
	_, _ = authService.Register(ctx, registerInput)

	// Login
	loginInput := service.LoginInput{
		Email:    "test@example.com",
		Password: "password123",
	}

	resp, err := authService.Login(ctx, loginInput)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if resp.Token == "" {
		t.Error("Expected token to be set")
	}
}

func TestAuthService_Login_InvalidPassword(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, "test-secret")

	ctx := context.Background()

	// Register
	registerInput := service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}
	_, _ = authService.Register(ctx, registerInput)

	// Login with wrong password
	loginInput := service.LoginInput{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}

	_, err := authService.Login(ctx, loginInput)
	if err != service.ErrInvalidCredentials {
		t.Errorf("Expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_ValidateToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, "test-secret")

	ctx := context.Background()

	// Register to get a token
	input := service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}
	resp, _ := authService.Register(ctx, input)

	// Validate token
	userID, err := authService.ValidateToken(resp.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if userID != resp.User.ID {
		t.Errorf("Expected user ID %d, got %d", resp.User.ID, userID)
	}
}
