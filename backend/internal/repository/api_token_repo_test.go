package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
)

func TestAPITokenRepository_Lifecycle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	repo := repository.NewAPITokenRepository(db)
	user := createTestUser(t, db, "tok@example.com")
	other := createTestUser(t, db, "tok-other@example.com")

	tok := &model.APIToken{UserID: user.ID, Name: "cli", TokenHash: "1111111111111111111111111111111111111111111111111111111111111111", TokenPrefix: "abcdefgh"}
	if err := repo.Create(ctx, tok); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tok.ID == 0 || tok.CreatedAt.IsZero() {
		t.Fatalf("expected ID and created_at populated, got %+v", tok)
	}

	got, err := repo.GetByHash(ctx, tok.TokenHash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got.ID != tok.ID || got.UserID != user.ID || got.TokenPrefix != "abcdefgh" || got.LastUsedAt.Valid || got.RevokedAt.Valid {
		t.Fatalf("unexpected token: %+v", got)
	}
	if _, err := repo.GetByHash(ctx, "nope"); !errors.Is(err, repository.ErrAPITokenNotFound) {
		t.Fatalf("expected ErrAPITokenNotFound, got %v", err)
	}

	at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	if err := repo.UpdateLastUsedAt(ctx, tok.ID, at); err != nil {
		t.Fatalf("UpdateLastUsedAt: %v", err)
	}
	got, _ = repo.GetByHash(ctx, tok.TokenHash)
	if !got.LastUsedAt.Valid || !got.LastUsedAt.Time.Equal(at) {
		t.Fatalf("last_used_at not stored: %+v", got.LastUsedAt)
	}

	list, err := repo.ListActiveByUserID(ctx, user.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListActiveByUserID: %v %v", list, err)
	}

	// Another user cannot revoke it.
	if err := repo.Revoke(ctx, other.ID, tok.ID); !errors.Is(err, repository.ErrAPITokenNotFound) {
		t.Fatalf("expected ErrAPITokenNotFound for foreign revoke, got %v", err)
	}
	if err := repo.Revoke(ctx, user.ID, tok.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	// Revoking twice is a not-found.
	if err := repo.Revoke(ctx, user.ID, tok.ID); !errors.Is(err, repository.ErrAPITokenNotFound) {
		t.Fatalf("expected ErrAPITokenNotFound on second revoke, got %v", err)
	}
	got, _ = repo.GetByHash(ctx, tok.TokenHash)
	if !got.RevokedAt.Valid {
		t.Fatal("expected revoked_at set")
	}
	list, _ = repo.ListActiveByUserID(ctx, user.ID)
	if len(list) != 0 {
		t.Fatalf("expected revoked token hidden from list, got %v", list)
	}
}
