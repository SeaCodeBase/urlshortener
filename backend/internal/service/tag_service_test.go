package service_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTagService_ListByGroup_QR(t *testing.T) {
	ctrl := gomock.NewController(t)
	tagRepo := mocks.NewMockTagRepository(ctrl)
	qrRepo := mocks.NewMockQRRepository(ctrl)
	svc := service.NewTagService(tagRepo, qrRepo)

	qrRepo.EXPECT().ListTagsByGroup(gomock.Any(), uint64(7)).Return([]string{"a"}, nil)
	tags, err := svc.ListByGroup(context.Background(), 7, service.TagTypeQR)
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, tags)

	// nil from the repository is returned as an empty list.
	qrRepo.EXPECT().ListTagsByGroup(gomock.Any(), uint64(7)).Return(nil, nil)
	tags, err = svc.ListByGroup(context.Background(), 7, service.TagTypeQR)
	require.NoError(t, err)
	assert.NotNil(t, tags)
	assert.Empty(t, tags)
}
