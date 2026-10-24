package service_test

import (
	"context"
	"database/sql"
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

type linkSvcFixture struct {
	svc      *service.LinkServiceImpl
	links    *mocks.MockLinkRepository
	tags     *mocks.MockTagRepository
	rules    *mocks.MockRoutingRuleRepository
	domains  *mocks.MockDomainRepository
	groups   *mocks.MockGroupRepository
	clicks   *mocks.MockClickRepository
	qrs      *mocks.MockQRRepository
	codes    *svcmocks.MockShortCodeService
	activity *svcmocks.MockActivityService
	titles   []string // URLs passed to the title fetcher
	// groupDefaults is the default_domain_id returned for a group id.
	groupDefaults map[uint64]*uint64
}

func newLinkSvc(t *testing.T) *linkSvcFixture {
	ctrl := gomock.NewController(t)
	f := &linkSvcFixture{
		links:    mocks.NewMockLinkRepository(ctrl),
		tags:     mocks.NewMockTagRepository(ctrl),
		rules:    mocks.NewMockRoutingRuleRepository(ctrl),
		domains:  mocks.NewMockDomainRepository(ctrl),
		groups:   mocks.NewMockGroupRepository(ctrl),
		clicks:   mocks.NewMockClickRepository(ctrl),
		qrs:      mocks.NewMockQRRepository(ctrl),
		codes:    svcmocks.NewMockShortCodeService(ctrl),
		activity: svcmocks.NewMockActivityService(ctrl),

		groupDefaults: map[uint64]*uint64{},
	}
	f.groups.EXPECT().GetByID(gomock.Any(), gomock.Any()).AnyTimes().
		DoAndReturn(func(_ context.Context, id uint64) (*model.Group, error) {
			return &model.Group{ID: id, UserID: 1, DefaultDomainID: f.groupDefaults[id]}, nil
		})
	f.svc = service.NewLinkService(service.LinkServiceDeps{
		Links:      f.links,
		ShortCodes: f.codes,
		Tags:       f.tags,
		Rules:      f.rules,
		Domains:    f.domains,
		Groups:     f.groups,
		Clicks:     f.clicks,
		QRs:        f.qrs,
		Activity:   f.activity,
		BaseURL:    "https://sho.rt",
		TitleFetcher: func(_ context.Context, rawURL string) string {
			f.titles = append(f.titles, rawURL)
			return "Fetched Title"
		},
	})
	return f
}

// expectCreate makes linkRepo.Create assign id and capture the link.
func (f *linkSvcFixture) expectCreate(id uint64, out **model.Link) {
	f.links.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, l *model.Link) error {
		l.ID = id
		if out != nil {
			*out = l
		}
		return nil
	})
}

func TestLinkService_Create_FetchesTitle(t *testing.T) {
	f := newLinkSvc(t)
	f.codes.EXPECT().Generate(gomock.Any(), (*uint64)(nil)).Return("Ab3dE5f", nil)
	var created *model.Link
	f.expectCreate(10, &created)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkCreate, service.EntityLink, uint64(10), gomock.Any())

	link, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://Example.com/page", GroupID: 7,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"https://Example.com/page"}, f.titles)
	require.NotNil(t, link.Title)
	assert.Equal(t, "Fetched Title", *link.Title)
	assert.Equal(t, uint64(7), created.GroupID)
	assert.Equal(t, uint64(1), created.UserID)
	require.NotNil(t, created.LongURLHost)
	assert.Equal(t, "example.com", *created.LongURLHost)
	assert.False(t, created.HasCustomBackHalf)
	assert.Equal(t, model.LinkSourceLink, created.Source)
	assert.True(t, created.IsActive)
}

func TestLinkService_Create_GivenTitleSkipsFetch(t *testing.T) {
	f := newLinkSvc(t)
	f.codes.EXPECT().IsValid("my-Brand_1").Return(true)
	f.codes.EXPECT().IsAvailable(gomock.Any(), (*uint64)(nil), "my-Brand_1").Return(true, nil)
	var created *model.Link
	f.expectCreate(11, &created)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkCreate, service.EntityLink, uint64(11), gomock.Any())

	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "Mine", CustomCode: "my-Brand_1", GroupID: 7,
	})
	require.NoError(t, err)
	assert.Empty(t, f.titles)
	assert.Equal(t, "Mine", *created.Title)
	assert.True(t, created.HasCustomBackHalf)
	assert.Equal(t, "my-Brand_1", created.ShortCode)
}

