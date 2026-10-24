package service_test

import (
	"context"
	"encoding/json"
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

const pageBaseURL = "http://localhost:8080"

type pageSvcFixture struct {
	svc      *service.PageServiceImpl
	pages    *mocks.MockPageRepository
	views    *mocks.MockPageViewRepository
	links    *mocks.MockLinkRepository
	linkSvc  *svcmocks.MockLinkService
	domains  *mocks.MockDomainRepository
	activity *svcmocks.MockActivityService
	now      time.Time
}

func newPageSvc(t *testing.T) *pageSvcFixture {
	ctrl := gomock.NewController(t)
	f := &pageSvcFixture{
		pages:    mocks.NewMockPageRepository(ctrl),
		views:    mocks.NewMockPageViewRepository(ctrl),
		links:    mocks.NewMockLinkRepository(ctrl),
		linkSvc:  svcmocks.NewMockLinkService(ctrl),
		domains:  mocks.NewMockDomainRepository(ctrl),
		activity: svcmocks.NewMockActivityService(ctrl),
		now:      time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC),
	}
	f.svc = service.NewPageService(service.PageServiceDeps{
		Pages:    f.pages,
		Views:    f.views,
		Links:    f.links,
		LinkSvc:  f.linkSvc,
		Domains:  f.domains,
		Activity: f.activity,
		BaseURL:  pageBaseURL + "/",
	})
	f.svc.SetClock(func() time.Time { return f.now })
	return f
}

func samplePage(id, groupID uint64, slug string) *model.Page {
	ts := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	return &model.Page{
		ID: id, UserID: 1, GroupID: groupID, Slug: slug, Title: "T " + slug,
		Design: service.DefaultDesign("classic"), Status: model.PageStatusDraft,
		CreatedAt: ts, UpdatedAt: ts,
	}
}

func strp(s string) *string { return &s }

func assertValidation(t *testing.T, err error) {
	t.Helper()
	var ve *service.ValidationError
	assert.True(t, errors.As(err, &ve), "want ValidationError, got %v", err)
}

func TestPageService_Create_SlugRules(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid and reserved", func(t *testing.T) {
		f := newPageSvc(t)
		for _, slug := range []string{"", "ab", "-abc", "abc-", "a_bc", "ab c", "é-abc", strings.Repeat("a", 41)} {
			_, err := f.svc.Create(ctx, 1, 7, service.CreatePageInput{Slug: slug})
			assertValidation(t, err)
		}
		for _, slug := range []string{"api", "admin", "dashboard", "login", "register", "static", "assets", "health", "Admin"} {
			_, err := f.svc.Create(ctx, 1, 7, service.CreatePageInput{Slug: slug})
			assertValidation(t, err)
		}
		_, err := f.svc.Create(ctx, 1, 7, service.CreatePageInput{Slug: "jose", Template: "neon"})
		assertValidation(t, err)
	})

	t.Run("taken", func(t *testing.T) {
		f := newPageSvc(t)
		f.pages.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repository.ErrPageSlugTaken)
		_, err := f.svc.Create(ctx, 1, 7, service.CreatePageInput{Slug: "jose"})
		assert.ErrorIs(t, err, repository.ErrPageSlugTaken)
	})

	t.Run("uppercase lowercased, scratch seeds nothing", func(t *testing.T) {
		f := newPageSvc(t)
		var created *model.Page
		f.pages.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *model.Page) error {
			created = p
			p.ID = 11
			p.Status = model.PageStatusDraft
			p.CreatedAt, p.UpdatedAt = f.now, f.now
			return nil
		})
		f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(11)).Return([]model.PageBlock{}, nil)
		f.views.EXPECT().CountAllTime(gomock.Any(), uint64(11)).Return(int64(0), nil)
		f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPageCreate, service.EntityPage, uint64(11), gomock.Any())

		v, err := f.svc.Create(ctx, 1, 7, service.CreatePageInput{Slug: "  Jose-1 ", Title: "Jose"})
		require.NoError(t, err)
		assert.Equal(t, "jose-1", created.Slug)
		assert.Equal(t, uint64(7), created.GroupID)
		assert.Equal(t, uint64(1), created.UserID)
		assert.Equal(t, service.DefaultDesign("scratch"), created.Design)
		assert.Equal(t, "jose-1", v.Slug)
		assert.Equal(t, pageBaseURL+"/m/jose-1", v.URL)
		assert.True(t, v.HasUnpublishedChanges)
		assert.NotNil(t, v.Blocks)
		assert.Empty(t, v.Blocks)
	})

	t.Run("template seeds a text block", func(t *testing.T) {
		f := newPageSvc(t)
		var created *model.Page
		f.pages.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *model.Page) error {
			created = p
			p.ID = 12
			p.Status = model.PageStatusDraft
			return nil
		})
		var seeded *model.PageBlock
		f.pages.EXPECT().CreateBlock(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, b *model.PageBlock) error {
			seeded = b
			b.ID = 30
			return nil
		})
		f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(12)).DoAndReturn(func(context.Context, uint64) ([]model.PageBlock, error) {
			return []model.PageBlock{*seeded}, nil
		})
		f.views.EXPECT().CountAllTime(gomock.Any(), uint64(12)).Return(int64(0), nil)
		f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPageCreate, service.EntityPage, uint64(12), gomock.Any())

		v, err := f.svc.Create(ctx, 1, 7, service.CreatePageInput{Slug: "bolder", Template: "bold"})
		require.NoError(t, err)
		assert.Equal(t, service.DefaultDesign("bold"), created.Design)
		assert.Equal(t, "bold", created.Design["theme"])
		require.NotNil(t, seeded)
		assert.Equal(t, uint64(12), seeded.PageID)
		assert.Equal(t, model.PageBlockText, seeded.Type)
		assert.True(t, seeded.IsActive)
		assert.Equal(t, model.JSONMap{"heading": "Welcome", "body": "This is your new page"}, seeded.Content)
		require.Len(t, v.Blocks, 1)
		assert.Nil(t, v.Blocks[0].Link)
	})
}

