package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	svcmocks "github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type campaignSvcFixture struct {
	svc       *service.CampaignServiceImpl
	campaigns *mocks.MockCampaignRepository
	links     *mocks.MockLinkRepository
	linkSvc   *svcmocks.MockLinkService
	clicks    *mocks.MockClickRepository
	activity  *svcmocks.MockActivityService
	now       time.Time
}

func newCampaignSvc(t *testing.T) *campaignSvcFixture {
	ctrl := gomock.NewController(t)
	f := &campaignSvcFixture{
		campaigns: mocks.NewMockCampaignRepository(ctrl),
		links:     mocks.NewMockLinkRepository(ctrl),
		linkSvc:   svcmocks.NewMockLinkService(ctrl),
		clicks:    mocks.NewMockClickRepository(ctrl),
		activity:  svcmocks.NewMockActivityService(ctrl),
		now:       time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC),
	}
	f.svc = service.NewCampaignService(service.CampaignServiceDeps{
		Campaigns: f.campaigns,
		Links:     f.links,
		LinkSvc:   f.linkSvc,
		Clicks:    f.clicks,
		Activity:  f.activity,
	})
	f.svc.SetClock(func() time.Time { return f.now })
	return f
}

func sampleCampaign(id, groupID uint64, autoUTM bool) *model.Campaign {
	return &model.Campaign{ID: id, UserID: 1, GroupID: groupID, Name: "Spring", AutoUTM: autoUTM}
}

func TestCampaignService_Create_ValidatesAndCreatesChannels(t *testing.T) {
	ctx := context.Background()

	t.Run("blank name", func(t *testing.T) {
		f := newCampaignSvc(t)
		_, err := f.svc.Create(ctx, 1, 7, service.CampaignInput{Name: "   "})
		assertValidation(t, err)
	})
	t.Run("long description", func(t *testing.T) {
		f := newCampaignSvc(t)
		long := make([]rune, 501)
		for i := range long {
			long[i] = 'é'
		}
		_, err := f.svc.Create(ctx, 1, 7, service.CampaignInput{Name: "n", Description: string(long)})
		assertValidation(t, err)
	})
	t.Run("foreign channel id", func(t *testing.T) {
		f := newCampaignSvc(t)
		f.campaigns.EXPECT().ListChannels(gomock.Any(), uint64(7)).Return([]model.Channel{{ID: 3, GroupID: 7, Name: "Social"}}, nil)
		_, err := f.svc.Create(ctx, 1, 7, service.CampaignInput{Name: "n", ChannelIDs: []uint64{99}})
		assertValidation(t, err)
	})
	t.Run("new channels deduped and added", func(t *testing.T) {
		f := newCampaignSvc(t)
		f.campaigns.EXPECT().ListChannels(gomock.Any(), uint64(7)).
			Return([]model.Channel{{ID: 3, GroupID: 7, Name: "Social"}}, nil)
		f.campaigns.EXPECT().CreateChannel(gomock.Any(), gomock.Any()).Times(1).
			DoAndReturn(func(_ context.Context, ch *model.Channel) error {
				assert.Equal(t, "Email", ch.Name)
				assert.Equal(t, uint64(7), ch.GroupID)
				ch.ID = 11
				return nil
			})
		f.campaigns.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c *model.Campaign) error {
			assert.Equal(t, "Spring", c.Name)
			assert.Equal(t, uint64(7), c.GroupID)
			assert.Equal(t, uint64(1), c.UserID)
			c.ID = 5
			return nil
		})
		f.campaigns.EXPECT().SetCampaignChannels(gomock.Any(), uint64(5), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uint64, ids []uint64) error {
				assert.ElementsMatch(t, []uint64{3, 11}, ids)
				return nil
			})
		f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionCampaignCreate, service.EntityCampaign, uint64(5),
			map[string]any{"name": "Spring"})

		d, err := f.svc.Create(ctx, 1, 7, service.CampaignInput{
			Name: "  Spring ", ChannelIDs: []uint64{3, 3}, NewChannels: []string{"Email", " email ", "social"},
		})
		require.NoError(t, err)
		assert.Equal(t, uint64(5), d.ID)
		require.Len(t, d.Channels, 2)
		ids := []uint64{d.Channels[0].ID, d.Channels[1].ID}
		assert.ElementsMatch(t, []uint64{3, 11}, ids)
		for _, ch := range d.Channels {
			assert.NotNil(t, ch.Links)
		}
	})
	t.Run("long channel name", func(t *testing.T) {
		f := newCampaignSvc(t)
		f.campaigns.EXPECT().ListChannels(gomock.Any(), uint64(7)).Return([]model.Channel{}, nil)
		long := make([]byte, 81)
		for i := range long {
			long[i] = 'a'
		}
		_, err := f.svc.Create(ctx, 1, 7, service.CampaignInput{Name: "n", NewChannels: []string{string(long)}})
		assertValidation(t, err)
	})
}

