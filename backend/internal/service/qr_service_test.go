package service_test

import (
	"context"
	"strings"
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

type qrSvcFixture struct {
	svc      *service.QRServiceImpl
	qrs      *mocks.MockQRRepository
	links    *svcmocks.MockLinkService
	linkRepo *mocks.MockLinkRepository
	domains  *mocks.MockDomainRepository
	activity *svcmocks.MockActivityService
}

func newQRSvc(t *testing.T) *qrSvcFixture {
	ctrl := gomock.NewController(t)
	f := &qrSvcFixture{
		qrs:      mocks.NewMockQRRepository(ctrl),
		links:    svcmocks.NewMockLinkService(ctrl),
		linkRepo: mocks.NewMockLinkRepository(ctrl),
		domains:  mocks.NewMockDomainRepository(ctrl),
		activity: svcmocks.NewMockActivityService(ctrl),
	}
	f.svc = service.NewQRService(service.QRServiceDeps{
		QRs:      f.qrs,
		Links:    f.links,
		LinkRepo: f.linkRepo,
		Activity: f.activity,
		Domains:  f.domains,
		BaseURL:  "https://sho.rt",
	})
	return f
}

// expectQRCreate makes QRRepository.Create assign id and capture the QR code.
func (f *qrSvcFixture) expectQRCreate(id uint64, out **model.QRCode) {
	f.qrs.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, qr *model.QRCode) error {
		qr.ID = id
		if out != nil {
			*out = qr
		}
		return nil
	})
}

func qrRow(id, userID, groupID uint64, link model.Link) *repository.QRCodeRow {
	return &repository.QRCodeRow{
		QRCode: model.QRCode{ID: id, UserID: userID, GroupID: groupID, LinkID: link.ID,
			RenderCustomizations: model.JSONMap{}},
		LinkUserID:            link.UserID,
		LinkShortCode:         link.ShortCode,
		LinkOriginalURL:       link.OriginalURL,
		LinkTitle:             link.Title,
		LinkDomainID:          link.DomainID,
		LinkSource:            link.Source,
		LinkRedirectCount:     link.RedirectCount,
		LinkHasCustomBackHalf: link.HasCustomBackHalf,
		LinkIsActive:          true,
		LinkCreatedAt:         link.CreatedAt,
	}
}

func TestQRService_Create_WithLongURL_CreatesQRLink(t *testing.T) {
	f := newQRSvc(t)
	var got service.CreateLinkInput
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uint64, in service.CreateLinkInput) (*model.Link, error) {
			got = in
			return &model.Link{ID: 50, UserID: 1, GroupID: 7, ShortCode: "Ab3dE5f", Source: model.LinkSourceQR}, nil
		})
	var created *model.QRCode
	f.expectQRCreate(9, &created)
	f.qrs.EXPECT().SetTags(gomock.Any(), uint64(9), []string{"Menu"}).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionQRCreate, service.EntityQRCode, uint64(9), gomock.Any())

	d, err := f.svc.Create(context.Background(), 1, service.CreateQRCodeInput{
		LongURL: "https://example.com/menu", CustomCode: "ignored", Title: "Menu",
		Tags: []string{" Menu "}, GroupID: 7,
	})
	require.NoError(t, err)
	assert.Equal(t, model.LinkSourceQR, got.Source)
	assert.Empty(t, got.CustomCode)
	assert.Empty(t, got.Tags)
	assert.Equal(t, "https://example.com/menu", got.OriginalURL)
	assert.Equal(t, "Menu", got.Title)
	assert.Equal(t, uint64(7), got.GroupID)

	assert.Equal(t, uint64(50), created.LinkID)
	assert.Equal(t, uint64(1), created.UserID)
	assert.Equal(t, uint64(7), created.GroupID)
	require.NotNil(t, created.Title)
	assert.Equal(t, "Menu", *created.Title)
	assert.Equal(t, model.JSONMap{}, created.RenderCustomizations)
	assert.Equal(t, uint64(9), d.ID)
	assert.Equal(t, "Ab3dE5f", d.Link.ShortCode)
	assert.Equal(t, []string{"Menu"}, d.Tags)
}