func TestPageService_CheckSlug(t *testing.T) {
	f := newPageSvc(t)
	ctx := context.Background()
	f.pages.EXPECT().SlugExists(gomock.Any(), "taken").Return(true, nil)
	f.pages.EXPECT().SlugExists(gomock.Any(), "free-one").Return(false, nil)

	for _, tc := range []struct{ in, slug, reason string }{
		{"x", "x", "invalid"},
		{"API", "api", "reserved"},
		{"m", "m", "invalid"}, // too short before reserved
		{"Taken", "taken", "taken"},
		{"free-one", "free-one", ""},
	} {
		got, err := f.svc.CheckSlug(ctx, tc.in)
		require.NoError(t, err)
		assert.Equal(t, tc.slug, got.Slug, tc.in)
		assert.Equal(t, tc.reason, got.Reason, tc.in)
		assert.Equal(t, tc.reason == "", got.Available, tc.in)
	}
}

func TestPageService_Get_WrongGroup(t *testing.T) {
	f := newPageSvc(t)
	ctx := context.Background()
	f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 8, "other"), nil).AnyTimes()

	_, err := f.svc.Get(ctx, 7, 5)
	assert.ErrorIs(t, err, repository.ErrPageNotFound)
	_, err = f.svc.Update(ctx, 1, 7, 5, service.UpdatePageInput{Title: strp("x")})
	assert.ErrorIs(t, err, repository.ErrPageNotFound)
	_, err = f.svc.SetArchived(ctx, 1, 7, 5, true)
	assert.ErrorIs(t, err, repository.ErrPageNotFound)
	assert.ErrorIs(t, f.svc.Delete(ctx, 1, 7, 5), repository.ErrPageNotFound)
	_, err = f.svc.Publish(ctx, 1, 7, 5)
	assert.ErrorIs(t, err, repository.ErrPageNotFound)
	_, err = f.svc.AddBlock(ctx, 1, 7, 5, service.BlockInput{Type: "text", Content: map[string]any{"heading": "h"}})
	assert.ErrorIs(t, err, repository.ErrPageNotFound)
	_, err = f.svc.Reorder(ctx, 1, 7, 5, []uint64{1})
	assert.ErrorIs(t, err, repository.ErrPageNotFound)

	f.pages.EXPECT().GetByID(gomock.Any(), uint64(6)).Return(nil, repository.ErrPageNotFound)
	_, err = f.svc.Get(ctx, 7, 6)
	assert.ErrorIs(t, err, repository.ErrPageNotFound)
}