func TestCampaignService_WrongGroup(t *testing.T) {
	ctx := context.Background()
	f := newCampaignSvc(t)
	f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 2, false), nil).AnyTimes()

	_, err := f.svc.Get(ctx, 7, 5)
	assert.ErrorIs(t, err, repository.ErrCampaignNotFound)
	_, err = f.svc.Update(ctx, 1, 7, 5, service.CampaignInput{Name: "x"})
	assert.ErrorIs(t, err, repository.ErrCampaignNotFound)
	err = f.svc.Delete(ctx, 1, 7, 5)
	assert.ErrorIs(t, err, repository.ErrCampaignNotFound)
	err = f.svc.AddLinks(ctx, 1, 7, 5, 3, []uint64{1})
	assert.ErrorIs(t, err, repository.ErrCampaignNotFound)
	err = f.svc.RemoveLink(ctx, 1, 7, 5, 3, 1)
	assert.ErrorIs(t, err, repository.ErrCampaignNotFound)
	_, err = f.svc.CreateLinkInChannel(ctx, 1, 7, 5, 3, service.CreateLinkInChannelInput{OriginalURL: "https://x.com/"})
	assert.ErrorIs(t, err, repository.ErrCampaignNotFound)
	_, err = f.svc.Stats(ctx, 7, 5, service.StatsQuery{})
	assert.ErrorIs(t, err, repository.ErrCampaignNotFound)
}

func TestCampaignService_AddLinks_ForeignLink(t *testing.T) {
	ctx := context.Background()
	channels := []model.Channel{{ID: 3, GroupID: 7, Name: "Email"}}
	setup := func(t *testing.T) *campaignSvcFixture {
		f := newCampaignSvc(t)
		f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, false), nil)
		return f
	}

	t.Run("link of another group", func(t *testing.T) {
		f := setup(t)
		f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return(channels, nil)
		f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{10, 11}).Return([]model.Link{
			{ID: 10, GroupID: 7, Source: model.LinkSourceLink},
			{ID: 11, GroupID: 2, Source: model.LinkSourceLink},
		}, nil)
		assertValidation(t, f.svc.AddLinks(ctx, 1, 7, 5, 3, []uint64{10, 11}))
	})
	t.Run("missing link", func(t *testing.T) {
		f := setup(t)
		f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return(channels, nil)
		f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{10}).Return([]model.Link{}, nil)
		assertValidation(t, f.svc.AddLinks(ctx, 1, 7, 5, 3, []uint64{10}))
	})
	t.Run("qr link", func(t *testing.T) {
		f := setup(t)
		f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return(channels, nil)
		f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{10}).Return([]model.Link{
			{ID: 10, GroupID: 7, Source: model.LinkSourceQR},
		}, nil)
		assertValidation(t, f.svc.AddLinks(ctx, 1, 7, 5, 3, []uint64{10}))
	})
	t.Run("channel not in campaign", func(t *testing.T) {
		f := setup(t)
		f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return(channels, nil)
		assertValidation(t, f.svc.AddLinks(ctx, 1, 7, 5, 99, []uint64{10}))
	})
	t.Run("empty and too many", func(t *testing.T) {
		f := setup(t)
		assertValidation(t, f.svc.AddLinks(ctx, 1, 7, 5, 3, nil))
		f2 := setup(t)
		ids := make([]uint64, 501)
		for i := range ids {
			ids[i] = uint64(i + 1)
		}
		assertValidation(t, f2.svc.AddLinks(ctx, 1, 7, 5, 3, ids))
	})
}