func TestQRService_Create_ShareAsLink_UsesLinkSource(t *testing.T) {
	f := newQRSvc(t)
	var got service.CreateLinkInput
	f.links.EXPECT().Create(gomock.Any(), uint64(1), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uint64, in service.CreateLinkInput) (*model.Link, error) {
			got = in
			return &model.Link{ID: 51, UserID: 1, GroupID: 7, ShortCode: "brand", Source: model.LinkSourceLink}, nil
		})
	var created *model.QRCode
	f.expectQRCreate(10, &created)
	f.qrs.EXPECT().SetTags(gomock.Any(), uint64(10), []string{"a", "b"}).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionQRCreate, service.EntityQRCode, uint64(10), gomock.Any())

	_, err := f.svc.Create(context.Background(), 1, service.CreateQRCodeInput{
		LongURL: "https://example.com", ShareAsLink: true, CustomCode: "brand",
		Tags: []string{"a", "b"}, GroupID: 7,
	})
	require.NoError(t, err)
	assert.Equal(t, model.LinkSourceLink, got.Source)
	assert.Equal(t, "brand", got.CustomCode)
	assert.Equal(t, []string{"a", "b"}, got.Tags)
	assert.Nil(t, created.Title, "blank title is stored as NULL")
}

func TestQRService_Create_WithLinkID_AttachesExisting(t *testing.T) {
	f := newQRSvc(t)
	link := &model.Link{ID: 60, UserID: 1, GroupID: 7, ShortCode: "x1", Source: model.LinkSourceLink}
	f.links.EXPECT().GetByID(gomock.Any(), uint64(1), uint64(60)).Return(link, nil)
	f.qrs.EXPECT().GetByLinkID(gomock.Any(), uint64(60)).Return(nil, repository.ErrQRCodeNotFound)
	var created *model.QRCode
	f.expectQRCreate(11, &created)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionQRCreate, service.EntityQRCode, uint64(11), gomock.Any())

	d, err := f.svc.Create(context.Background(), 1, service.CreateQRCodeInput{
		LinkID: u64(60), RenderCustomizations: model.JSONMap{"color": "#112233", "error_correction": "Q"}, GroupID: 7,
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(60), created.LinkID)
	assert.Equal(t, "#112233", created.RenderCustomizations["color"])
	assert.Equal(t, "x1", d.Link.ShortCode)
	assert.NotNil(t, d.Tags)
}

func TestQRService_Create_WithLinkID_AlreadyHasQR(t *testing.T) {
	f := newQRSvc(t)
	link := &model.Link{ID: 60, UserID: 1, GroupID: 7, Source: model.LinkSourceLink}
	f.links.EXPECT().GetByID(gomock.Any(), uint64(1), uint64(60)).Return(link, nil)
	f.qrs.EXPECT().GetByLinkID(gomock.Any(), uint64(60)).Return(qrRow(3, 1, 7, *link), nil)

	_, err := f.svc.Create(context.Background(), 1, service.CreateQRCodeInput{LinkID: u64(60), GroupID: 7})
	assert.ErrorIs(t, err, service.ErrQRCodeExistsForLink)
	assert.Equal(t, "This link already has a QR Code", err.Error())
}

func TestQRService_Create_WithLinkID_QRSourceLinkRejected(t *testing.T) {
	f := newQRSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(1), uint64(61)).
		Return(&model.Link{ID: 61, UserID: 1, GroupID: 7, Source: model.LinkSourceQR}, nil)
	_, err := f.svc.Create(context.Background(), 1, service.CreateQRCodeInput{LinkID: u64(61), GroupID: 7})
	assert.ErrorIs(t, err, service.ErrLinkNotFound)

	// A link of another group is not found either.
	f.links.EXPECT().GetByID(gomock.Any(), uint64(1), uint64(62)).
		Return(&model.Link{ID: 62, UserID: 1, GroupID: 8, Source: model.LinkSourceLink}, nil)
	_, err = f.svc.Create(context.Background(), 1, service.CreateQRCodeInput{LinkID: u64(62), GroupID: 7})
	assert.ErrorIs(t, err, service.ErrLinkNotFound)
}