func TestPageService_Get_WithBlocksAndLinks(t *testing.T) {
	f := newPageSvc(t)
	ctx := context.Background()
	p := samplePage(5, 7, "jose")
	f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(p, nil)
	f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(5)).Return([]model.PageBlock{
		{ID: 1, PageID: 5, Type: "link", Position: 0, Content: model.JSONMap{"title": "Menu"}, IsActive: true, LinkID: u64(9)},
		{ID: 2, PageID: 5, Type: "link", Position: 1, Content: model.JSONMap{"title": "Shop"}, IsActive: true, LinkID: u64(10)},
		{ID: 3, PageID: 5, Type: "text", Position: 2, Content: model.JSONMap{"heading": "Hi"}, IsActive: true},
	}, nil)
	f.links.EXPECT().GetByIDs(gomock.Any(), gomock.InAnyOrder([]uint64{9, 10})).Return([]model.Link{
		{ID: 9, GroupID: 7, ShortCode: "abc", OriginalURL: "https://menu.example", Title: strp("Menu link")},
		{ID: 10, GroupID: 7, ShortCode: "shop", OriginalURL: "https://shop.example", DomainID: u64(3)},
	}, nil)
	f.domains.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(&model.Domain{ID: 3, Domain: "go.example"}, nil)
	f.views.EXPECT().CountAllTime(gomock.Any(), uint64(5)).Return(int64(12), nil)

	v, err := f.svc.Get(ctx, 7, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(12), v.Views)
	assert.Equal(t, pageBaseURL+"/m/jose", v.URL)
	require.Len(t, v.Blocks, 3)
	require.NotNil(t, v.Blocks[0].Link)
	assert.Equal(t, service.BlockLink{ID: 9, Title: "Menu link", ShortURL: pageBaseURL + "/abc", OriginalURL: "https://menu.example"}, *v.Blocks[0].Link)
	assert.Equal(t, service.BlockLink{ID: 10, Title: "shop", ShortURL: "http://go.example/shop", OriginalURL: "https://shop.example"}, *v.Blocks[1].Link)
	assert.Nil(t, v.Blocks[2].Link)
}

func TestPageService_AddLinkBlock_ForeignLink(t *testing.T) {
	ctx := context.Background()
	content := map[string]any{"title": "Menu"}
	for name, link := range map[string]model.Link{
		"other group": {ID: 9, GroupID: 8, Source: model.LinkSourceLink},
		"qr-owned":    {ID: 9, GroupID: 7, Source: model.LinkSourceQR},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPageSvc(t)
			f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil)
			f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{9}).Return([]model.Link{link}, nil)
			_, err := f.svc.AddBlock(ctx, 1, 7, 5, service.BlockInput{Type: "link", Content: content, LinkID: u64(9)})
			assertValidation(t, err)
		})
	}
	t.Run("missing link", func(t *testing.T) {
		f := newPageSvc(t)
		f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil)
		f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{9}).Return([]model.Link{}, nil)
		_, err := f.svc.AddBlock(ctx, 1, 7, 5, service.BlockInput{Type: "link", Content: content, LinkID: u64(9)})
		assertValidation(t, err)
	})
	t.Run("no link and both", func(t *testing.T) {
		f := newPageSvc(t)
		f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil).Times(3)
		_, err := f.svc.AddBlock(ctx, 1, 7, 5, service.BlockInput{Type: "link", Content: content})
		assertValidation(t, err)
		_, err = f.svc.AddBlock(ctx, 1, 7, 5, service.BlockInput{Type: "link", Content: content, LinkID: u64(9),
			NewLink: &service.NewLinkInput{OriginalURL: "https://x.example"}})
		assertValidation(t, err)
		_, err = f.svc.AddBlock(ctx, 1, 7, 5, service.BlockInput{Type: "text", Content: map[string]any{"heading": "h"}, LinkID: u64(9)})
		assertValidation(t, err)
	})
	t.Run("same group ok", func(t *testing.T) {
		f := newPageSvc(t)
		f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil)
		f.links.EXPECT().GetByIDs(gomock.Any(), []uint64{9}).Return([]model.Link{
			{ID: 9, GroupID: 7, Source: model.LinkSourceLink, ShortCode: "abc", OriginalURL: "https://a.example"}}, nil)
		f.pages.EXPECT().CreateBlock(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, b *model.PageBlock) error {
			assert.Equal(t, uint64(9), *b.LinkID)
			assert.Equal(t, model.JSONMap{"title": "Menu", "description": "", "thumbnail_url": ""}, b.Content)
			b.ID = 40
			return nil
		})
		f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPageUpdate, service.EntityPage, uint64(5), gomock.Any())
		bv, err := f.svc.AddBlock(ctx, 1, 7, 5, service.BlockInput{Type: "link", Content: content, LinkID: u64(9)})
		require.NoError(t, err)
		assert.Equal(t, uint64(40), bv.ID)
		require.NotNil(t, bv.Link)
		assert.Equal(t, pageBaseURL+"/abc", bv.Link.ShortURL)
		assert.Equal(t, "abc", bv.Link.Title)
	})
}