func TestLinkService_Create_RequiresGroup(t *testing.T) {
	f := newLinkSvc(t)
	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{OriginalURL: "https://example.com", Title: "x"})
	assert.ErrorIs(t, err, service.ErrGroupRequired)
}

func TestLinkService_Create_RejectsNonHTTP(t *testing.T) {
	f := newLinkSvc(t)
	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{OriginalURL: "javascript:alert(1)", Title: "x", GroupID: 7})
	assert.ErrorIs(t, err, service.ErrInvalidURL)
}

func TestLinkService_Create_BackHalfTaken(t *testing.T) {
	f := newLinkSvc(t)
	f.codes.EXPECT().IsValid("taken").Return(true)
	f.codes.EXPECT().IsAvailable(gomock.Any(), (*uint64)(nil), "taken").Return(false, nil)
	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "x", CustomCode: "taken", GroupID: 7,
	})
	assert.ErrorIs(t, err, service.ErrShortCodeTaken)
}

func TestLinkService_Create_ForeignDomain(t *testing.T) {
	f := newLinkSvc(t)
	f.domains.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Domain{ID: 5, UserID: 2, Domain: "x.test",
		VerificationStatus: model.DomainVerificationOK}, nil)
	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "x", GroupID: 7, DomainID: u64(5),
	})
	assert.ErrorIs(t, err, service.ErrDomainNotFound)
}

func TestLinkService_Create_AppliesUTM(t *testing.T) {
	f := newLinkSvc(t)
	f.codes.EXPECT().Generate(gomock.Any(), (*uint64)(nil)).Return("abcdefg", nil)
	var created *model.Link
	f.expectCreate(12, &created)
	f.activity.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com/p?a=1",
		Title:       "t",
		GroupID:     7,
		UTM:         &service.UTMParams{Source: "news", Medium: "email", Custom: []service.KV{{Key: "ref", Value: "x"}}},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/p?a=1&utm_source=news&utm_medium=email&ref=x", created.OriginalURL)
}

func TestLinkService_Create_SavesTagsAndRules(t *testing.T) {
	f := newLinkSvc(t)
	f.codes.EXPECT().Generate(gomock.Any(), (*uint64)(nil)).Return("abcdefg", nil)
	var created *model.Link
	f.expectCreate(13, &created)
	f.tags.EXPECT().SetLinkTags(gomock.Any(), uint64(13), []string{"news", "promo"}).Return(nil)
	f.rules.EXPECT().ReplaceForLink(gomock.Any(), uint64(13), []model.RoutingRule{
		{LinkID: 13, LongURL: "https://m.example.com", DeviceMatch: model.StringList{"mobile"}, OSMatch: model.StringList{},
			CountryMatch: model.StringList{"US"}, CountryExclude: model.StringList{}},
	}).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	link, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL:  "https://example.com",
		Title:        "t",
		GroupID:      7,
		Tags:         []string{" news ", "promo", "news"},
		ExpiresAt:    &exp,
		RoutingRules: []service.RoutingRuleInput{{LongURL: "https://m.example.com", DeviceMatch: []string{"mobile"}, CountryMatch: []string{"US"}}},
	})
	require.NoError(t, err)
	assert.True(t, created.ExpiresAt.Valid)
	assert.Equal(t, exp, created.ExpiresAt.Time)
	assert.Equal(t, uint64(13), link.ID)
}

func TestLinkService_Create_InvalidRoutingRule(t *testing.T) {
	f := newLinkSvc(t)
	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "t", GroupID: 7,
		RoutingRules: []service.RoutingRuleInput{{LongURL: "ftp://nope"}},
	})
	assert.ErrorIs(t, err, service.ErrInvalidRoutingRule)
}

func TestLinkService_Delete_NotDeletableAfterRedirect(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, RedirectCount: 1}, nil)
	_, err := f.svc.Delete(context.Background(), 1, 3)
	assert.ErrorIs(t, err, service.ErrLinkNotDeletable)
}

