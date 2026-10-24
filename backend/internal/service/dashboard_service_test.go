package service_test

import (
	"context"
	"errors"
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

type dashSvcFixture struct {
	svc      *service.DashboardServiceImpl
	repo     *mocks.MockDashboardRepository
	links    *mocks.MockLinkRepository
	qrs      *mocks.MockQRRepository
	activity *svcmocks.MockActivityService
}

func newDashSvc(t *testing.T) *dashSvcFixture {
	ctrl := gomock.NewController(t)
	f := &dashSvcFixture{
		repo:     mocks.NewMockDashboardRepository(ctrl),
		links:    mocks.NewMockLinkRepository(ctrl),
		qrs:      mocks.NewMockQRRepository(ctrl),
		activity: svcmocks.NewMockActivityService(ctrl),
	}
	f.svc = service.NewDashboardService(service.DashboardServiceDeps{
		Dashboards: f.repo,
		Links:      f.links,
		QRs:        f.qrs,
		Activity:   f.activity,
	})
	return f
}

func defaultLayoutMap() model.JSONMap {
	return model.JSONMap{"modules": []any{
		map[string]any{"key": "top_day", "viz": "scorecard"},
		map[string]any{"key": "top_location", "viz": "scorecard"},
		map[string]any{"key": "over_time", "viz": "line"},
		map[string]any{"key": "devices", "viz": "doughnut"},
		map[string]any{"key": "referrers", "viz": "bar"},
		map[string]any{"key": "locations", "viz": "table"},
		map[string]any{"key": "items", "viz": "table"},
	}}
}

func sampleDashboard(id, groupID uint64, name string, isDefault bool) *model.Dashboard {
	ts := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	return &model.Dashboard{
		ID: id, GroupID: groupID, CreatedBy: 1, Name: name, IsDefault: isDefault,
		DateRange: "last_7_days", Filter: model.JSONMap{"type": "all"}, Layout: defaultLayoutMap(),
		CreatedAt: ts, UpdatedAt: ts,
	}
}

func cloneLayout() service.DashboardLayout {
	return service.DashboardLayout{Modules: append([]service.DashboardModule(nil), service.DefaultLayout.Modules...)}
}

func TestDashboardService_List_CreatesDefault(t *testing.T) {
	f := newDashSvc(t)
	ctx := context.Background()
	def := sampleDashboard(3, 7, "Default", true)
	gomock.InOrder(
		f.repo.EXPECT().EnsureDefault(gomock.Any(), uint64(7), uint64(1), defaultLayoutMap()).Return(def, nil),
		f.repo.EXPECT().ListByGroup(gomock.Any(), uint64(7), "created").Return([]model.Dashboard{*def}, nil),
	)
	f.repo.EXPECT().CreatorNames(gomock.Any(), []uint64{1}).Return(map[uint64]string{1: "Ann"}, nil)

	views, err := f.svc.List(ctx, 1, 7, "")
	require.NoError(t, err)
	require.Len(t, views, 1)
	v := views[0]
	assert.Equal(t, "Default", v.Name)
	assert.True(t, v.IsDefault)
	assert.Equal(t, service.DashboardFilter{Type: "all"}, v.Filter)
	assert.Equal(t, service.DefaultLayout, v.Layout)
	assert.Equal(t, uint64(1), v.CreatedBy.ID)
	assert.Equal(t, "Ann", v.CreatedBy.Name)

	t.Run("unknown sort", func(t *testing.T) {
		f := newDashSvc(t)
		_, err := f.svc.List(ctx, 1, 7, "size")
		var ve *service.ValidationError
		require.ErrorAs(t, err, &ve)
	})
}

func TestDashboardService_Get_WrongGroup(t *testing.T) {
	f := newDashSvc(t)
	f.repo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleDashboard(5, 9, "Other", false), nil)
	_, err := f.svc.Get(context.Background(), 7, 5)
	assert.ErrorIs(t, err, repository.ErrDashboardNotFound)
}

