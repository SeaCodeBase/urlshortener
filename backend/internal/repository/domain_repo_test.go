package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainRepository_CreateWritesVerificationFields(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	user := createTestUser(t, db, "dom@example.com")
	group := &model.Group{UserID: user.ID, Name: "dom"}
	require.NoError(t, repository.NewGroupRepository(db).Create(ctx, group))

	repo := repository.NewDomainRepository(db)
	d := &model.Domain{
		UserID:             user.ID,
		GroupID:            &group.ID,
		Domain:             "go.example.com",
		VerificationToken:  "0123456789abcdef0123456789abcdef",
		VerificationStatus: model.DomainVerificationPending,
	}
	require.NoError(t, repo.Create(ctx, d))
	require.NotZero(t, d.ID)

	got, err := repo.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "go.example.com", got.Domain)
	require.NotNil(t, got.GroupID)
	assert.Equal(t, group.ID, *got.GroupID)
	assert.Equal(t, "0123456789abcdef0123456789abcdef", got.VerificationToken)
	assert.Equal(t, model.DomainVerificationPending, got.VerificationStatus)
	assert.False(t, got.VerifiedAt.Valid)

	_, err = repo.ListByUserID(ctx, user.ID)
	require.NoError(t, err)

	dup := &model.Domain{UserID: user.ID, Domain: "go.example.com", VerificationStatus: model.DomainVerificationPending}
	assert.ErrorIs(t, repo.Create(ctx, dup), repository.ErrDomainExists)
}

func TestDomainRepository_Update(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	user := createTestUser(t, db, "domup@example.com")
	repo := repository.NewDomainRepository(db)
	d := &model.Domain{UserID: user.ID, Domain: "up.example.com", VerificationToken: "tok", VerificationStatus: model.DomainVerificationPending}
	require.NoError(t, repo.Create(ctx, d))

	now := time.Now().UTC().Truncate(time.Second)
	msg := "TXT record not found"
	root := "https://example.com/"
	d.VerificationStatus = model.DomainVerificationError
	d.VerificationError = &msg
	d.LastCheckedAt = model.NullTime{NullTime: sql.NullTime{Time: now, Valid: true}}
	d.RootRedirect = &root
	require.NoError(t, repo.Update(ctx, d))

	got, err := repo.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, model.DomainVerificationError, got.VerificationStatus)
	require.NotNil(t, got.VerificationError)
	assert.Equal(t, msg, *got.VerificationError)
	require.True(t, got.LastCheckedAt.Valid)
	assert.True(t, got.LastCheckedAt.Time.Equal(now), "last_checked_at round-trips in UTC: %v vs %v", got.LastCheckedAt.Time, now)
	require.NotNil(t, got.RootRedirect)
	assert.Equal(t, root, *got.RootRedirect)
	assert.Nil(t, got.NotFoundRedirect)
	assert.Nil(t, got.GroupID)

	// Clearing fields writes NULLs.
	d.VerificationStatus = model.DomainVerificationOK
	d.VerificationError = nil
	d.VerifiedAt = model.NullTime{NullTime: sql.NullTime{Time: now, Valid: true}}
	d.RootRedirect = nil
	require.NoError(t, repo.Update(ctx, d))
	got, err = repo.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, model.DomainVerificationOK, got.VerificationStatus)
	assert.Nil(t, got.VerificationError)
	assert.Nil(t, got.RootRedirect)
	assert.True(t, got.VerifiedAt.Valid)

	assert.ErrorIs(t, repo.Update(ctx, &model.Domain{ID: 999999, VerificationStatus: model.DomainVerificationOK}), repository.ErrDomainNotFound)
}