func TestLinkService_Delete_NotDeletableCustomBackHalf(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, HasCustomBackHalf: true}, nil)
	_, err := f.svc.Delete(context.Background(), 1, 3)
	assert.ErrorIs(t, err, service.ErrLinkNotDeletable)
}

func TestLinkService_Delete_Deletable(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, ShortCode: "abc"}, nil)
	f.links.EXPECT().Delete(gomock.Any(), uint64(3)).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkDelete, service.EntityLink, uint64(3), gomock.Any())
	link, err := f.svc.Delete(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.Equal(t, "abc", link.ShortCode)
}

func TestLinkService_Delete_OtherUser(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 2}, nil)
	_, err := f.svc.Delete(context.Background(), 1, 3)
	assert.ErrorIs(t, err, service.ErrNotLinkOwner)
}

func TestLinkService_EditBackHalf_CreatesChild(t *testing.T) {
	f := newLinkSvc(t)
	title := "Old title"
	host := "example.com"
	exp := model.NullTime{}
	exp.Time, exp.Valid = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), true
	old := &model.Link{ID: 3, UserID: 1, GroupID: 7, ShortCode: "abc1234", OriginalURL: "https://example.com/x",
		LongURLHost: &host, Title: &title, ExpiresAt: exp, IsActive: true, Source: model.LinkSourceLink, RedirectCount: 2}
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(old, nil)
	f.codes.EXPECT().IsValid("New-Half").Return(true)
	f.codes.EXPECT().IsAvailable(gomock.Any(), (*uint64)(nil), "New-Half").Return(true, nil)
	f.tags.EXPECT().GetLinkTags(gomock.Any(), uint64(3)).Return([]string{"a", "b"}, nil)
	f.rules.EXPECT().ListByLinkID(gomock.Any(), uint64(3)).Return([]model.RoutingRule{
		{ID: 50, LinkID: 3, Position: 0, LongURL: "https://m.example.com"},
	}, nil)
	var created *model.Link
	f.expectCreate(20, &created)
	f.tags.EXPECT().SetLinkTags(gomock.Any(), uint64(20), []string{"a", "b"}).Return(nil)
	f.rules.EXPECT().ReplaceForLink(gomock.Any(), uint64(20), []model.RoutingRule{
		{LinkID: 20, Position: 0, LongURL: "https://m.example.com"},
	}).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkBackHalf, service.EntityLink, uint64(20), gomock.Any())

	link, err := f.svc.EditBackHalf(context.Background(), 1, 3, nil, "New-Half")
	require.NoError(t, err)
	assert.Equal(t, uint64(20), link.ID)
	assert.Equal(t, "New-Half", created.ShortCode)
	require.NotNil(t, created.ParentLinkID)
	assert.Equal(t, uint64(3), *created.ParentLinkID)
	assert.True(t, created.HasCustomBackHalf)
	assert.Equal(t, 0, created.RedirectCount)
	assert.Equal(t, "Old title", *created.Title)
	assert.Equal(t, old.OriginalURL, created.OriginalURL)
	assert.Equal(t, old.ExpiresAt, created.ExpiresAt)
	assert.Equal(t, uint64(7), created.GroupID)
	assert.Nil(t, created.DomainID)
	assert.Empty(t, f.titles)
	// The old link is untouched.
	assert.Equal(t, "abc1234", old.ShortCode)
}

func TestLinkService_EditBackHalf_DomainZeroMeansDefault(t *testing.T) {
	f := newLinkSvc(t)
	old := &model.Link{ID: 3, UserID: 1, GroupID: 7, ShortCode: "abc", OriginalURL: "https://e.com", DomainID: u64(5)}
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(old, nil)
	f.codes.EXPECT().IsValid("x").Return(true)
	f.codes.EXPECT().IsAvailable(gomock.Any(), (*uint64)(nil), "x").Return(false, nil)
	_, err := f.svc.EditBackHalf(context.Background(), 1, 3, u64(0), "x")
	assert.ErrorIs(t, err, service.ErrShortCodeTaken)
}