func TestCampaignService_AddLinks_NeverRewritesURL(t *testing.T) {
	ctx := context.Background()
	f := newCampaignSvc(t)
	f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, true), nil)
	f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return([]model.Channel{{ID: 3, GroupID: 7, Name: "Email"}}, nil)
	f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{10, 11}).Return([]model.Link{
		{ID: 10, GroupID: 7, Source: model.LinkSourceLink, OriginalURL: "https://a.com/"},
		{ID: 11, GroupID: 7, Source: model.LinkSourceLink, OriginalURL: "https://b.com/"},
	}, nil)
	f.links.EXPECT().Update(gomock.Any(), gomock.Any()).Times(0)
	f.linkSvc.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	f.campaigns.EXPECT().AddLinks(gomock.Any(), uint64(5), uint64(3), []uint64{10, 11}).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionCampaignUpdate, service.EntityCampaign, uint64(5),
		map[string]any{"name": "Spring", "links": 2})

	require.NoError(t, f.svc.AddLinks(ctx, 1, 7, 5, 3, []uint64{10, 11, 10}))
}

func TestCampaignService_CreateLinkInChannel_AutoUTM(t *testing.T) {
	ctx := context.Background()
	run := func(t *testing.T, autoUTM bool, wantURL string) {
		f := newCampaignSvc(t)
		f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, autoUTM), nil)
		f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return([]model.Channel{{ID: 3, GroupID: 7, Name: "Email"}}, nil)
		domainID := uint64(4)
		f.linkSvc.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uint64, in service.CreateLinkInput) (*model.Link, error) {
				assert.Equal(t, wantURL, in.OriginalURL)
				assert.Equal(t, uint64(7), in.GroupID)
				assert.Equal(t, "T", in.Title)
				assert.Equal(t, "spring", in.CustomCode)
				assert.Equal(t, &domainID, in.DomainID)
				assert.Equal(t, []string{"a"}, in.Tags)
				return &model.Link{ID: 42, GroupID: 7, OriginalURL: in.OriginalURL, Source: model.LinkSourceLink}, nil
			})
		f.campaigns.EXPECT().AddLinks(gomock.Any(), uint64(5), uint64(3), []uint64{42}).Return(nil)
		f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionCampaignUpdate, service.EntityCampaign, uint64(5), gomock.Any())

		link, err := f.svc.CreateLinkInChannel(ctx, 1, 7, 5, 3, service.CreateLinkInChannelInput{
			OriginalURL: "https://x.com/", Title: "T", CustomCode: "spring", DomainID: &domainID, Tags: []string{"a"},
		})
		require.NoError(t, err)
		assert.Equal(t, uint64(42), link.ID)
	}
	// ApplyUTM writes utm_source, utm_medium, utm_campaign in that order.
	t.Run("auto utm on", func(t *testing.T) {
		run(t, true, "https://x.com/?utm_source=Email&utm_medium=bitly&utm_campaign=Spring")
	})
	t.Run("auto utm off", func(t *testing.T) { run(t, false, "https://x.com/") })

	t.Run("invalid url with auto utm", func(t *testing.T) {
		f := newCampaignSvc(t)
		f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, true), nil)
		f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return([]model.Channel{{ID: 3, GroupID: 7, Name: "Email"}}, nil)
		_, err := f.svc.CreateLinkInChannel(ctx, 1, 7, 5, 3, service.CreateLinkInChannelInput{OriginalURL: "not a url"})
		assertValidation(t, err)
	})
	t.Run("channel not in campaign", func(t *testing.T) {
		f := newCampaignSvc(t)
		f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, true), nil)
		f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return([]model.Channel{{ID: 3, GroupID: 7, Name: "Email"}}, nil)
		_, err := f.svc.CreateLinkInChannel(ctx, 1, 7, 5, 9, service.CreateLinkInChannelInput{OriginalURL: "https://x.com/"})
		assertValidation(t, err)
	})
}