func TestPageService_AddLinkBlock_NewLink(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct{ newTitle, want string }{
		"title from block":    {"", "Menu"},
		"title from new_link": {"Our menu", "Our menu"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPageSvc(t)
			f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil)
			f.linkSvc.EXPECT().Create(gomock.Any(), uint64(1), service.CreateLinkInput{
				OriginalURL: "https://menu.example", Title: tc.want, GroupID: 7, Source: model.LinkSourceLink,
			}).Return(&model.Link{ID: 77, GroupID: 7, ShortCode: "new", OriginalURL: "https://menu.example", Title: strp(tc.want), Source: "link"}, nil)
			f.pages.EXPECT().CreateBlock(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, b *model.PageBlock) error {
				require.NotNil(t, b.LinkID)
				assert.Equal(t, uint64(77), *b.LinkID)
				b.ID = 41
				return nil
			})
			f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPageUpdate, service.EntityPage, uint64(5), gomock.Any())
			bv, err := f.svc.AddBlock(ctx, 1, 7, 5, service.BlockInput{Type: "link", Content: map[string]any{"title": "Menu"},
				NewLink: &service.NewLinkInput{OriginalURL: " https://menu.example ", Title: tc.newTitle}})
			require.NoError(t, err)
			require.NotNil(t, bv.Link)
			assert.Equal(t, uint64(77), bv.Link.ID)
			assert.Equal(t, tc.want, bv.Link.Title)
		})
	}
	t.Run("invalid content creates no link", func(t *testing.T) {
		f := newPageSvc(t)
		f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil)
		_, err := f.svc.AddBlock(ctx, 1, 7, 5, service.BlockInput{Type: "link", Content: map[string]any{"title": ""},
			NewLink: &service.NewLinkInput{OriginalURL: "https://menu.example"}})
		assertValidation(t, err)
	})
}

func TestPageService_Block_ForeignBlock404(t *testing.T) {
	f := newPageSvc(t)
	ctx := context.Background()
	f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil).AnyTimes()
	f.pages.EXPECT().GetBlock(gomock.Any(), uint64(50)).Return(&model.PageBlock{ID: 50, PageID: 6, Type: "text"}, nil).AnyTimes()
	f.pages.EXPECT().GetBlock(gomock.Any(), uint64(51)).Return(nil, repository.ErrBlockNotFound).AnyTimes()

	for _, bid := range []uint64{50, 51} {
		_, err := f.svc.UpdateBlock(ctx, 1, 7, 5, bid, service.BlockPatch{IsActive: boolp(false)})
		assert.ErrorIs(t, err, repository.ErrBlockNotFound)
		assert.ErrorIs(t, f.svc.DeleteBlock(ctx, 1, 7, 5, bid), repository.ErrBlockNotFound)
	}
}

func TestPageService_UpdateBlock(t *testing.T) {
	f := newPageSvc(t)
	ctx := context.Background()
	f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil).Times(2)
	blk := &model.PageBlock{ID: 50, PageID: 5, Type: "text", Content: model.JSONMap{"heading": "a", "body": ""}, IsActive: true}
	f.pages.EXPECT().GetBlock(gomock.Any(), uint64(50)).Return(blk, nil).Times(2)

	_, err := f.svc.UpdateBlock(ctx, 1, 7, 5, 50, service.BlockPatch{Content: map[string]any{"heading": "", "body": ""}})
	assertValidation(t, err)

	f.pages.EXPECT().UpdateBlock(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, b *model.PageBlock) error {
		assert.Equal(t, model.JSONMap{"heading": "b", "body": ""}, b.Content)
		assert.True(t, b.IsPinned)
		assert.True(t, b.IsActive)
		return nil
	})
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPageUpdate, service.EntityPage, uint64(5), gomock.Any())
	bv, err := f.svc.UpdateBlock(ctx, 1, 7, 5, 50, service.BlockPatch{Content: map[string]any{"heading": "b"}, IsPinned: boolp(true)})
	require.NoError(t, err)
	assert.True(t, bv.IsPinned)
}

func TestPageService_Reorder_Validation(t *testing.T) {
	ctx := context.Background()
	blocks := []model.PageBlock{{ID: 1, PageID: 5}, {ID: 2, PageID: 5}, {ID: 3, PageID: 5}}
	for name, ids := range map[string][]uint64{
		"missing":   {3, 1},
		"foreign":   {3, 1, 99},
		"duplicate": {3, 1, 1},
		"extra":     {3, 1, 2, 99},
		"empty":     {},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPageSvc(t)
			f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil)
			f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(5)).Return(blocks, nil)
			_, err := f.svc.Reorder(ctx, 1, 7, 5, ids)
			assertValidation(t, err)
		})
	}
	t.Run("repo race maps to validation", func(t *testing.T) {
		f := newPageSvc(t)
		f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil)
		f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(5)).Return(blocks, nil)
		f.pages.EXPECT().ReorderBlocks(gomock.Any(), uint64(5), []uint64{3, 1, 2}).Return(repository.ErrInvalidBlockOrder)
		_, err := f.svc.Reorder(ctx, 1, 7, 5, []uint64{3, 1, 2})
		assertValidation(t, err)
	})
	t.Run("ok", func(t *testing.T) {
		f := newPageSvc(t)
		f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(samplePage(5, 7, "jose"), nil)
		gomock.InOrder(
			f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(5)).Return(blocks, nil),
			f.pages.EXPECT().ReorderBlocks(gomock.Any(), uint64(5), []uint64{3, 1, 2}).Return(nil),
			f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(5)).Return([]model.PageBlock{
				{ID: 3, PageID: 5, Position: 0}, {ID: 1, PageID: 5, Position: 1}, {ID: 2, PageID: 5, Position: 2}}, nil),
		)
		f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPageUpdate, service.EntityPage, uint64(5), gomock.Any())
		out, err := f.svc.Reorder(ctx, 1, 7, 5, []uint64{3, 1, 2})
		require.NoError(t, err)
		require.Len(t, out, 3)
		assert.Equal(t, uint64(3), out[0].ID)
	})
}