func TestLinkService_Update_DestinationIncrementsRedirect(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, OriginalURL: "https://old.example.com"}, nil)
	f.links.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, l *model.Link) error {
		assert.Equal(t, "https://new.example.org/x", l.OriginalURL)
		require.NotNil(t, l.LongURLHost)
		assert.Equal(t, "new.example.org", *l.LongURLHost)
		return nil
	})
	f.links.EXPECT().IncrementRedirectCount(gomock.Any(), uint64(3)).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkUpdate, service.EntityLink, uint64(3), gomock.Any())

	link, prev, err := f.svc.Update(context.Background(), 1, 3, service.UpdateLinkInput{OriginalURL: "https://new.example.org/x"})
	require.NoError(t, err)
	assert.Equal(t, "https://old.example.com", prev.OriginalURL)
	assert.Equal(t, 1, link.RedirectCount)
}

func TestLinkService_Update_SameDestinationNoRedirect(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, OriginalURL: "https://e.com"}, nil)
	f.links.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	f.tags.EXPECT().SetLinkTags(gomock.Any(), uint64(3), []string{"x"}).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), service.ActionLinkUpdate, gomock.Any(), gomock.Any(), gomock.Any())
	tags := []string{"x"}
	newTitle := "T"
	link, _, err := f.svc.Update(context.Background(), 1, 3, service.UpdateLinkInput{OriginalURL: "https://e.com", Title: &newTitle, Tags: &tags})
	require.NoError(t, err)
	assert.Equal(t, 0, link.RedirectCount)
	assert.Equal(t, "T", *link.Title)
}

func TestLinkService_SetArchived_ForeignIDs404(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{1, 2}).Return([]model.Link{{ID: 1, GroupID: 7}, {ID: 2, GroupID: 8}}, nil)
	_, err := f.svc.SetArchived(context.Background(), 1, 7, []uint64{1, 2, 1}, true)
	assert.ErrorIs(t, err, service.ErrLinkNotFound)
}

func TestLinkService_SetArchived_LogsHide(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{1, 2}).Return([]model.Link{{ID: 1, GroupID: 7}, {ID: 2, GroupID: 7}}, nil)
	f.links.EXPECT().SetArchived(gomock.Any(), []uint64{1, 2}, true).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkHide, service.EntityLink, uint64(1), gomock.Any())
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkHide, service.EntityLink, uint64(2), gomock.Any())
	links, err := f.svc.SetArchived(context.Background(), 1, 7, []uint64{1, 2}, true)
	require.NoError(t, err)
	require.Len(t, links, 2)
	assert.True(t, links[0].Archived)
}

func TestLinkService_BulkTags(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{4, 5}).Return([]model.Link{{ID: 4, GroupID: 7}, {ID: 5, GroupID: 7}}, nil)
	f.tags.EXPECT().AddTagsToLinks(gomock.Any(), []uint64{4, 5}, []string{"new"}).Return(nil)
	f.tags.EXPECT().RemoveTagsFromLinks(gomock.Any(), []uint64{4, 5}, []string{"old"}).Return(nil)
	require.NoError(t, f.svc.BulkTags(context.Background(), 7, []uint64{4, 5}, []string{" new "}, []string{"old"}))
}

func TestLinkService_BulkTags_ForeignIDs404(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{4, 99}).Return([]model.Link{{ID: 4, GroupID: 7}}, nil)
	err := f.svc.BulkTags(context.Background(), 7, []uint64{4, 99}, []string{"x"}, nil)
	assert.ErrorIs(t, err, service.ErrLinkNotFound)
}