func TestDashboardService_Get_MalformedStoredJSONFallsBack(t *testing.T) {
	f := newDashSvc(t)
	d := sampleDashboard(5, 7, "Broken", false)
	d.Filter = model.JSONMap{"type": 42}
	d.Layout = model.JSONMap{"modules": "nope"}
	f.repo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(d, nil)
	f.repo.EXPECT().CreatorNames(gomock.Any(), []uint64{1}).Return(map[uint64]string{}, nil)
	v, err := f.svc.Get(context.Background(), 7, 5)
	require.NoError(t, err)
	assert.Equal(t, service.DashboardFilter{Type: "all"}, v.Filter)
	assert.Equal(t, service.DefaultLayout, v.Layout)
}

func TestDashboardService_Create(t *testing.T) {
	f := newDashSvc(t)
	f.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, d *model.Dashboard) error {
		assert.Equal(t, uint64(7), d.GroupID)
		assert.Equal(t, uint64(1), d.CreatedBy)
		assert.Equal(t, "Campaign", d.Name)
		assert.False(t, d.IsDefault)
		assert.Equal(t, "last_7_days", d.DateRange)
		assert.Equal(t, model.JSONMap{"type": "all"}, d.Filter)
		assert.Equal(t, defaultLayoutMap(), d.Layout)
		d.ID = 11
		return nil
	})
	f.repo.EXPECT().CreatorNames(gomock.Any(), []uint64{1}).Return(map[uint64]string{1: "Ann"}, nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionDashboardCreate, service.EntityDashboard, uint64(11), gomock.Any())
	v, err := f.svc.Create(context.Background(), 1, 7, "  Campaign  ")
	require.NoError(t, err)
	assert.Equal(t, uint64(11), v.ID)

	for _, name := range []string{"   ", strings.Repeat("a", 256)} {
		_, err := newDashSvc(t).svc.Create(context.Background(), 1, 7, name)
		var ve *service.ValidationError
		require.ErrorAs(t, err, &ve)
	}
}