func TestPageService_Publish_Snapshot(t *testing.T) {
	f := newPageSvc(t)
	ctx := context.Background()
	p := samplePage(5, 7, "jose")
	p.Description = "Bio"
	p.AvatarURL = "/uploads/a.png"
	p.UpdatedAt = time.Date(2026, 10, 25, 11, 0, 0, 0, time.UTC)
	published := *p
	published.Status = model.PageStatusPublished
	at := f.now
	published.PublishedAt = &at

	gomock.InOrder(
		f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(p, nil),
		f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(5)).Return([]model.PageBlock{
			{ID: 1, PageID: 5, Type: "text", Position: 0, Content: model.JSONMap{"heading": "Hi", "body": ""}, IsActive: true},
			{ID: 2, PageID: 5, Type: "link", Position: 1, Content: model.JSONMap{"title": "Menu"}, IsActive: true, LinkID: u64(9)},
			{ID: 3, PageID: 5, Type: "text", Position: 2, Content: model.JSONMap{"heading": "Off"}, IsActive: false},
			{ID: 4, PageID: 5, Type: "link", Position: 3, Content: model.JSONMap{"title": "Pinned"}, IsActive: true, IsPinned: true, LinkID: u64(10)},
		}, nil),
	)
	f.links.EXPECT().GetByIDs(gomock.Any(), gomock.InAnyOrder([]uint64{9, 10})).Return([]model.Link{
		{ID: 9, GroupID: 7, ShortCode: "abc"},
		{ID: 10, GroupID: 7, ShortCode: "def"},
	}, nil).AnyTimes()
	var snap model.JSONMap
	f.pages.EXPECT().Publish(gomock.Any(), uint64(5), gomock.Any(), at).DoAndReturn(
		func(_ context.Context, _ uint64, c model.JSONMap, _ time.Time) error { snap = c; return nil })
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPagePublish, service.EntityPage, uint64(5), gomock.Any())
	// The returned view is reloaded.
	f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&published, nil)
	f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(5)).Return([]model.PageBlock{}, nil)
	f.views.EXPECT().CountAllTime(gomock.Any(), uint64(5)).Return(int64(0), nil)

	v, err := f.svc.Publish(ctx, 1, 7, 5)
	require.NoError(t, err)
	assert.Equal(t, model.PageStatusPublished, v.Status)
	assert.False(t, v.HasUnpublishedChanges)

	assert.Equal(t, "jose", snap["slug"])
	assert.Equal(t, "T jose", snap["title"])
	assert.Equal(t, "Bio", snap["description"])
	assert.Equal(t, "/uploads/a.png", snap["avatar_url"])
	assert.Equal(t, "2026-10-25T12:00:00Z", snap["published_at"])
	assert.Equal(t, map[string]any(p.Design), snap["design"])
	for _, k := range []string{"id", "user_id", "group_id"} {
		assert.NotContains(t, snap, k)
	}
	blocks, ok := snap["blocks"].([]any)
	require.True(t, ok)
	require.Len(t, blocks, 3)
	ids := []any{}
	for _, raw := range blocks {
		b := raw.(map[string]any)
		ids = append(ids, b["id"])
		for _, k := range []string{"link_id", "position", "is_active"} {
			assert.NotContains(t, b, k)
		}
	}
	assert.Equal(t, []any{uint64(4), uint64(1), uint64(2)}, ids)
	pinned := blocks[0].(map[string]any)
	assert.Equal(t, true, pinned["is_pinned"])
	assert.Equal(t, pageBaseURL+"/def?r=page", pinned["short_url"])
	assert.Equal(t, "link", pinned["type"])
	assert.Equal(t, map[string]any{"title": "Pinned"}, pinned["content"])
	assert.NotContains(t, blocks[1].(map[string]any), "short_url")
	assert.Equal(t, pageBaseURL+"/abc?r=page", blocks[2].(map[string]any)["short_url"])
}

