package repository_test

import (
	"context"
	"testing"

	"github.com/jose/urlshortener/internal/model"
	"github.com/jose/urlshortener/internal/repository"
	"github.com/jose/urlshortener/internal/testutil"
)

func TestUserRepository_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if user.ID == 0 {
		t.Error("Expected user ID to be set after creation")
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	// Create user first
	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
	}
	_ = repo.Create(ctx, user)

	// Get by email
	found, err := repo.GetByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}

	if found.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, found.Email)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
	if err != repository.ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	// Create user first
	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
	}
	_ = repo.Create(ctx, user)

	// Get by ID
	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if found.ID != user.ID {
		t.Errorf("Expected ID %d, got %d", user.ID, found.ID)
	}
	if found.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, found.Email)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, 99999)
	if err != repository.ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_EmailExists(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	// Check non-existent email
	exists, err := repo.EmailExists(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("EmailExists failed: %v", err)
	}
	if exists {
		t.Error("Expected email to not exist")
	}

	// Create user
	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
	}
	_ = repo.Create(ctx, user)

	// Check existing email
	exists, err = repo.EmailExists(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("EmailExists failed: %v", err)
	}
	if !exists {
		t.Error("Expected email to exist")
	}
}
