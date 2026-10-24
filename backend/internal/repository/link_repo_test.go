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
