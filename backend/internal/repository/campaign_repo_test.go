package repository_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createChannel(t *testing.T, repo *repository.CampaignRepositoryImpl, groupID uint64, name string) *model.Channel {
	t.Helper()
	ch := &model.Channel{GroupID: groupID, Name: name}
	require.NoError(t, repo.CreateChannel(context.Background(), ch))
	require.NotZero(t, ch.ID)
	return ch
}

func createCampaign(t *testing.T, repo *repository.CampaignRepositoryImpl, group *model.Group, name string) *model.Campaign {
	t.Helper()
	c := &model.Campaign{UserID: group.UserID, GroupID: group.ID, Name: name, Description: "d " + name}
	require.NoError(t, repo.Create(context.Background(), c))
	require.NotZero(t, c.ID)
	return c
}

func TestCampaignRepo_CRUDAndSummary(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := linkFixture(t, db, "camp.crud@example.com",
		&model.Link{ShortCode: "camp1"}, &model.Link{ShortCode: "camp2"})
	repo := repository.NewCampaignRepository(db)

	email := createChannel(t, repo, group.ID, "Email")
	social := createChannel(t, repo, group.ID, "Social")
	assert.False(t, email.CreatedAt.IsZero())

	older := createCampaign(t, repo, group, "Older")
	c := createCampaign(t, repo, group, "Spring")
	assert.False(t, c.CreatedAt.IsZero())
	assert.False(t, c.UpdatedAt.IsZero())

	got, err := repo.GetByID(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "Spring", got.Name)
	assert.Equal(t, "d Spring", got.Description)
	assert.Equal(t, group.ID, got.GroupID)
	assert.Equal(t, group.UserID, got.UserID)
	assert.False(t, got.AutoUTM)
	_, err = repo.GetByID(ctx, c.ID+999)
	assert.ErrorIs(t, err, repository.ErrCampaignNotFound)

	got.Name, got.Description, got.AutoUTM = "Spring 2", "new", true
	require.NoError(t, repo.Update(ctx, got))
	got, err = repo.GetByID(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "Spring 2", got.Name)
	assert.Equal(t, "new", got.Description)
	assert.True(t, got.AutoUTM)
	require.NoError(t, repo.Update(ctx, got), "no-op update is not an error")
	missing := *got
	missing.ID = c.ID + 999
	assert.ErrorIs(t, repo.Update(ctx, &missing), repository.ErrCampaignNotFound)

	ch, err := repo.GetChannel(ctx, email.ID)
	require.NoError(t, err)
	assert.Equal(t, "Email", ch.Name)
	_, err = repo.GetChannel(ctx, social.ID+999)
	assert.ErrorIs(t, err, repository.ErrChannelNotFound)
	chans, err := repo.ListChannels(ctx, group.ID)
	require.NoError(t, err)
	assert.Len(t, chans, 2)

	require.NoError(t, repo.SetCampaignChannels(ctx, c.ID, []uint64{email.ID, social.ID}))
	cc, err := repo.CampaignChannels(ctx, c.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []uint64{email.ID, social.ID}, []uint64{cc[0].ID, cc[1].ID})

	require.NoError(t, repo.AddLinks(ctx, c.ID, email.ID, []uint64{links[0].ID, links[1].ID}))
	require.NoError(t, repo.AddLinks(ctx, c.ID, email.ID, []uint64{links[0].ID}), "INSERT IGNORE on duplicates")
	require.NoError(t, repo.AddLinks(ctx, c.ID, email.ID, nil))

	sums, err := repo.ListByGroup(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, sums, 2)
	assert.Equal(t, c.ID, sums[0].ID, "newest first")
	assert.Equal(t, older.ID, sums[1].ID)
	assert.Equal(t, 2, sums[0].ChannelCount)
	assert.Equal(t, 2, sums[0].LinkCount)
	assert.Equal(t, "Spring 2", sums[0].Name)
	assert.Equal(t, 0, sums[1].ChannelCount)
	assert.Equal(t, 0, sums[1].LinkCount)

	members, err := repo.Links(ctx, c.ID)
	require.NoError(t, err)
	assert.Len(t, members, 2)
	for _, m := range members {
		assert.Equal(t, email.ID, m.ChannelID)
		assert.False(t, m.CreatedAt.IsZero())
	}
	lc, err := repo.LinkCampaigns(ctx, links[0].ID)
	require.NoError(t, err)
	require.Len(t, lc, 1)
	assert.Equal(t, c.ID, lc[0].CampaignID)

	require.NoError(t, repo.RemoveLink(ctx, c.ID, email.ID, links[1].ID))
	members, err = repo.Links(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.NoError(t, repo.AddLinks(ctx, c.ID, social.ID, []uint64{links[1].ID}))

	// Replacing the channel set drops the removed channel's memberships.
	require.NoError(t, repo.SetCampaignChannels(ctx, c.ID, []uint64{social.ID}))
	cc, err = repo.CampaignChannels(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, cc, 1)
	assert.Equal(t, social.ID, cc[0].ID)
	members, err = repo.Links(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, social.ID, members[0].ChannelID)
	assert.Equal(t, links[1].ID, members[0].LinkID)
	// Channels themselves are kept (shared by the group's campaigns).
	chans, err = repo.ListChannels(ctx, group.ID)
	require.NoError(t, err)
	assert.Len(t, chans, 2)

	require.NoError(t, repo.SetCampaignChannels(ctx, c.ID, nil))
	cc, err = repo.CampaignChannels(ctx, c.ID)
	require.NoError(t, err)
	assert.Empty(t, cc)

	require.NoError(t, repo.Delete(ctx, older.ID))
	assert.ErrorIs(t, repo.Delete(ctx, older.ID), repository.ErrCampaignNotFound)
}

func TestCampaignRepo_Cascades(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := linkFixture(t, db, "camp.cascade@example.com",
		&model.Link{ShortCode: "casc1"}, &model.Link{ShortCode: "casc2"})
	repo := repository.NewCampaignRepository(db)
	linkRepo := repository.NewLinkRepository(db)
	ch := createChannel(t, repo, group.ID, "Email")
	c := createCampaign(t, repo, group, "C")
	require.NoError(t, repo.SetCampaignChannels(ctx, c.ID, []uint64{ch.ID}))
	require.NoError(t, repo.AddLinks(ctx, c.ID, ch.ID, []uint64{links[0].ID, links[1].ID}))

	require.NoError(t, linkRepo.Delete(ctx, links[0].ID))
	members, err := repo.Links(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, links[1].ID, members[0].LinkID)

	require.NoError(t, repo.Delete(ctx, c.ID))
	lc, err := repo.LinkCampaigns(ctx, links[1].ID)
	require.NoError(t, err)
	assert.Empty(t, lc)
	var n int
	require.NoError(t, db.Get(&n, `SELECT COUNT(*) FROM campaign_channels WHERE campaign_id = ?`, c.ID))
	assert.Zero(t, n)
	_, err = linkRepo.GetByID(ctx, links[1].ID)
	assert.NoError(t, err, "links remain after campaign delete")
	_, err = repo.GetChannel(ctx, ch.ID)
	assert.NoError(t, err, "channels remain after campaign delete")
}

func TestCampaignRepo_ChannelUniquePerGroup(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := linkFixture(t, db, "camp.uniq@example.com")
	other, _ := linkFixture(t, db, "camp.uniq2@example.com")
	repo := repository.NewCampaignRepository(db)

	createChannel(t, repo, group.ID, "Email")
	err := repo.CreateChannel(ctx, &model.Channel{GroupID: group.ID, Name: "Email"})
	assert.ErrorIs(t, err, repository.ErrChannelExists)
	createChannel(t, repo, other.ID, "Email")
}

func TestLinkRepo_FilterByCampaign(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := linkFixture(t, db, "camp.filter@example.com",
		&model.Link{ShortCode: "cf1"}, &model.Link{ShortCode: "cf2"}, &model.Link{ShortCode: "cf3"})
	repo := repository.NewCampaignRepository(db)
	linkRepo := repository.NewLinkRepository(db)
	a := createChannel(t, repo, group.ID, "A")
	b := createChannel(t, repo, group.ID, "B")
	c := createCampaign(t, repo, group, "C")
	other := createCampaign(t, repo, group, "Other")
	require.NoError(t, repo.SetCampaignChannels(ctx, c.ID, []uint64{a.ID, b.ID}))
	require.NoError(t, repo.SetCampaignChannels(ctx, other.ID, []uint64{a.ID}))
	// cf1 in two channels of the same campaign: listed once.
	require.NoError(t, repo.AddLinks(ctx, c.ID, a.ID, []uint64{links[0].ID, links[1].ID}))
	require.NoError(t, repo.AddLinks(ctx, c.ID, b.ID, []uint64{links[0].ID}))
	require.NoError(t, repo.AddLinks(ctx, other.ID, a.ID, []uint64{links[2].ID}))

	rows, total, err := linkRepo.ListByGroup(ctx, group.ID, repository.LinkFilter{CampaignID: &c.ID}, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.ElementsMatch(t, []string{"cf1", "cf2"}, codesOf(rows))

	rows, total, err = linkRepo.ListByGroup(ctx, group.ID, repository.LinkFilter{}, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, rows, 3)
}