func TestPageService_Publish_EmptyAndSkew(t *testing.T) {
	f := newPageSvc(t)
	ctx := context.Background()
	p := samplePage(5, 7, "jose")
	// The DB clock ran ahead of the app clock: published_at must not be
	// earlier than the last edit, or the page would look dirty right away.
	p.UpdatedAt = f.now.Add(2 * time.Second)
	f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(p, nil).Times(2)
	f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(5)).Return([]model.PageBlock{}, nil).Times(2)
	f.pages.EXPECT().Publish(gomock.Any(), uint64(5), gomock.Any(), f.now.Add(2*time.Second)).DoAndReturn(
		func(_ context.Context, _ uint64, c model.JSONMap, _ time.Time) error {
			assert.Equal(t, []any{}, c["blocks"])
			return nil
		})
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPagePublish, service.EntityPage, uint64(5), gomock.Any())
	f.views.EXPECT().CountAllTime(gomock.Any(), uint64(5)).Return(int64(0), nil)
	_, err := f.svc.Publish(ctx, 1, 7, 5)
	require.NoError(t, err)
}

func TestPageService_ShortURLWithQuery(t *testing.T) {
	assert.Equal(t, "http://x/a?r=page", service.PageShortURL("http://x/a"))
	assert.Equal(t, "http://x/a?b=1&r=page", service.PageShortURL("http://x/a?b=1"))
}

func TestPageService_HasUnpublishedChanges(t *testing.T) {
	pub := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	later := pub.Add(time.Second)
	earlier := pub.Add(-time.Second)
	// Same instant in another zone: equal, not newer.
	pubOtherZone := pub.In(time.FixedZone("X", 3*3600))
	for name, tc := range map[string]struct {
		status    string
		published *time.Time
		updated   time.Time
		maxBlock  *time.Time
		want      bool
	}{
		"draft":                {model.PageStatusDraft, nil, earlier, nil, true},
		"draft even if older":  {model.PageStatusDraft, &pub, earlier, nil, true},
		"clean":                {model.PageStatusPublished, &pub, earlier, &earlier, false},
		"equal is clean":       {model.PageStatusPublished, &pub, pubOtherZone, &pub, false},
		"page edited":          {model.PageStatusPublished, &pub, later, nil, true},
		"block edited":         {model.PageStatusPublished, &pub, earlier, &later, true},
		"no blocks, clean":     {model.PageStatusPublished, &pub, pub, nil, false},
		"published without at": {model.PageStatusPublished, nil, pub, nil, true},
	} {
		got := service.HasUnpublishedChanges(&model.Page{Status: tc.status, PublishedAt: tc.published, UpdatedAt: tc.updated}, tc.maxBlock)
		assert.Equal(t, tc.want, got, name)
	}

	// Get uses the blocks' updated_at.
	f := newPageSvc(t)
	p := samplePage(5, 7, "jose")
	p.Status = model.PageStatusPublished
	p.PublishedAt = &pub
	p.UpdatedAt = earlier
	f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(p, nil)
	f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(5)).Return([]model.PageBlock{
		{ID: 1, PageID: 5, Type: "text", UpdatedAt: earlier}, {ID: 2, PageID: 5, Type: "text", UpdatedAt: later}}, nil)
	f.views.EXPECT().CountAllTime(gomock.Any(), uint64(5)).Return(int64(0), nil)
	v, err := f.svc.Get(context.Background(), 7, 5)
	require.NoError(t, err)
	assert.True(t, v.HasUnpublishedChanges)

	// List uses MaxBlockUpdatedAt for published pages only.
	draft := samplePage(6, 7, "draft")
	f.pages.EXPECT().ListByGroup(gomock.Any(), uint64(7), repository.PageFilter{Query: "j"}).Return([]repository.PageWithViews{
		{Page: *p, Views: 4}, {Page: *draft, Views: 0}}, nil)
	f.pages.EXPECT().MaxBlockUpdatedAt(gomock.Any(), uint64(5)).Return(&earlier, nil)
	list, err := f.svc.List(context.Background(), 7, repository.PageFilter{Query: "j"})
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.False(t, list[0].HasUnpublishedChanges)
	assert.Equal(t, int64(4), list[0].Views)
	assert.Nil(t, list[0].Blocks)
	assert.True(t, list[1].HasUnpublishedChanges)
}