func TestQRService_Create_InvalidDesign(t *testing.T) {
	f := newQRSvc(t)
	big := model.JSONMap{"logo": strings.Repeat("x", 9*1024)}
	_, err := f.svc.Create(context.Background(), 1, service.CreateQRCodeInput{
		LongURL: "https://example.com", RenderCustomizations: big, GroupID: 7})
	assert.ErrorIs(t, err, service.ErrInvalidRenderCustomizations)

	for _, rc := range []model.JSONMap{
		{"color": "red"},
		{"background": "#12345"},
		{"color": 7},
		{"error_correction": "X"},
	} {
		_, err := f.svc.Create(context.Background(), 1, service.CreateQRCodeInput{
			LongURL: "https://example.com", RenderCustomizations: rc, GroupID: 7})
		assert.ErrorIs(t, err, service.ErrInvalidRenderCustomizations, "%v", rc)
	}

	_, err = f.svc.Create(context.Background(), 1, service.CreateQRCodeInput{GroupID: 7})
	assert.ErrorIs(t, err, service.ErrQRLinkRequired)
}

func TestQRService_Update_DestinationGoesThroughLinkUpdate(t *testing.T) {
	f := newQRSvc(t)
	link := model.Link{ID: 70, UserID: 1, GroupID: 7, ShortCode: "q1", OriginalURL: "https://old.example", Source: model.LinkSourceQR}
	f.qrs.EXPECT().GetByID(gomock.Any(), uint64(12)).Return(qrRow(12, 1, 7, link), nil)
	exp := time.Now().Add(time.Hour)
	updated := link
	updated.OriginalURL = "https://new.example"
	f.links.EXPECT().Update(gomock.Any(), uint64(1), uint64(70), service.UpdateLinkInput{
		OriginalURL: "https://new.example", ExpiresAt: &exp,
	}).Return(&updated, &link, nil)
	title := "Poster"
	var saved *model.QRCode
	f.qrs.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, qr *model.QRCode) error {
		saved = qr
		return nil
	})
	f.qrs.EXPECT().GetTagsFor(gomock.Any(), []uint64{12}).Return(map[uint64][]string{}, nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionQRUpdate, service.EntityQRCode, uint64(12),
		map[string]any{"short_code": "q1", "fields": []string{"long_url", "expires_at", "title"}})

	d, prev, err := f.svc.Update(context.Background(), 1, 12, service.UpdateQRCodeInput{
		Title: &title, LongURL: "https://new.example", ExpiresAt: &exp,
	})
	require.NoError(t, err)
	require.NotNil(t, prev)
	assert.Equal(t, "https://old.example", prev.OriginalURL)
	assert.Equal(t, "https://new.example", d.Link.OriginalURL)
	require.NotNil(t, saved.Title)
	assert.Equal(t, "Poster", *saved.Title)
	assert.NotNil(t, d.Tags)
}

func TestQRService_Update_OtherUser(t *testing.T) {
	f := newQRSvc(t)
	link := model.Link{ID: 70, UserID: 2, GroupID: 7, Source: model.LinkSourceQR}
	f.qrs.EXPECT().GetByID(gomock.Any(), uint64(12)).Return(qrRow(12, 2, 7, link), nil)
	_, _, err := f.svc.Update(context.Background(), 1, 12, service.UpdateQRCodeInput{LongURL: "https://x.example"})
	assert.ErrorIs(t, err, service.ErrQRCodeNotFound)
}