func TestCampaignService_Stats_PerChannel(t *testing.T) {
	ctx := context.Background()
	f := newCampaignSvc(t)
	f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, false), nil)
	f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return([]model.Channel{
		{ID: 1, GroupID: 7, Name: "Email"},
		{ID: 2, GroupID: 7, Name: "Social"},
		{ID: 3, GroupID: 7, Name: "Print"},
	}, nil)
	f.campaigns.EXPECT().Links(gomock.Any(), uint64(5)).Return([]model.CampaignLink{
		{CampaignID: 5, ChannelID: 1, LinkID: 10},
		{CampaignID: 5, ChannelID: 1, LinkID: 11},
		{CampaignID: 5, ChannelID: 2, LinkID: 20},
	}, nil)
	from := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	f.clicks.EXPECT().SeriesScope(gomock.Any(), gomock.Any(), from, to, "day").Times(2).
		DoAndReturn(func(_ context.Context, scope repository.ClickScope, _, _ time.Time, _ string) ([]repository.Bucket, error) {
			assert.Equal(t, uint64(7), scope.GroupID)
			switch {
			case assert.ObjectsAreEqual([]uint64{10, 11}, scope.LinkIDs):
				return []repository.Bucket{{Date: "2026-10-23", Count: 1}, {Date: "2026-10-24", Count: 2}, {Date: "2026-10-25", Count: 0}}, nil
			case assert.ObjectsAreEqual([]uint64{20}, scope.LinkIDs):
				return []repository.Bucket{{Date: "2026-10-23", Count: 0}, {Date: "2026-10-24", Count: 0}, {Date: "2026-10-25", Count: 4}}, nil
			}
			t.Fatalf("unexpected link ids %v", scope.LinkIDs)
			return nil, nil
		})

	st, err := f.svc.Stats(ctx, 7, 5, service.StatsQuery{From: from, To: to})
	require.NoError(t, err)
	assert.Equal(t, 3, st.Totals.Links)
	assert.Equal(t, int64(7), st.Totals.Engagements)
	require.Len(t, st.Channels, 3)

	assert.Equal(t, uint64(1), st.Channels[0].ChannelID)
	assert.Equal(t, "Email", st.Channels[0].Name)
	assert.Equal(t, 2, st.Channels[0].Links)
	assert.Equal(t, int64(3), st.Channels[0].Engagements)
	assert.Equal(t, []service.SeriesPoint{{Date: "2026-10-23", Clicks: 1}, {Date: "2026-10-24", Clicks: 2}, {Date: "2026-10-25", Clicks: 0}}, st.Channels[0].Series)

	assert.Equal(t, int64(4), st.Channels[1].Engagements)
	assert.Equal(t, 1, st.Channels[1].Links)

	// Empty channel: zero-filled with the same keys and no repo call.
	assert.Equal(t, 0, st.Channels[2].Links)
	assert.Equal(t, int64(0), st.Channels[2].Engagements)
	assert.Equal(t, []service.SeriesPoint{{Date: "2026-10-23", Clicks: 0}, {Date: "2026-10-24", Clicks: 0}, {Date: "2026-10-25", Clicks: 0}}, st.Channels[2].Series)

	t.Run("invalid query", func(t *testing.T) {
		f := newCampaignSvc(t)
		f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, false), nil)
		_, err := f.svc.Stats(ctx, 7, 5, service.StatsQuery{Unit: "week"})
		assertValidation(t, err)
	})
	t.Run("hourly empty channel keys", func(t *testing.T) {
		f := newCampaignSvc(t)
		f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, false), nil)
		f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return([]model.Channel{{ID: 1, GroupID: 7, Name: "Email"}}, nil)
		f.campaigns.EXPECT().Links(gomock.Any(), uint64(5)).Return([]model.CampaignLink{}, nil)
		st, err := f.svc.Stats(ctx, 7, 5, service.StatsQuery{From: to, To: to, Unit: "hour"})
		require.NoError(t, err)
		require.Len(t, st.Channels[0].Series, 24)
		assert.Equal(t, "2026-10-25T00:00", st.Channels[0].Series[0].Date)
		assert.Equal(t, "2026-10-25T23:00", st.Channels[0].Series[23].Date)
	})
	t.Run("link in two channels counted once in totals", func(t *testing.T) {
		f := newCampaignSvc(t)
		f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, false), nil)
		f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return([]model.Channel{
			{ID: 1, GroupID: 7, Name: "Email"}, {ID: 2, GroupID: 7, Name: "Social"},
		}, nil)
		f.campaigns.EXPECT().Links(gomock.Any(), uint64(5)).Return([]model.CampaignLink{
			{CampaignID: 5, ChannelID: 1, LinkID: 10}, {CampaignID: 5, ChannelID: 2, LinkID: 10},
		}, nil)
		f.clicks.EXPECT().SeriesScope(gomock.Any(), gomock.Any(), to, to, "day").Times(2).
			Return([]repository.Bucket{{Date: "2026-10-25", Count: 5}}, nil)
		f.clicks.EXPECT().CountScope(gomock.Any(), repository.ClickScope{GroupID: 7, LinkIDs: []uint64{10}}, to, to).Return(int64(5), nil)
		st, err := f.svc.Stats(ctx, 7, 5, service.StatsQuery{From: to, To: to})
		require.NoError(t, err)
		assert.Equal(t, 1, st.Totals.Links)
		assert.Equal(t, int64(5), st.Totals.Engagements)
	})
}

