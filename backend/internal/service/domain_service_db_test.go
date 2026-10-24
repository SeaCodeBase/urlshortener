package service_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainService_DemoteReservedDomains(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	users := repository.NewUserRepository(db)
	user := &model.User{Email: "legacy@example.com", PasswordHash: "x"}
	require.NoError(t, users.Create(ctx, user))

	domains := repository.NewDomainRepository(db)
	mk := func(name, status string) *model.Domain {
		d := &model.Domain{UserID: user.ID, Domain: name, VerificationStatus: status}
		require.NoError(t, domains.Create(ctx, d))
		return d
	}
	// Legacy rows backfilled as ok by migration 002.
	base := mk("sho.rt", model.DomainVerificationOK)
	front := mk("app.example.com", model.DomainVerificationOK)
	other := mk("go.brand.com", model.DomainVerificationOK)

	svc := service.NewDomainService(domains, repository.NewGroupRepository(db), repository.NewLinkRepository(db), &fakeResolver{}, "r.sho.rt", "SHO.RT:8080", "app.example.com")
	n, err := svc.DemoteReservedDomains(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	for _, d := range []*model.Domain{base, front} {
		got, err := domains.GetByID(ctx, d.ID)
		require.NoError(t, err)
		assert.Equal(t, model.DomainVerificationError, got.VerificationStatus, d.Domain)
		require.NotNil(t, got.VerificationError, d.Domain)
		assert.Equal(t, "reserved host", *got.VerificationError)
	}
	got, err := domains.GetByID(ctx, other.ID)
	require.NoError(t, err)
	assert.Equal(t, model.DomainVerificationOK, got.VerificationStatus)

	// Idempotent: already-demoted rows are not counted again.
	n, err = svc.DemoteReservedDomains(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}