func TestPageService_UpdateAndArchive(t *testing.T) {
	f := newPageSvc(t)
	ctx := context.Background()
	p := samplePage(5, 7, "jose")
	f.pages.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(p, nil).AnyTimes()
	f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(5)).Return([]model.PageBlock{}, nil).AnyTimes()
	f.views.EXPECT().CountAllTime(gomock.Any(), uint64(5)).Return(int64(0), nil).AnyTimes()

	bad := map[string]any(service.DefaultDesign("classic"))
	bad["font"] = "comic"
	for _, in := range []service.UpdatePageInput{
		{Design: bad},
		{Title: strp(strings.Repeat("x", 256))},
		{Description: strp(strings.Repeat("x", 501))},
		{AvatarURL: strp("javascript:alert(1)")},
	} {
		_, err := f.svc.Update(ctx, 1, 7, 5, in)
		assertValidation(t, err)
	}

	f.pages.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, got *model.Page) error {
		assert.Equal(t, "New", got.Title)
		assert.Equal(t, "/uploads/x.png", got.AvatarURL)
		assert.Equal(t, "dark", got.Design["theme"])
		return nil
	})
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPageUpdate, service.EntityPage, uint64(5), gomock.Any())
	_, err := f.svc.Update(ctx, 1, 7, 5, service.UpdatePageInput{Title: strp(" New "), AvatarURL: strp("/uploads/x.png"),
		Design: map[string]any(service.DefaultDesign("dark"))})
	require.NoError(t, err)

	f.pages.EXPECT().SetArchived(gomock.Any(), uint64(5), true).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPageHide, service.EntityPage, uint64(5), gomock.Any())
	_, err = f.svc.SetArchived(ctx, 1, 7, 5, true)
	require.NoError(t, err)
	f.pages.EXPECT().SetArchived(gomock.Any(), uint64(5), false).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPageUnhide, service.EntityPage, uint64(5), gomock.Any())
	_, err = f.svc.SetArchived(ctx, 1, 7, 5, false)
	require.NoError(t, err)

	f.pages.EXPECT().Delete(gomock.Any(), uint64(5)).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionPageDelete, service.EntityPage, uint64(5), gomock.Any())
	require.NoError(t, f.svc.Delete(ctx, 1, 7, 5))
}

func TestPageService_ValidateDesign(t *testing.T) {
	for _, tpl := range []string{"scratch", "classic", "bold", "minimal", "dark", "sunset"} {
		d := service.DefaultDesign(tpl)
		require.NotNil(t, d, tpl)
		assert.NoError(t, service.ValidateDesign(d), tpl)
	}
	assert.Nil(t, service.DefaultDesign("neon"))
	assert.Equal(t, "gradient", service.DefaultDesign("sunset")["background"].(map[string]any)["type"])

	mutate := func(fn func(d map[string]any)) map[string]any {
		d := deepCopy(service.DefaultDesign("classic"))
		fn(d)
		return d
	}
	bg := func(d map[string]any) map[string]any { return d["background"].(map[string]any) }
	btn := func(d map[string]any) map[string]any { return d["button"].(map[string]any) }
	for name, tc := range map[string]struct {
		d  map[string]any
		ok bool
	}{
		"nil":                {nil, false},
		"bad theme":          {mutate(func(d map[string]any) { d["theme"] = "neon" }), false},
		"missing theme":      {mutate(func(d map[string]any) { delete(d, "theme") }), false},
		"bad bg type":        {mutate(func(d map[string]any) { bg(d)["type"] = "image" }), false},
		"bg not object":      {mutate(func(d map[string]any) { d["background"] = "red" }), false},
		"short hex":          {mutate(func(d map[string]any) { bg(d)["color"] = "#FFF" }), false},
		"no hash":            {mutate(func(d map[string]any) { d["text_color"] = "273144" }), false},
		"non-hex":            {mutate(func(d map[string]any) { btn(d)["color"] = "#GGGGGG" }), false},
		"lowercase hex ok":   {mutate(func(d map[string]any) { btn(d)["color"] = "#abcdef" }), true},
		"number color":       {mutate(func(d map[string]any) { btn(d)["text_color"] = 255.0 }), false},
		"bad style":          {mutate(func(d map[string]any) { btn(d)["style"] = "ghost" }), false},
		"bad radius":         {mutate(func(d map[string]any) { btn(d)["radius"] = "round" }), false},
		"bad font":           {mutate(func(d map[string]any) { d["font"] = "comic" }), false},
		"gradient no color2": {mutate(func(d map[string]any) { bg(d)["type"] = "gradient"; delete(bg(d), "color2") }), false},
		"solid no color2 ok": {mutate(func(d map[string]any) { delete(bg(d), "color2") }), true},
		"theme any preset":   {mutate(func(d map[string]any) { d["theme"] = "sunset" }), true},
	} {
		err := service.ValidateDesign(tc.d)
		if tc.ok {
			assert.NoError(t, err, name)
		} else {
			assertValidation(t, err)
			assert.Error(t, err, name)
		}
	}
}

func deepCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			out[k] = deepCopy(sub)
		} else {
			out[k] = v
		}
	}
	return out
}

func TestPageService_ValidateBlockContent(t *testing.T) {
	long := func(n int) string { return strings.Repeat("x", n) }
	items := func(n int) []any {
		out := make([]any, n)
		for i := range out {
			out[i] = map[string]any{"platform": "instagram", "url": "https://instagram.com/x"}
		}
		return out
	}
	for name, tc := range map[string]struct {
		typ     string
		content map[string]any
		ok      bool
	}{
		"unknown type": {"video", map[string]any{"url": "https://x"}, false},
		"nil content":  {"text", nil, false},

		"link ok":           {"link", map[string]any{"title": "Menu", "description": "d", "thumbnail_url": "https://img.example/a.png"}, true},
		"link no title":     {"link", map[string]any{"title": "  "}, false},
		"link title 80":     {"link", map[string]any{"title": long(80)}, true},
		"link title 81":     {"link", map[string]any{"title": long(81)}, false},
		"link desc 161":     {"link", map[string]any{"title": "t", "description": long(161)}, false},
		"link bad thumb":    {"link", map[string]any{"title": "t", "thumbnail_url": "ftp://x"}, false},
		"link title number": {"link", map[string]any{"title": 5.0}, false},

		"text heading only": {"text", map[string]any{"heading": "Hi"}, true},
		"text body only":    {"text", map[string]any{"body": "Hi"}, true},
		"text empty":        {"text", map[string]any{"heading": "", "body": " "}, false},
		"text heading 121":  {"text", map[string]any{"heading": long(121)}, false},
		"text body 1000":    {"text", map[string]any{"body": long(1000)}, true},
		"text body 1001":    {"text", map[string]any{"body": long(1001)}, false},

		"image http":     {"image", map[string]any{"url": "https://img.example/a.png", "alt": "a"}, true},
		"image uploads":  {"image", map[string]any{"url": "/uploads/abc.png"}, true},
		"image relative": {"image", map[string]any{"url": "/etc/passwd"}, false},
		"image js":       {"image", map[string]any{"url": "javascript:alert(1)"}, false},
		"image missing":  {"image", map[string]any{"alt": "a"}, false},
		"image alt 121":  {"image", map[string]any{"url": "https://x.example/a", "alt": long(121)}, false},

		"social ok":         {"social", map[string]any{"items": items(1)}, true},
		"social 10":         {"social", map[string]any{"items": items(10)}, true},
		"social 11":         {"social", map[string]any{"items": items(11)}, false},
		"social empty":      {"social", map[string]any{"items": []any{}}, false},
		"social missing":    {"social", map[string]any{}, false},
		"social bad plat":   {"social", map[string]any{"items": []any{map[string]any{"platform": "myspace", "url": "https://x"}}}, false},
		"social bad url":    {"social", map[string]any{"items": []any{map[string]any{"platform": "x", "url": "x.com/me"}}}, false},
		"social mailto":     {"social", map[string]any{"items": []any{map[string]any{"platform": "email", "url": "mailto:a@b.example"}}}, true},
		"social mailto bad": {"social", map[string]any{"items": []any{map[string]any{"platform": "website", "url": "mailto:a@b.example"}}}, false},
		"social typed":      {"social", map[string]any{"items": []map[string]any{{"platform": "youtube", "url": "https://youtube.com/x"}}}, true},
		"social item str":   {"social", map[string]any{"items": []any{"https://x"}}, false},
	} {
		err := service.ValidateBlockContent(tc.typ, tc.content)
		if tc.ok {
			assert.NoError(t, err, name)
		} else if assert.Error(t, err, name) {
			assertValidation(t, err)
		}
	}
}

func TestPageService_PageViewJSON(t *testing.T) {
	list, err := json.Marshal(service.PageView{ID: 1, Slug: "a"})
	require.NoError(t, err)
	assert.NotContains(t, string(list), `"blocks"`)
	assert.Contains(t, string(list), `"has_unpublished_changes":false`)
	assert.Contains(t, string(list), `"published_at":null`)

	one, err := json.Marshal(service.PageView{ID: 1, Slug: "a", Blocks: []service.BlockView{}})
	require.NoError(t, err)
	assert.Contains(t, string(one), `"blocks":[]`)

	blk, err := json.Marshal(service.BlockView{ID: 2, Type: "text", Content: model.JSONMap{}})
	require.NoError(t, err)
	assert.Contains(t, string(blk), `"link":null`)
	assert.Contains(t, string(blk), `"link_id":null`)
}