func TestDashboardService_Update_FilterValidation(t *testing.T) {
	ctx := context.Background()
	expectGet := func(f *dashSvcFixture) {
		f.repo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleDashboard(5, 7, "Mine", false), nil)
	}
	assertValidation := func(t *testing.T, err error) {
		t.Helper()
		var ve *service.ValidationError
		require.ErrorAs(t, err, &ve)
	}

	t.Run("more than 200 ids", func(t *testing.T) {
		f := newDashSvc(t)
		expectGet(f)
		ids := make([]uint64, 201)
		for i := range ids {
			ids[i] = uint64(i + 1)
		}
		_, err := f.svc.Update(ctx, 1, 7, 5, service.DashboardPatch{Filter: &service.DashboardFilter{Type: "links", IDs: ids}})
		assertValidation(t, err)
	})

	t.Run("no ids, duplicate ids, unknown type", func(t *testing.T) {
		for _, flt := range []service.DashboardFilter{
			{Type: "links"}, {Type: "links", IDs: []uint64{1, 1}}, {Type: "pages", IDs: []uint64{1}}, {Type: "all", IDs: []uint64{1}},
		} {
			f := newDashSvc(t)
			expectGet(f)
			_, err := f.svc.Update(ctx, 1, 7, 5, service.DashboardPatch{Filter: &flt})
			assertValidation(t, err)
		}
	})

	t.Run("foreign link id", func(t *testing.T) {
		f := newDashSvc(t)
		expectGet(f)
		f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{1, 2}).Return([]model.Link{
			{ID: 1, GroupID: 7, Source: model.LinkSourceLink},
			{ID: 2, GroupID: 8, Source: model.LinkSourceLink},
		}, nil)
		_, err := f.svc.Update(ctx, 1, 7, 5, service.DashboardPatch{Filter: &service.DashboardFilter{Type: "links", IDs: []uint64{1, 2}}})
		assertValidation(t, err)
	})

	t.Run("missing link id", func(t *testing.T) {
		f := newDashSvc(t)
		expectGet(f)
		f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{1, 2}).Return([]model.Link{{ID: 1, GroupID: 7, Source: model.LinkSourceLink}}, nil)
		_, err := f.svc.Update(ctx, 1, 7, 5, service.DashboardPatch{Filter: &service.DashboardFilter{Type: "links", IDs: []uint64{1, 2}}})
		assertValidation(t, err)
	})

	t.Run("qr link under links", func(t *testing.T) {
		f := newDashSvc(t)
		expectGet(f)
		f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{3}).Return([]model.Link{{ID: 3, GroupID: 7, Source: model.LinkSourceQR}}, nil)
		_, err := f.svc.Update(ctx, 1, 7, 5, service.DashboardPatch{Filter: &service.DashboardFilter{Type: "links", IDs: []uint64{3}}})
		assertValidation(t, err)
	})

	t.Run("foreign qr id", func(t *testing.T) {
		f := newDashSvc(t)
		expectGet(f)
		qr := repository.QRCodeRow{}
		qr.ID, qr.GroupID = 4, 9
		f.qrs.EXPECT().GetByIDs(gomock.Any(), []uint64{4}).Return([]repository.QRCodeRow{qr}, nil)
		_, err := f.svc.Update(ctx, 1, 7, 5, service.DashboardPatch{Filter: &service.DashboardFilter{Type: "qr", IDs: []uint64{4}}})
		assertValidation(t, err)
	})

	t.Run("bad date range", func(t *testing.T) {
		f := newDashSvc(t)
		expectGet(f)
		dr := "last_year"
		_, err := f.svc.Update(ctx, 1, 7, 5, service.DashboardPatch{DateRange: &dr})
		assertValidation(t, err)
	})

	t.Run("valid", func(t *testing.T) {
		f := newDashSvc(t)
		expectGet(f)
		f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{1, 2}).Return([]model.Link{
			{ID: 1, GroupID: 7, Source: model.LinkSourceLink},
			{ID: 2, GroupID: 7, Source: model.LinkSourceLink},
		}, nil)
		f.repo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, d *model.Dashboard) error {
			assert.Equal(t, uint64(5), d.ID)
			assert.Equal(t, "Renamed", d.Name)
			assert.Equal(t, "last_30_days", d.DateRange)
			assert.Equal(t, model.JSONMap{"type": "links", "ids": []any{float64(1), float64(2)}}, d.Filter)
			assert.Equal(t, defaultLayoutMap(), d.Layout) // untouched
			return nil
		})
		updated := sampleDashboard(5, 7, "Renamed", false)
		updated.DateRange = "last_30_days"
		updated.Filter = model.JSONMap{"type": "links", "ids": []any{float64(1), float64(2)}}
		f.repo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(updated, nil)
		f.repo.EXPECT().CreatorNames(gomock.Any(), []uint64{1}).Return(map[uint64]string{1: "Ann"}, nil)
		f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionDashboardUpdate, service.EntityDashboard, uint64(5), gomock.Any())

		name, dr := " Renamed ", "last_30_days"
		v, err := f.svc.Update(ctx, 1, 7, 5, service.DashboardPatch{
			Name: &name, DateRange: &dr, Filter: &service.DashboardFilter{Type: "links", IDs: []uint64{1, 2}},
		})
		require.NoError(t, err)
		assert.Equal(t, service.DashboardFilter{Type: "links", IDs: []uint64{1, 2}}, v.Filter)
		assert.Equal(t, "last_30_days", v.DateRange)
	})

	t.Run("wrong group", func(t *testing.T) {
		f := newDashSvc(t)
		f.repo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleDashboard(5, 9, "Other", false), nil)
		dr := "today"
		_, err := f.svc.Update(ctx, 1, 7, 5, service.DashboardPatch{DateRange: &dr})
		assert.ErrorIs(t, err, repository.ErrDashboardNotFound)
	})
}

func TestDashboardService_Update_LayoutValidation(t *testing.T) {
	ctx := context.Background()
	missing := cloneLayout()
	missing.Modules = missing.Modules[:6]
	dup := cloneLayout()
	dup.Modules[6] = service.DashboardModule{Key: "locations", Viz: "bar"}
	badViz := cloneLayout()
	badViz.Modules[0].Viz = "line"
	unknown := cloneLayout()
	unknown.Modules[6] = service.DashboardModule{Key: "pages", Viz: "table"}

	for name, l := range map[string]service.DashboardLayout{
		"missing key": missing, "duplicate key": dup, "bad viz": badViz, "unknown key": unknown,
	} {
		t.Run(name, func(t *testing.T) {
			f := newDashSvc(t)
			f.repo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleDashboard(5, 7, "Mine", false), nil)
			_, err := f.svc.Update(ctx, 1, 7, 5, service.DashboardPatch{Layout: &l})
			var ve *service.ValidationError
			require.ErrorAs(t, err, &ve)
		})
	}

	t.Run("reordered with other viz is valid", func(t *testing.T) {
		l := cloneLayout()
		l.Modules[0], l.Modules[6] = l.Modules[6], l.Modules[0]
		l.Modules[0].Viz = "bar" // items
		require.NoError(t, service.ValidateLayout(l))
	})
}

