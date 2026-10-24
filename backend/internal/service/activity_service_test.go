package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestActivityService_Log(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	svc := service.NewActivityService(repo)

	repo.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *model.ActivityEntry) error {
		assert.Equal(t, uint64(1), e.UserID)
		assert.Equal(t, uint64(2), e.GroupID)
		assert.Equal(t, service.ActionLinkCreate, e.Action)
		assert.Equal(t, "link", e.EntityType)
		assert.Equal(t, uint64(3), e.EntityID)
		assert.Equal(t, "abc", e.Metadata["short_code"])
		return nil
	})
	svc.Log(context.Background(), 1, 2, service.ActionLinkCreate, "link", 3, map[string]any{"short_code": "abc"})

	// nil metadata and repository failures are tolerated silently.
	repo.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *model.ActivityEntry) error {
		assert.Nil(t, e.Metadata)
		return errors.New("db down")
	})
	assert.NotPanics(t, func() {
		svc.Log(context.Background(), 1, 2, service.ActionLinkDelete, "link", 3, nil)
	})
}

func TestActivityService_List_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockActivityRepository(ctrl)
	svc := service.NewActivityService(repo)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	repo.EXPECT().List(gomock.Any(), uint64(2), &from, nil, 20, 40).
		Return([]model.ActivityEntry{{ID: 9}}, int64(41), nil)
	res, err := svc.List(context.Background(), 2, &from, nil, 3, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(41), res.Total)
	assert.Equal(t, 3, res.Page)
	assert.Equal(t, 3, res.TotalPages)
	assert.Len(t, res.Entries, 1)

	// Out-of-range page/limit are clamped; nil entries become empty.
	repo.EXPECT().List(gomock.Any(), uint64(2), nil, nil, 100, 0).Return(nil, int64(0), nil)
	res, err = svc.List(context.Background(), 2, nil, nil, 0, 500)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Page)
	assert.Equal(t, 0, res.TotalPages)
	assert.NotNil(t, res.Entries)

	repo.EXPECT().List(gomock.Any(), uint64(2), nil, nil, 20, 0).Return(nil, int64(0), errors.New("boom"))
	_, err = svc.List(context.Background(), 2, nil, nil, 1, 0)
	assert.Error(t, err)
}

func TestTagService_ListByGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockTagRepository(ctrl)
	svc := service.NewTagService(repo, mocks.NewMockQRRepository(ctrl))

	repo.EXPECT().ListByGroup(gomock.Any(), uint64(5)).Return([]string{"a", "b"}, nil).Times(2)
	tags, err := svc.ListByGroup(context.Background(), 5, "link")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, tags)
	tags, err = svc.ListByGroup(context.Background(), 5, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, tags)

	_, err = svc.ListByGroup(context.Background(), 5, "bogus")
	assert.ErrorIs(t, err, service.ErrInvalidTagType)
}
