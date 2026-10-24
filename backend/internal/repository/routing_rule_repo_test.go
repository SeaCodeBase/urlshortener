package repository_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
)

func TestRoutingRuleRepository_ReplaceAndList(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	user := createTestUser(t, db, "router@example.com")
	group := &model.Group{UserID: user.ID, Name: "router"}
	if err := repository.NewGroupRepository(db).Create(ctx, group); err != nil {
		t.Fatalf("create group: %v", err)
	}
	link := &model.Link{UserID: user.ID, GroupID: group.ID, ShortCode: "rt1", OriginalURL: "https://example.com", IsActive: true}
	if err := repository.NewLinkRepository(db).Create(ctx, link); err != nil {
		t.Fatalf("create link: %v", err)
	}

	repo := repository.NewRoutingRuleRepository(db)
	if err := repo.ReplaceForLink(ctx, link.ID, []model.RoutingRule{
		{LongURL: "https://ios.example.com", OSMatch: model.StringList{"ios"}},
		{LongURL: "https://eu.example.com", CountryMatch: model.StringList{"FR", "DE"}, CountryExclude: model.StringList{"US"}, DeviceMatch: model.StringList{"mobile"}},
	}); err != nil {
		t.Fatalf("ReplaceForLink: %v", err)
	}

	rules, err := repo.ListByLinkID(ctx, link.ID)
	if err != nil {
		t.Fatalf("ListByLinkID: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].LongURL != "https://ios.example.com" || rules[0].Position != 0 || len(rules[0].OSMatch) != 1 || rules[0].OSMatch[0] != "ios" {
		t.Errorf("unexpected first rule: %+v", rules[0])
	}
	if rules[1].Position != 1 || len(rules[1].CountryMatch) != 2 || rules[1].CountryExclude[0] != "US" || rules[1].DeviceMatch[0] != "mobile" || len(rules[1].OSMatch) != 0 {
		t.Errorf("unexpected second rule: %+v", rules[1])
	}

	// Replace with a single rule; old ones are gone.
	if err := repo.ReplaceForLink(ctx, link.ID, []model.RoutingRule{{LongURL: "https://only.example.com"}}); err != nil {
		t.Fatalf("ReplaceForLink #2: %v", err)
	}
	rules, _ = repo.ListByLinkID(ctx, link.ID)
	if len(rules) != 1 || rules[0].LongURL != "https://only.example.com" || rules[0].LinkID != link.ID {
		t.Fatalf("unexpected rules after replace: %+v", rules)
	}

	// Empty replace clears.
	if err := repo.ReplaceForLink(ctx, link.ID, nil); err != nil {
		t.Fatalf("ReplaceForLink clear: %v", err)
	}
	rules, _ = repo.ListByLinkID(ctx, link.ID)
	if len(rules) != 0 {
		t.Fatalf("expected no rules, got %+v", rules)
	}
}