func TestDashboardService_Delete_DefaultConflict(t *testing.T) {
	ctx := context.Background()
	f := newDashSvc(t)
	f.repo.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(sampleDashboard(3, 7, "Default", true), nil)
	err := f.svc.Delete(ctx, 1, 7, 3)
	assert.ErrorIs(t, err, service.ErrDashboardDefault)

	t.Run("wrong group", func(t *testing.T) {
		f := newDashSvc(t)
		f.repo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleDashboard(5, 9, "Other", false), nil)
		assert.ErrorIs(t, f.svc.Delete(ctx, 1, 7, 5), repository.ErrDashboardNotFound)
	})

	t.Run("deletes and logs", func(t *testing.T) {
		f := newDashSvc(t)
		f.repo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleDashboard(5, 7, "Mine", false), nil)
		f.repo.EXPECT().Delete(gomock.Any(), uint64(5)).Return(nil)
		f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionDashboardDelete, service.EntityDashboard, uint64(5), gomock.Any())
		require.NoError(t, f.svc.Delete(ctx, 1, 7, 5))
	})

	t.Run("repo error", func(t *testing.T) {
		f := newDashSvc(t)
		f.repo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleDashboard(5, 7, "Mine", false), nil)
		f.repo.EXPECT().Delete(gomock.Any(), uint64(5)).Return(errors.New("boom"))
		require.Error(t, f.svc.Delete(ctx, 1, 7, 5))
	})
}

func TestDashboardService_Duplicate_NameAndFlags(t *testing.T) {
	ctx := context.Background()
	src := sampleDashboard(3, 7, "Default", true)
	src.DateRange = "last_90_days"
	src.Filter = model.JSONMap{"type": "qr", "ids": []any{float64(4)}}
	f := newDashSvc(t)
	f.repo.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(src, nil)
	f.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, d *model.Dashboard) error {
		assert.Equal(t, "Copy of Default", d.Name)
		assert.False(t, d.IsDefault)
		assert.Equal(t, uint64(7), d.GroupID)
		assert.Equal(t, uint64(2), d.CreatedBy)
		assert.Equal(t, "last_90_days", d.DateRange)
		assert.Equal(t, model.JSONMap{"type": "qr", "ids": []any{float64(4)}}, d.Filter)
		assert.Equal(t, defaultLayoutMap(), d.Layout)
		d.ID = 12
		return nil
	})
	f.repo.EXPECT().CreatorNames(gomock.Any(), []uint64{2}).Return(map[uint64]string{2: "Bob"}, nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(2), uint64(7), service.ActionDashboardDuplicate, service.EntityDashboard, uint64(12), gomock.Any())
	v, err := f.svc.Duplicate(ctx, 2, 7, 3)
	require.NoError(t, err)
	assert.Equal(t, "Copy of Default", v.Name)
	assert.False(t, v.IsDefault)
	assert.Equal(t, service.DashboardFilter{Type: "qr", IDs: []uint64{4}}, v.Filter)

	t.Run("long name truncated", func(t *testing.T) {
		f := newDashSvc(t)
		f.repo.EXPECT().GetByID(gomock.Any(), uint64(4)).Return(sampleDashboard(4, 7, strings.Repeat("é", 255), false), nil)
		f.repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, d *model.Dashboard) error {
			assert.Equal(t, 255, len([]rune(d.Name)))
			assert.True(t, strings.HasPrefix(d.Name, "Copy of é"))
			d.ID = 13
			return nil
		})
		f.repo.EXPECT().CreatorNames(gomock.Any(), gomock.Any()).Return(map[uint64]string{}, nil)
		f.activity.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		_, err := f.svc.Duplicate(ctx, 2, 7, 4)
		require.NoError(t, err)
	})

	t.Run("wrong group", func(t *testing.T) {
		f := newDashSvc(t)
		f.repo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleDashboard(5, 9, "Other", false), nil)
		_, err := f.svc.Duplicate(ctx, 2, 7, 5)
		assert.ErrorIs(t, err, repository.ErrDashboardNotFound)
	})
}