func TestQRService_Delete_QRSourceDeletesLink(t *testing.T) {
	f := newQRSvc(t)
	link := model.Link{ID: 80, UserID: 1, GroupID: 7, ShortCode: "q8", Source: model.LinkSourceQR,
		RedirectCount: 3, HasCustomBackHalf: true}
	f.qrs.EXPECT().GetByID(gomock.Any(), uint64(13)).Return(qrRow(13, 1, 7, link), nil)
	// Only the link is deleted; the QR row goes by ON DELETE CASCADE, so no
	// qrRepo.Delete expectation (a call fails the test).
	f.linkRepo.EXPECT().Delete(gomock.Any(), uint64(80)).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionQRDelete, service.EntityQRCode, uint64(13), gomock.Any())

	d, linkDeleted, err := f.svc.Delete(context.Background(), 1, 13)
	require.NoError(t, err)
	assert.True(t, linkDeleted)
	assert.Equal(t, "q8", d.Link.ShortCode)
}

func TestQRService_Delete_SharedLinkKept(t *testing.T) {
	f := newQRSvc(t)
	link := model.Link{ID: 81, UserID: 1, GroupID: 7, ShortCode: "s1", Source: model.LinkSourceLink}
	f.qrs.EXPECT().GetByID(gomock.Any(), uint64(14)).Return(qrRow(14, 1, 7, link), nil)
	f.qrs.EXPECT().Delete(gomock.Any(), uint64(14)).Return(nil)
	// No linkRepo.Delete expectation: the shared link stays.
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionQRDelete, service.EntityQRCode, uint64(14), gomock.Any())

	_, linkDeleted, err := f.svc.Delete(context.Background(), 1, 14)
	require.NoError(t, err)
	assert.False(t, linkDeleted)
}

func TestQRService_SetArchived_LeavesLinkAlone(t *testing.T) {
	f := newQRSvc(t)
	l1 := model.Link{ID: 90, UserID: 1, GroupID: 7, ShortCode: "a"}
	l2 := model.Link{ID: 91, UserID: 1, GroupID: 7, ShortCode: "b"}
	f.qrs.EXPECT().GetByIDs(gomock.Any(), []uint64{1, 2}).
		Return([]repository.QRCodeRow{*qrRow(1, 1, 7, l1), *qrRow(2, 1, 7, l2)}, nil)
	f.qrs.EXPECT().SetArchived(gomock.Any(), []uint64{1, 2}, true).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionQRHide, service.EntityQRCode, uint64(1), gomock.Any())
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionQRHide, service.EntityQRCode, uint64(2), gomock.Any())
	// No LinkService / LinkRepository expectations: any call fails the test.

	n, err := f.svc.SetArchived(context.Background(), 1, 7, []uint64{1, 2, 1}, true)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestQRService_SetArchived_ForeignIDs404(t *testing.T) {
	f := newQRSvc(t)
	f.qrs.EXPECT().GetByIDs(gomock.Any(), []uint64{1, 2}).
		Return([]repository.QRCodeRow{*qrRow(1, 1, 7, model.Link{ID: 90}), *qrRow(2, 1, 8, model.Link{ID: 91})}, nil)
	_, err := f.svc.SetArchived(context.Background(), 1, 7, []uint64{1, 2}, true)
	assert.ErrorIs(t, err, service.ErrQRCodeNotFound)
}

func TestQRService_BulkTags(t *testing.T) {
	f := newQRSvc(t)
	f.qrs.EXPECT().GetByIDs(gomock.Any(), []uint64{4, 5}).
		Return([]repository.QRCodeRow{*qrRow(4, 1, 7, model.Link{ID: 1}), *qrRow(5, 1, 7, model.Link{ID: 2})}, nil)
	f.qrs.EXPECT().AddTags(gomock.Any(), []uint64{4, 5}, []string{"new"}).Return(nil)
	f.qrs.EXPECT().RemoveTags(gomock.Any(), []uint64{4, 5}, []string{"old"}).Return(nil)
	require.NoError(t, f.svc.BulkTags(context.Background(), 7, []uint64{4, 5}, []string{" new "}, []string{"old"}))
}

