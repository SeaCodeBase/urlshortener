package repository_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
)

func TestLinkRepository_Create_WritesGroupID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	user := createTestUser(t, db, "linker@example.com")
	group := &model.Group{UserID: user.ID, Name: "linker"}
	if err := repository.NewGroupRepository(db).Create(ctx, group); err != nil {
		t.Fatalf("create group: %v", err)
	}

	repo := repository.NewLinkRepository(db)
	link := &model.Link{
		UserID:      user.ID,
		GroupID:     group.ID,
		ShortCode:   "grp1",
		OriginalURL: "https://example.com/grouped",
		IsActive:    true,
	}
	if err := repo.Create(ctx, link); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.GetByID(ctx, link.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.GroupID != group.ID {
		t.Errorf("GetByID: expected group_id %d, got %d", group.ID, got.GroupID)
	}

	byCode, err := repo.GetByDomainAndShortCode(ctx, nil, "grp1")
	if err != nil {
		t.Fatalf("GetByDomainAndShortCode failed: %v", err)
	}
	if byCode.GroupID != group.ID {
		t.Errorf("GetByDomainAndShortCode: expected group_id %d, got %d", group.ID, byCode.GroupID)
	}

	list, err := repo.ListByUserID(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListByUserID failed: %v", err)
	}
	if len(list) != 1 || list[0].GroupID != group.ID {
		t.Errorf("ListByUserID: expected one link with group_id %d, got %+v", group.ID, list)
	}
}

func TestDomainRepository_GetByDomain_ReturnsRedirects(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	user := createTestUser(t, db, "domainer@example.com")
	repo := repository.NewDomainRepository(db)
	d := &model.Domain{UserID: user.ID, Domain: "go.redirects.test"}
	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("create domain: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE domains SET root_redirect = ?, not_found_redirect = ? WHERE id = ?`,
		"https://root.example.com", "https://404.example.com", d.ID); err != nil {
		t.Fatalf("set redirects: %v", err)
	}

	got, err := repo.GetByDomain(ctx, "go.redirects.test")
	if err != nil {
		t.Fatalf("GetByDomain: %v", err)
	}
	if got.RootRedirect == nil || *got.RootRedirect != "https://root.example.com" ||
		got.NotFoundRedirect == nil || *got.NotFoundRedirect != "https://404.example.com" {
		t.Fatalf("expected redirects populated, got %+v", got)
	}
}

func TestLinkRepository_GetByDomainAndShortCode_ReturnsArchived(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	user := createTestUser(t, db, "archiver@example.com")
	group := &model.Group{UserID: user.ID, Name: "archiver"}
	if err := repository.NewGroupRepository(db).Create(ctx, group); err != nil {
		t.Fatalf("create group: %v", err)
	}
	repo := repository.NewLinkRepository(db)
	link := &model.Link{UserID: user.ID, GroupID: group.ID, ShortCode: "arch1", OriginalURL: "https://example.com", IsActive: true}
	if err := repo.Create(ctx, link); err != nil {
		t.Fatalf("create link: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE links SET archived = TRUE WHERE id = ?`, link.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	got, err := repo.GetByDomainAndShortCode(ctx, nil, "arch1")
	if err != nil {
		t.Fatalf("GetByDomainAndShortCode: %v", err)
	}
	if !got.Archived {
		t.Fatal("expected Archived=true")
	}
}