func TestCampaignService_Delete_LogsActivity(t *testing.T) {
	ctx := context.Background()
	f := newCampaignSvc(t)
	f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, false), nil)
	f.campaigns.EXPECT().Delete(gomock.Any(), uint64(5)).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionCampaignDelete, service.EntityCampaign, uint64(5),
		map[string]any{"name": "Spring"})
	require.NoError(t, f.svc.Delete(ctx, 1, 7, 5))
}

func TestCampaignService_List_Engagements(t *testing.T) {
	ctx := context.Background()
	f := newCampaignSvc(t)
	f.campaigns.EXPECT().ListByGroup(gomock.Any(), uint64(7)).Return([]model.CampaignSummary{
		{Campaign: *sampleCampaign(5, 7, false), LinkCount: 2},
		{Campaign: *sampleCampaign(6, 7, false), LinkCount: 0},
	}, nil)
	f.campaigns.EXPECT().Links(gomock.Any(), uint64(5)).Return([]model.CampaignLink{
		{CampaignID: 5, ChannelID: 1, LinkID: 10}, {CampaignID: 5, ChannelID: 2, LinkID: 10}, {CampaignID: 5, ChannelID: 2, LinkID: 11},
	}, nil)
	f.clicks.EXPECT().CountScope(gomock.Any(), repository.ClickScope{GroupID: 7, LinkIDs: []uint64{10, 11}},
		time.Unix(0, 0).UTC(), f.now.Add(24*time.Hour)).Return(int64(9), nil)

	rows, err := f.svc.List(ctx, 7)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, int64(9), rows[0].Engagements)
	assert.Equal(t, int64(0), rows[1].Engagements)
}