func TestLinkService_List_DefaultsAndTags(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().ListByGroup(gomock.Any(), uint64(7), repository.LinkFilter{Query: "x"}, 20, 20).
		Return([]model.LinkWithStats{{Link: model.Link{ID: 1}}, {Link: model.Link{ID: 2}}}, int64(22), nil)
	f.tags.EXPECT().GetTagsForLinks(gomock.Any(), []uint64{1, 2}).Return(map[uint64][]string{1: {"a"}}, nil)

	res, err := f.svc.List(context.Background(), 7, service.ListLinksParams{Filter: repository.LinkFilter{Query: "x"}, Page: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(22), res.Total)
	assert.Equal(t, 2, res.TotalPages)
	assert.Equal(t, []string{"a"}, res.Links[0].Tags)
	assert.NotNil(t, res.Links[1].Tags)
	assert.Empty(t, res.Links[1].Tags)
}

func TestLinkService_GetDetails_History(t *testing.T) {
	f := newLinkSvc(t)
	created := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(30)).Return(&model.Link{ID: 30, UserID: 1, ParentLinkID: u64(20)}, nil)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(20)).Return(&model.Link{ID: 20, UserID: 1, ShortCode: "b", ParentLinkID: u64(10)}, nil)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(10)).Return(&model.Link{ID: 10, UserID: 1, ShortCode: "a", CreatedAt: created}, nil)
	f.tags.EXPECT().GetLinkTags(gomock.Any(), uint64(30)).Return([]string{}, nil)
	f.rules.EXPECT().ListByLinkID(gomock.Any(), uint64(30)).Return(nil, nil)
	f.clicks.EXPECT().GetTotalByLinkID(gomock.Any(), uint64(30)).Return(int64(9), nil)
	f.qrs.EXPECT().QRCodeIDsForLinks(gomock.Any(), []uint64{30}).Return(map[uint64]uint64{}, nil)

	d, err := f.svc.GetDetails(context.Background(), 1, 30)
	require.NoError(t, err)
	assert.Equal(t, int64(9), d.TotalClicks)
	assert.Nil(t, d.QRCodeID)
	require.Len(t, d.History, 2)
	assert.Equal(t, "b", d.History[0].ShortCode)
	assert.Equal(t, "a", d.History[1].ShortCode)
	assert.NotNil(t, d.RoutingRules)
	assert.NotNil(t, d.Tags)
}

func TestLinkService_Export_Rows(t *testing.T) {
	f := newLinkSvc(t)
	title := `=HYPERLINK("x")`
	created := time.Date(2026, 10, 2, 3, 4, 5, 0, time.UTC)
	f.links.EXPECT().ListByGroup(gomock.Any(), uint64(7), repository.LinkFilter{}, gomock.Any(), 0).
		Return([]model.LinkWithStats{
			{Link: model.Link{ID: 1, ShortCode: "abc", OriginalURL: "https://e.com", Title: &title, CreatedAt: created}, TotalClicks: 4},
			{Link: model.Link{ID: 2, ShortCode: "brand", OriginalURL: "https://f.com", HasCustomBackHalf: true, Archived: true,
				DomainID: u64(5), CreatedAt: created}},
		}, int64(2), nil)
	f.domains.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Domain{ID: 5, Domain: "go.brand.test"}, nil)

	rows, err := f.svc.Export(context.Background(), 7, repository.LinkFilter{})
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"https://sho.rt/abc", "", "2026-10-02T03:04:05Z", `'=HYPERLINK("x")`, "https://e.com", "4", "active"},
		{"https://go.brand.test/brand", "https://go.brand.test/brand", "2026-10-02T03:04:05Z", "", "https://f.com", "0", "hidden"},
	}, rows)
}

func TestLinkService_Create_UnverifiedDomainRejected(t *testing.T) {
	f := newLinkSvc(t)
	f.domains.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Domain{ID: 5, UserID: 1, Domain: "x.test",
		VerificationStatus: model.DomainVerificationPending}, nil)
	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "x", GroupID: 7, DomainID: u64(5),
	})
	assert.ErrorIs(t, err, service.ErrDomainNotVerified)
}

func TestLinkService_Update_ClearsExpiration(t *testing.T) {
	f := newLinkSvc(t)
	exp := model.NullTime{}
	exp.Time, exp.Valid = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), true
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, OriginalURL: "https://e.com", ExpiresAt: exp}, nil)
	f.links.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, l *model.Link) error {
		assert.False(t, l.ExpiresAt.Valid)
		return nil
	})
	f.activity.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), service.ActionLinkUpdate, gomock.Any(), gomock.Any(), gomock.Any())
	link, prev, err := f.svc.Update(context.Background(), 1, 3, service.UpdateLinkInput{ClearExpiration: true})
	require.NoError(t, err)
	assert.False(t, link.ExpiresAt.Valid)
	assert.True(t, prev.ExpiresAt.Valid)
}

