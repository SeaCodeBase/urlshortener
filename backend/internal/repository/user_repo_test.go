package repository_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
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

func TestUserRepository_UpdatePassword(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	// Create user first
	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: "old_hash",
	}
	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update password
	err = repo.UpdatePassword(ctx, user.ID, "new_hash")
	if err != nil {
		t.Fatalf("UpdatePassword failed: %v", err)
	}

	// Verify password was updated
	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if found.PasswordHash != "new_hash" {
		t.Errorf("Expected password hash 'new_hash', got '%s'", found.PasswordHash)
	}
}

func TestUserRepository_UpdatePassword_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	err := repo.UpdatePassword(ctx, 99999, "new_hash")
	if err != repository.ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_UpdateDefaultGroup(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := repository.NewUserRepository(db)
	user := createTestUser(t, db, "def@example.com")
	group := &model.Group{UserID: user.ID, Name: "def"}
	if err := repository.NewGroupRepository(db).Create(ctx, group); err != nil {
		t.Fatalf("create group: %v", err)
	}

	if err := repo.UpdateDefaultGroup(ctx, user.ID, group.ID); err != nil {
		t.Fatalf("UpdateDefaultGroup failed: %v", err)
	}
	got, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.DefaultGroupID == nil || *got.DefaultGroupID != group.ID {
		t.Errorf("expected default_group_id %d, got %v", group.ID, got.DefaultGroupID)
	}
	byEmail, err := repo.GetByEmail(ctx, "def@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	if byEmail.DefaultGroupID == nil || *byEmail.DefaultGroupID != group.ID {
		t.Errorf("GetByEmail: expected default_group_id %d, got %v", group.ID, byEmail.DefaultGroupID)
	}

	if err := repo.UpdateDefaultGroup(ctx, 999999, group.ID); err != repository.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_UpdateAccount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := repository.NewUserRepository(db)
	user := createTestUser(t, db, "acct@example.com")

	name := "Acme"
	dismissed := true
	if err := repo.UpdateAccount(ctx, user.ID, &name, &dismissed); err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}
	got, _ := repo.GetByID(ctx, user.ID)
	if got.AccountName == nil || *got.AccountName != "Acme" || !got.HomeChecklistDismissed {
		t.Errorf("unexpected account fields: %+v", got)
	}

	// nil leaves a field unchanged
	if err := repo.UpdateAccount(ctx, user.ID, nil, nil); err != nil {
		t.Fatalf("UpdateAccount (no-op) failed: %v", err)
	}
	other := "Other"
	if err := repo.UpdateAccount(ctx, user.ID, &other, nil); err != nil {
		t.Fatalf("UpdateAccount (name only) failed: %v", err)
	}
	got, _ = repo.GetByID(ctx, user.ID)
	if got.AccountName == nil || *got.AccountName != "Other" || !got.HomeChecklistDismissed {
		t.Errorf("unexpected account fields after partial update: %+v", got)
	}

	if err := repo.UpdateAccount(ctx, 999999, &name, nil); err != repository.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_SetDefaultGroupIfUnset(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := repository.NewUserRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	user := createTestUser(t, db, "race@example.com")
	g1 := &model.Group{UserID: user.ID, Name: "first"}
	g2 := &model.Group{UserID: user.ID, Name: "second"}
	if err := groupRepo.Create(ctx, g1); err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := groupRepo.Create(ctx, g2); err != nil {
		t.Fatalf("create group: %v", err)
	}

	ok, err := repo.SetDefaultGroupIfUnset(ctx, user.ID, g1.ID, nil)
	if err != nil || !ok {
		t.Fatalf("first SetDefaultGroupIfUnset: ok=%v err=%v", ok, err)
	}
	// A second caller that also observed NULL loses.
	ok, err = repo.SetDefaultGroupIfUnset(ctx, user.ID, g2.ID, nil)
	if err != nil || ok {
		t.Fatalf("stale SetDefaultGroupIfUnset: expected ok=false, got ok=%v err=%v", ok, err)
	}
	got, _ := repo.GetByID(ctx, user.ID)
	if got.DefaultGroupID == nil || *got.DefaultGroupID != g1.ID {
		t.Fatalf("expected default_group_id %d, got %v", g1.ID, got.DefaultGroupID)
	}

	// A caller that observed the current value wins (non-NULL compare).
	ok, err = repo.SetDefaultGroupIfUnset(ctx, user.ID, g2.ID, &g1.ID)
	if err != nil || !ok {
		t.Fatalf("SetDefaultGroupIfUnset with current value: ok=%v err=%v", ok, err)
	}
	got, _ = repo.GetByID(ctx, user.ID)
	if got.DefaultGroupID == nil || *got.DefaultGroupID != g2.ID {
		t.Errorf("expected default_group_id %d, got %v", g2.ID, got.DefaultGroupID)
	}
}

func TestUserRepository_UpdateEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "old@example.com")
	other := createTestUser(t, db, "taken@example.com")

	if err := repo.UpdateEmail(ctx, user.ID, "new@example.com"); err != nil {
		t.Fatalf("UpdateEmail failed: %v", err)
	}
	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if found.Email != "new@example.com" {
		t.Errorf("expected email new@example.com, got %s", found.Email)
	}

	// Same value again is not an error (0 rows changed but the user exists).
	if err := repo.UpdateEmail(ctx, user.ID, "new@example.com"); err != nil {
		t.Errorf("UpdateEmail with unchanged value: %v", err)
	}

	if err := repo.UpdateEmail(ctx, user.ID, other.Email); err != repository.ErrEmailExists {
		t.Errorf("expected ErrEmailExists, got %v", err)
	}
	if err := repo.UpdateEmail(ctx, 999999, "ghost@example.com"); err != repository.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}