func TestCampaignService_Get_BuildsChannelLinks(t *testing.T) {
	ctx := context.Background()
	f := newCampaignSvc(t)
	f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, false), nil)
	f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return([]model.Channel{
		{ID: 1, GroupID: 7, Name: "Email"}, {ID: 2, GroupID: 7, Name: "Social"},
	}, nil)
	f.campaigns.EXPECT().Links(gomock.Any(), uint64(5)).Return([]model.CampaignLink{
		{CampaignID: 5, ChannelID: 1, LinkID: 10}, {CampaignID: 5, ChannelID: 2, LinkID: 10},
	}, nil)
	f.linkSvc.EXPECT().List(gomock.Any(), uint64(7), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uint64, p service.ListLinksParams) (*service.ListLinksResult, error) {
			require.NotNil(t, p.Filter.CampaignID)
			assert.Equal(t, uint64(5), *p.Filter.CampaignID)
			return &service.ListLinksResult{
				Links: []model.LinkWithStats{{Link: model.Link{ID: 10, GroupID: 7}, TotalClicks: 3, Tags: []string{"a"}}},
				Total: 1, Page: 1, TotalPages: 1,
			}, nil
		})

	d, err := f.svc.Get(ctx, 7, 5)
	require.NoError(t, err)
	require.Len(t, d.Channels, 2)
	require.Len(t, d.Channels[0].Links, 1)
	assert.Equal(t, uint64(1), d.Channels[0].Links[0].ChannelID)
	assert.Equal(t, "Email", d.Channels[0].Links[0].ChannelName)
	assert.Equal(t, int64(3), d.Channels[0].Links[0].TotalClicks)
	require.Len(t, d.Channels[1].Links, 1)
	assert.Equal(t, "Social", d.Channels[1].Links[0].ChannelName)
}

func TestCampaignService_LinkMemberships(t *testing.T) {
	ctx := context.Background()
	f := newCampaignSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(10)).Return(&model.Link{ID: 10, GroupID: 7}, nil)
	f.campaigns.EXPECT().LinkCampaigns(gomock.Any(), uint64(10)).Return([]model.CampaignLink{
		{CampaignID: 5, ChannelID: 1, LinkID: 10}, {CampaignID: 5, ChannelID: 2, LinkID: 10},
	}, nil)
	f.campaigns.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleCampaign(5, 7, false), nil)
	f.campaigns.EXPECT().CampaignChannels(gomock.Any(), uint64(5)).Return([]model.Channel{
		{ID: 1, GroupID: 7, Name: "Email"}, {ID: 2, GroupID: 7, Name: "Social"},
	}, nil)
	ms, err := f.svc.LinkMemberships(ctx, 7, 10)
	require.NoError(t, err)
	assert.Equal(t, []service.LinkMembership{
		{CampaignID: 5, CampaignName: "Spring", ChannelID: 1, ChannelName: "Email"},
		{CampaignID: 5, CampaignName: "Spring", ChannelID: 2, ChannelName: "Social"},
	}, ms)

	f2 := newCampaignSvc(t)
	f2.links.EXPECT().GetByID(gomock.Any(), uint64(10)).Return(&model.Link{ID: 10, GroupID: 2}, nil)
	_, err = f2.svc.LinkMemberships(ctx, 7, 10)
	assert.ErrorIs(t, err, repository.ErrLinkNotFound)
}

func TestCampaignService_CreateChannel(t *testing.T) {
	ctx := context.Background()
	f := newCampaignSvc(t)
	_, err := f.svc.CreateChannel(ctx, 7, "  ")
	assertValidation(t, err)

	f.campaigns.EXPECT().CreateChannel(gomock.Any(), gomock.Any()).Return(repository.ErrChannelExists)
	_, err = f.svc.CreateChannel(ctx, 7, "Email")
	assert.ErrorIs(t, err, repository.ErrChannelExists)
}