func TestLinkService_Update_ClearsTitle(t *testing.T) {
	f := newLinkSvc(t)
	title := "Old"
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, OriginalURL: "https://e.com", Title: &title}, nil)
	f.links.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, l *model.Link) error {
		assert.Nil(t, l.Title)
		return nil
	})
	f.activity.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), service.ActionLinkUpdate, gomock.Any(), gomock.Any(), gomock.Any())
	empty := ""
	link, _, err := f.svc.Update(context.Background(), 1, 3, service.UpdateLinkInput{Title: &empty})
	require.NoError(t, err)
	assert.Nil(t, link.Title)
}

func TestLinkService_SetRoutingRules_ReplacesAndReturns(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, ShortCode: "abc"}, nil)
	f.rules.EXPECT().ReplaceForLink(gomock.Any(), uint64(3), []model.RoutingRule{
		{LinkID: 3, LongURL: "https://ios.example.com", DeviceMatch: model.StringList{"mobile"}, OSMatch: model.StringList{"ios"},
			CountryMatch: model.StringList{"US", "FR"}, CountryExclude: model.StringList{}},
		{LinkID: 3, LongURL: "https://eu.example.com", DeviceMatch: model.StringList{}, OSMatch: model.StringList{},
			CountryMatch: model.StringList{}, CountryExclude: model.StringList{"DE"}},
	}).DoAndReturn(func(_ context.Context, linkID uint64, rules []model.RoutingRule) error {
		for i := range rules {
			rules[i].ID = uint64(100 + i)
			rules[i].Position = i
		}
		return nil
	})
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkUpdate, service.EntityLink, uint64(3),
		map[string]any{"short_code": "abc", "routing_rules": 2})

	rules, err := f.svc.SetRoutingRules(context.Background(), 1, 3, []service.RoutingRuleInput{
		{LongURL: " https://ios.example.com ", DeviceMatch: []string{"Mobile"}, OSMatch: []string{"iOS"}, CountryMatch: []string{"US", "FR"}},
		{LongURL: "https://eu.example.com", CountryExclude: []string{"DE"}},
	})
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, uint64(100), rules[0].ID)
	assert.Equal(t, 0, rules[0].Position)
	assert.Equal(t, uint64(101), rules[1].ID)
	assert.Equal(t, 1, rules[1].Position)
	assert.Equal(t, "https://eu.example.com", rules[1].LongURL)
}

func TestLinkService_SetRoutingRules_EmptyClears(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, ShortCode: "abc"}, nil)
	f.rules.EXPECT().ReplaceForLink(gomock.Any(), uint64(3), []model.RoutingRule{}).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkUpdate, service.EntityLink, uint64(3),
		map[string]any{"short_code": "abc", "routing_rules": 0})

	rules, err := f.svc.SetRoutingRules(context.Background(), 1, 3, nil)
	require.NoError(t, err)
	assert.NotNil(t, rules)
	assert.Empty(t, rules)
}

func TestLinkService_SetRoutingRules_InvalidURL(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7}, nil).AnyTimes()
	_, err := f.svc.SetRoutingRules(context.Background(), 1, 3, []service.RoutingRuleInput{{LongURL: "javascript:alert(1)"}})
	assert.ErrorIs(t, err, service.ErrInvalidRoutingRule)
	assert.Contains(t, err.Error(), "rule 1")
}

func TestLinkService_SetRoutingRules_InvalidMatchValues(t *testing.T) {
	tooMany := make([]service.RoutingRuleInput, 21)
	for i := range tooMany {
		tooMany[i] = service.RoutingRuleInput{LongURL: "https://e.com"}
	}
	cases := map[string][]service.RoutingRuleInput{
		"device_match":    {{LongURL: "https://e.com", DeviceMatch: []string{"phone"}}},
		"os_match":        {{LongURL: "https://e.com", OSMatch: []string{"windows"}}},
		"country_match":   {{LongURL: "https://e.com", CountryMatch: []string{"USA"}}},
		"country_exclude": {{LongURL: "https://e.com", CountryExclude: []string{"1A"}}},
		"at most 20":      tooMany,
	}
	for want, in := range cases {
		f := newLinkSvc(t)
		f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7}, nil).AnyTimes()
		_, err := f.svc.SetRoutingRules(context.Background(), 1, 3, in)
		assert.ErrorIs(t, err, service.ErrInvalidRoutingRule, want)
		if err != nil {
			assert.Contains(t, err.Error(), want)
		}
	}
}

