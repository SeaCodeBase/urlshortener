package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/jmoiron/sqlx"
)

func createTestUser(t *testing.T, db *sqlx.DB, email string) *model.User {
	t.Helper()
	user := &model.User{Email: email, PasswordHash: "hashed_password"}
	if err := repository.NewUserRepository(db).Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func TestGroupRepository_CreateAndList(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := repository.NewGroupRepository(db)
	user := createTestUser(t, db, "owner@example.com")
	other := createTestUser(t, db, "other@example.com")

	g1 := &model.Group{UserID: user.ID, Name: "owner"}
	if err := repo.Create(ctx, g1); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if g1.ID == 0 {
		t.Fatal("expected group ID to be set")
	}
	if g1.CreatedAt.IsZero() || g1.UpdatedAt.IsZero() {
		t.Error("expected timestamps to be populated")
	}
	g2 := &model.Group{UserID: user.ID, Name: "second"}
	if err := repo.Create(ctx, g2); err != nil {
		t.Fatalf("Create second failed: %v", err)
	}
	if err := repo.Create(ctx, &model.Group{UserID: other.ID, Name: "other"}); err != nil {
		t.Fatalf("Create other failed: %v", err)
	}

	groups, err := repo.ListByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUserID failed: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].ID != g1.ID || groups[1].ID != g2.ID {
		t.Errorf("expected groups ordered by id, got %d, %d", groups[0].ID, groups[1].ID)
	}

	n, err := repo.CountByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("CountByUserID failed: %v", err)
	}
	if n != 2 {
		t.Errorf("expected count 2, got %d", n)
	}

	got, err := repo.GetByID(ctx, g1.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != "owner" || got.UserID != user.ID || got.DefaultDomainID != nil {
		t.Errorf("unexpected group: %+v", got)
	}
}

func TestGroupRepository_GetByID_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	_, err := repository.NewGroupRepository(db).GetByID(context.Background(), 999999)
	if !errors.Is(err, repository.ErrGroupNotFound) {
		t.Errorf("expected ErrGroupNotFound, got %v", err)
	}
}

func TestGroupRepository_Update(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := repository.NewGroupRepository(db)
	user := createTestUser(t, db, "owner@example.com")

	res, err := db.ExecContext(ctx,
		"INSERT INTO domains (user_id, domain) VALUES (?, 'go.example.com')", user.ID)
	if err != nil {
		t.Fatalf("insert domain: %v", err)
	}
	id, _ := res.LastInsertId()
	domainID := uint64(id)

	g := &model.Group{UserID: user.ID, Name: "before"}
	if err := repo.Create(ctx, g); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	g.Name = "after"
	g.DefaultDomainID = &domainID
	if err := repo.Update(ctx, g); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	got, err := repo.GetByID(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != "after" || got.DefaultDomainID == nil || *got.DefaultDomainID != domainID {
		t.Errorf("unexpected group after update: %+v", got)
	}

	g.DefaultDomainID = nil
	if err := repo.Update(ctx, g); err != nil {
		t.Fatalf("Update (clear domain) failed: %v", err)
	}
	got, _ = repo.GetByID(ctx, g.ID)
	if got.DefaultDomainID != nil {
		t.Errorf("expected default domain cleared, got %v", *got.DefaultDomainID)
	}

	missing := &model.Group{ID: 999999, Name: "x"}
	if err := repo.Update(ctx, missing); !errors.Is(err, repository.ErrGroupNotFound) {
		t.Errorf("expected ErrGroupNotFound, got %v", err)
	}
}