func TestQRService_Export_Header(t *testing.T) {
	assert.Equal(t, []string{"Title", "Short Link", "Destination URL", "Date created", "Scans", "Status"}, service.QRExportHeader)

	f := newQRSvc(t)
	created := time.Date(2026, 10, 2, 3, 4, 5, 0, time.UTC)
	linkTitle := "=cmd"
	r1 := qrRow(1, 1, 7, model.Link{ID: 10, ShortCode: "abc", OriginalURL: "https://e.com", Title: &linkTitle})
	r1.CreatedAt = created
	r1.TotalScans = 4
	qrTitle := "Poster"
	r2 := qrRow(2, 1, 7, model.Link{ID: 11, ShortCode: "brand", OriginalURL: "https://f.com", DomainID: u64(5)})
	r2.Title = &qrTitle
	r2.Archived = true
	r2.CreatedAt = created
	f.qrs.EXPECT().ListByGroup(gomock.Any(), uint64(7), repository.QRFilter{}, gomock.Any(), 0).
		Return([]repository.QRCodeRow{*r1, *r2}, int64(2), nil)
	f.domains.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Domain{ID: 5, Domain: "go.brand.test"}, nil)

	rows, err := f.svc.Export(context.Background(), 7, repository.QRFilter{})
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"'=cmd", "https://sho.rt/abc", "https://e.com", "2026-10-02T03:04:05Z", "4", "Active"},
		{"Poster", "https://go.brand.test/brand", "https://f.com", "2026-10-02T03:04:05Z", "0", "Hidden"},
	}, rows)
}

func TestQRService_Delete_QRSourceLinkDeleteFails_KeepsQR(t *testing.T) {
	f := newQRSvc(t)
	link := model.Link{ID: 82, UserID: 1, GroupID: 7, Source: model.LinkSourceQR}
	f.qrs.EXPECT().GetByID(gomock.Any(), uint64(15)).Return(qrRow(15, 1, 7, link), nil)
	f.linkRepo.EXPECT().Delete(gomock.Any(), uint64(82)).Return(assert.AnError)

	_, linkDeleted, err := f.svc.Delete(context.Background(), 1, 15)
	assert.ErrorIs(t, err, assert.AnError)
	assert.False(t, linkDeleted)
}

func TestQRService_Update_LaterFailureReturnsPrevLink(t *testing.T) {
	f := newQRSvc(t)
	link := model.Link{ID: 71, UserID: 1, GroupID: 7, ShortCode: "q2", OriginalURL: "https://old.example", Source: model.LinkSourceQR}
	f.qrs.EXPECT().GetByID(gomock.Any(), uint64(16)).Return(qrRow(16, 1, 7, link), nil)
	updated := link
	updated.OriginalURL = "https://new.example"
	f.links.EXPECT().Update(gomock.Any(), uint64(1), uint64(71), service.UpdateLinkInput{OriginalURL: "https://new.example"}).
		Return(&updated, &link, nil)
	f.qrs.EXPECT().SetTags(gomock.Any(), uint64(16), []string{"x"}).Return(assert.AnError)

	d, prev, err := f.svc.Update(context.Background(), 1, 16, service.UpdateQRCodeInput{
		LongURL: "https://new.example", Tags: &[]string{"x"},
	})
	assert.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, d)
	require.NotNil(t, prev, "the link changed, so callers must still invalidate it")
	assert.Equal(t, "https://old.example", prev.OriginalURL)
}

func TestQRService_Update_RoutingRulesFailureReturnsPrevLink(t *testing.T) {
	f := newQRSvc(t)
	link := model.Link{ID: 72, UserID: 1, GroupID: 7, ShortCode: "q3", Source: model.LinkSourceQR}
	f.qrs.EXPECT().GetByID(gomock.Any(), uint64(17)).Return(qrRow(17, 1, 7, link), nil)
	f.links.EXPECT().SetRoutingRules(gomock.Any(), uint64(1), uint64(72), gomock.Any()).Return(nil, assert.AnError)

	_, prev, err := f.svc.Update(context.Background(), 1, 17, service.UpdateQRCodeInput{
		RoutingRules: &[]service.RoutingRuleInput{{LongURL: "https://r.example"}},
	})
	assert.ErrorIs(t, err, assert.AnError)
	require.NotNil(t, prev)
	assert.Equal(t, "q3", prev.ShortCode)
}