func TestLinkService_SetRoutingRules_NotOwner(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 2, GroupID: 7}, nil)
	_, err := f.svc.SetRoutingRules(context.Background(), 1, 3, []service.RoutingRuleInput{{LongURL: "https://e.com"}})
	assert.ErrorIs(t, err, service.ErrNotLinkOwner)
}

func TestLinkService_GetRoutingRules(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7}, nil)
	f.rules.EXPECT().ListByLinkID(gomock.Any(), uint64(3)).Return(nil, nil)
	rules, err := f.svc.GetRoutingRules(context.Background(), 1, 3)
	require.NoError(t, err)
	assert.NotNil(t, rules)
	assert.Empty(t, rules)

	f2 := newLinkSvc(t)
	f2.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 2}, nil)
	_, err = f2.svc.GetRoutingRules(context.Background(), 1, 3)
	assert.ErrorIs(t, err, service.ErrNotLinkOwner)
}

func TestLinkService_Create_UsesGroupDefaultDomain(t *testing.T) {
	f := newLinkSvc(t)
	f.groupDefaults[7] = u64(5)
	f.domains.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Domain{ID: 5, UserID: 1, Domain: "go.brand.test",
		VerificationStatus: model.DomainVerificationOK}, nil)
	f.codes.EXPECT().Generate(gomock.Any(), u64(5)).Return("Ab3dE5f", nil)
	var created *model.Link
	f.expectCreate(12, &created)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkCreate, service.EntityLink, uint64(12), gomock.Any())

	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "x", GroupID: 7,
	})
	require.NoError(t, err)
	require.NotNil(t, created.DomainID)
	assert.Equal(t, uint64(5), *created.DomainID)
}

func TestLinkService_Create_ExplicitDefaultDomainOverridesGroupDefault(t *testing.T) {
	f := newLinkSvc(t)
	f.groupDefaults[7] = u64(5)
	f.codes.EXPECT().Generate(gomock.Any(), (*uint64)(nil)).Return("Ab3dE5f", nil)
	var created *model.Link
	f.expectCreate(13, &created)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkCreate, service.EntityLink, uint64(13), gomock.Any())

	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "x", GroupID: 7, DomainID: u64(0),
	})
	require.NoError(t, err)
	assert.Nil(t, created.DomainID)
}

func TestLinkService_Create_UnusableGroupDefaultFallsBack(t *testing.T) {
	f := newLinkSvc(t)
	f.groupDefaults[7] = u64(5)
	f.domains.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Domain{ID: 5, UserID: 1, Domain: "go.brand.test",
		VerificationStatus: model.DomainVerificationError}, nil)
	f.codes.EXPECT().Generate(gomock.Any(), (*uint64)(nil)).Return("Ab3dE5f", nil)
	var created *model.Link
	f.expectCreate(14, &created)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkCreate, service.EntityLink, uint64(14), gomock.Any())

	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "x", GroupID: 7,
	})
	require.NoError(t, err)
	assert.Nil(t, created.DomainID)
}

func TestLinkService_Create_DomainOfOtherGroupRejected(t *testing.T) {
	f := newLinkSvc(t)
	f.domains.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Domain{ID: 5, UserID: 1, GroupID: u64(8),
		Domain: "go.brand.test", VerificationStatus: model.DomainVerificationOK}, nil)
	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "x", GroupID: 7, DomainID: u64(5),
	})
	assert.ErrorIs(t, err, service.ErrDomainNotFound)
}

func TestLinkService_Create_ExpirationTooSoon(t *testing.T) {
	f := newLinkSvc(t)
	soon := time.Now().Add(time.Minute)
	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "x", GroupID: 7, ExpiresAt: &soon,
	})
	assert.ErrorIs(t, err, service.ErrExpirationTooSoon)
	assert.EqualError(t, err, "expiration must be at least 5 minutes in the future")
}

func TestLinkService_Update_ExpirationTooSoon(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(10)).Return(&model.Link{ID: 10, UserID: 1, GroupID: 7, ShortCode: "abc",
		OriginalURL: "https://example.com", IsActive: true}, nil)
	past := time.Now().Add(-time.Hour)
	_, _, err := f.svc.Update(context.Background(), 1, 10, service.UpdateLinkInput{ExpiresAt: &past})
	assert.ErrorIs(t, err, service.ErrExpirationTooSoon)
}

func TestLinkService_Update_UnchangedExpirationNotRevalidated(t *testing.T) {
	f := newLinkSvc(t)
	past := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(10)).Return(&model.Link{ID: 10, UserID: 1, GroupID: 7, ShortCode: "abc",
		OriginalURL: "https://example.com", IsActive: true,
		ExpiresAt: model.NullTime{NullTime: sql.NullTime{Time: past, Valid: true}}}, nil)
	f.links.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionLinkUpdate, service.EntityLink, uint64(10), gomock.Any())
	title := "renamed"
	_, _, err := f.svc.Update(context.Background(), 1, 10, service.UpdateLinkInput{Title: &title, ExpiresAt: &past})
	require.NoError(t, err)
}

func TestLinkService_Update_DomainZeroMovesToDefault(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, ShortCode: "abc", OriginalURL: "https://e.com", DomainID: u64(7)}, nil)
	f.codes.EXPECT().IsAvailable(gomock.Any(), (*uint64)(nil), "abc").Return(true, nil)
	f.links.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, l *model.Link) error {
		assert.Nil(t, l.DomainID)
		return nil
	})
	f.activity.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), service.ActionLinkUpdate, gomock.Any(), gomock.Any(), gomock.Any())
	link, prev, err := f.svc.Update(context.Background(), 1, 3, service.UpdateLinkInput{DomainID: u64(0)})
	require.NoError(t, err)
	assert.Nil(t, link.DomainID)
	require.NotNil(t, prev.DomainID)
	assert.Equal(t, uint64(7), *prev.DomainID)
}

func TestLinkService_Update_DomainZeroTaken(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, ShortCode: "abc", OriginalURL: "https://e.com", DomainID: u64(7)}, nil)
	f.codes.EXPECT().IsAvailable(gomock.Any(), (*uint64)(nil), "abc").Return(false, nil)
	_, _, err := f.svc.Update(context.Background(), 1, 3, service.UpdateLinkInput{DomainID: u64(0)})
	assert.ErrorIs(t, err, service.ErrShortCodeTaken)
}

func TestLinkService_Update_DomainZeroWhenAlreadyDefault(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Link{ID: 3, UserID: 1, GroupID: 7, ShortCode: "abc", OriginalURL: "https://e.com"}, nil)
	f.links.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), service.ActionLinkUpdate, gomock.Any(), gomock.Any(), gomock.Any())
	link, _, err := f.svc.Update(context.Background(), 1, 3, service.UpdateLinkInput{DomainID: u64(0)})
	require.NoError(t, err)
	assert.Nil(t, link.DomainID)
}

func TestLinkService_Create_SourceQR_NoActivity(t *testing.T) {
	f := newLinkSvc(t)
	f.codes.EXPECT().Generate(gomock.Any(), (*uint64)(nil)).Return("Qr3dE5f", nil)
	var created *model.Link
	f.expectCreate(12, &created)
	// No activity expectation: a link.create entry fails the test.

	_, err := f.svc.Create(context.Background(), 1, service.CreateLinkInput{
		OriginalURL: "https://example.com", Title: "T", GroupID: 7, Source: model.LinkSourceQR,
	})
	require.NoError(t, err)
	assert.Equal(t, model.LinkSourceQR, created.Source)
}

func TestLinkService_GetDetails_FillsQRCodeID(t *testing.T) {
	f := newLinkSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(30)).Return(&model.Link{ID: 30, UserID: 1}, nil)
	f.tags.EXPECT().GetLinkTags(gomock.Any(), uint64(30)).Return([]string{}, nil)
	f.rules.EXPECT().ListByLinkID(gomock.Any(), uint64(30)).Return(nil, nil)
	f.clicks.EXPECT().GetTotalByLinkID(gomock.Any(), uint64(30)).Return(int64(0), nil)
	f.qrs.EXPECT().QRCodeIDsForLinks(gomock.Any(), []uint64{30}).Return(map[uint64]uint64{30: 5}, nil)

	d, err := f.svc.GetDetails(context.Background(), 1, 30)
	require.NoError(t, err)
	require.NotNil(t, d.QRCodeID)
	assert.Equal(t, uint64(5), *d.QRCodeID)
}
