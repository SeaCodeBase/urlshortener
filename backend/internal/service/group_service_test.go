package service_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newGroupServiceWithDomains(t *testing.T) (*service.GroupServiceImpl, *mocks.MockGroupRepository, *mocks.MockUserRepository, *mocks.MockDomainRepository) {
	ctrl := gomock.NewController(t)
	groupRepo := mocks.NewMockGroupRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	domainRepo := mocks.NewMockDomainRepository(ctrl)
	return service.NewGroupService(groupRepo, userRepo, domainRepo), groupRepo, userRepo, domainRepo
}

func newGroupService(t *testing.T) (*service.GroupServiceImpl, *mocks.MockGroupRepository, *mocks.MockUserRepository) {
	svc, groupRepo, userRepo, _ := newGroupServiceWithDomains(t)
	return svc, groupRepo, userRepo
}

func u64(v uint64) *uint64 { return &v }

func TestGroupService_ResolveActive_HeaderOwned(t *testing.T) {
	svc, groupRepo, _ := newGroupService(t)
	g := &model.Group{ID: 7, UserID: 1, Name: "mine"}
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(7)).Return(g, nil)

	got, err := svc.ResolveActive(context.Background(), 1, u64(7))
	require.NoError(t, err)
	assert.Equal(t, g, got)
}

func TestGroupService_ResolveActive_HeaderForeign(t *testing.T) {
	svc, groupRepo, _ := newGroupService(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(7)).
		Return(&model.Group{ID: 7, UserID: 2, Name: "theirs"}, nil)

	got, err := svc.ResolveActive(context.Background(), 1, u64(7))
	assert.ErrorIs(t, err, service.ErrNotGroupOwner)
	assert.Nil(t, got)
}

func TestGroupService_ResolveActive_HeaderMissingGroup(t *testing.T) {
	svc, groupRepo, _ := newGroupService(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(7)).Return(nil, repository.ErrGroupNotFound)

	_, err := svc.ResolveActive(context.Background(), 1, u64(7))
	assert.ErrorIs(t, err, service.ErrGroupNotFound)
}

func TestGroupService_ResolveActive_NoHeaderUsesDefault(t *testing.T) {
	svc, groupRepo, userRepo := newGroupService(t)
	userRepo.EXPECT().GetByID(gomock.Any(), uint64(1)).
		Return(&model.User{ID: 1, Email: "a@example.com", DefaultGroupID: u64(9)}, nil)
	g := &model.Group{ID: 9, UserID: 1, Name: "a"}
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(g, nil)

	got, err := svc.ResolveActive(context.Background(), 1, nil)
	require.NoError(t, err)
	assert.Equal(t, g, got)
}

func TestGroupService_ResolveActive_CreatesDefaultWhenMissing(t *testing.T) {
	svc, groupRepo, userRepo := newGroupService(t)
	userRepo.EXPECT().GetByID(gomock.Any(), uint64(1)).
		Return(&model.User{ID: 1, Email: "jane.doe@example.com"}, nil)
	gomock.InOrder(
		groupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, g *model.Group) error {
				assert.Equal(t, uint64(1), g.UserID)
				assert.Equal(t, "jane.doe", g.Name)
				g.ID = 42
				return nil
			}),
		userRepo.EXPECT().SetDefaultGroupIfUnset(gomock.Any(), uint64(1), uint64(42), (*uint64)(nil)).Return(true, nil),
	)

	got, err := svc.ResolveActive(context.Background(), 1, nil)
	require.NoError(t, err)
	assert.Equal(t, uint64(42), got.ID)
	assert.Equal(t, "jane.doe", got.Name)
}

func TestGroupService_Create(t *testing.T) {
	svc, groupRepo, _ := newGroupService(t)
	groupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, g *model.Group) error {
			g.ID = 3
			return nil
		})

	got, err := svc.Create(context.Background(), 1, "  Marketing  ")
	require.NoError(t, err)
	assert.Equal(t, uint64(3), got.ID)
	assert.Equal(t, uint64(1), got.UserID)
	assert.Equal(t, "Marketing", got.Name)
}

func TestGroupService_Create_EmptyName(t *testing.T) {
	svc, _, _ := newGroupService(t)
	_, err := svc.Create(context.Background(), 1, "   ")
	assert.ErrorIs(t, err, service.ErrInvalidGroupName)
}

func TestGroupService_List(t *testing.T) {
	svc, groupRepo, _ := newGroupService(t)
	groups := []model.Group{{ID: 1, UserID: 1, Name: "a"}}
	groupRepo.EXPECT().ListByUserID(gomock.Any(), uint64(1)).Return(groups, nil)

	got, err := svc.List(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, groups, got)
}

func TestGroupService_Get_Foreign(t *testing.T) {
	svc, groupRepo, _ := newGroupService(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(5)).
		Return(&model.Group{ID: 5, UserID: 2}, nil)

	_, err := svc.Get(context.Background(), 1, 5)
	assert.ErrorIs(t, err, service.ErrNotGroupOwner)
}

func TestGroupService_Get_NotFound(t *testing.T) {
	svc, groupRepo, _ := newGroupService(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(nil, repository.ErrGroupNotFound)

	_, err := svc.Get(context.Background(), 1, 5)
	assert.ErrorIs(t, err, service.ErrGroupNotFound)
}

func TestGroupService_Update(t *testing.T) {
	svc, groupRepo, _, domainRepo := newGroupServiceWithDomains(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(5)).
		Return(&model.Group{ID: 5, UserID: 1, Name: "old"}, nil)
	domainRepo.EXPECT().GetByID(gomock.Any(), uint64(8)).
		Return(&model.Domain{ID: 8, UserID: 1, VerificationStatus: model.DomainVerificationOK}, nil)
	groupRepo.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, g *model.Group) error {
			assert.Equal(t, "new", g.Name)
			require.NotNil(t, g.DefaultDomainID)
			assert.Equal(t, uint64(8), *g.DefaultDomainID)
			return nil
		})

	name := "new"
	got, err := svc.Update(context.Background(), 1, 5, service.UpdateGroupInput{Name: &name, DefaultDomainID: u64(8)})
	require.NoError(t, err)
	assert.Equal(t, "new", got.Name)
}

func TestGroupService_Update_ClearDefaultDomain(t *testing.T) {
	svc, groupRepo, _ := newGroupService(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(5)).
		Return(&model.Group{ID: 5, UserID: 1, Name: "g", DefaultDomainID: u64(8)}, nil)
	groupRepo.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, g *model.Group) error {
			assert.Nil(t, g.DefaultDomainID)
			assert.Equal(t, "g", g.Name)
			return nil
		})

	got, err := svc.Update(context.Background(), 1, 5, service.UpdateGroupInput{ClearDefaultDomain: true})
	require.NoError(t, err)
	assert.Nil(t, got.DefaultDomainID)
}

func TestGroupService_Update_Foreign(t *testing.T) {
	svc, groupRepo, _ := newGroupService(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(5)).
		Return(&model.Group{ID: 5, UserID: 2, Name: "theirs"}, nil)

	name := "mine now"
	_, err := svc.Update(context.Background(), 1, 5, service.UpdateGroupInput{Name: &name})
	assert.ErrorIs(t, err, service.ErrNotGroupOwner)
}

func TestGroupService_Update_EmptyName(t *testing.T) {
	svc, groupRepo, _ := newGroupService(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(5)).
		Return(&model.Group{ID: 5, UserID: 1, Name: "g"}, nil)

	name := " "
	_, err := svc.Update(context.Background(), 1, 5, service.UpdateGroupInput{Name: &name})
	assert.ErrorIs(t, err, service.ErrInvalidGroupName)
}

func TestGroupService_ResolveActive_DefaultGroupGoneRecreates(t *testing.T) {
	svc, groupRepo, userRepo := newGroupService(t)
	userRepo.EXPECT().GetByID(gomock.Any(), uint64(1)).
		Return(&model.User{ID: 1, Email: "bob@example.com", DefaultGroupID: u64(9)}, nil)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(nil, repository.ErrGroupNotFound)
	groupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, g *model.Group) error {
			g.ID = 10
			return nil
		})
	userRepo.EXPECT().SetDefaultGroupIfUnset(gomock.Any(), uint64(1), uint64(10), u64(9)).Return(true, nil)

	got, err := svc.ResolveActive(context.Background(), 1, nil)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), got.ID)
	assert.Equal(t, "bob", got.Name)
}

func TestGroupService_ResolveActive_LostRaceUsesWinner(t *testing.T) {
	svc, groupRepo, userRepo := newGroupService(t)
	winner := &model.Group{ID: 50, UserID: 1, Name: "jane"}
	gomock.InOrder(
		userRepo.EXPECT().GetByID(gomock.Any(), uint64(1)).
			Return(&model.User{ID: 1, Email: "jane@example.com"}, nil),
		groupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, g *model.Group) error {
				g.ID = 51
				return nil
			}),
		userRepo.EXPECT().SetDefaultGroupIfUnset(gomock.Any(), uint64(1), uint64(51), (*uint64)(nil)).
			Return(false, nil),
		groupRepo.EXPECT().Delete(gomock.Any(), uint64(51)).Return(nil),
		userRepo.EXPECT().GetByID(gomock.Any(), uint64(1)).
			Return(&model.User{ID: 1, Email: "jane@example.com", DefaultGroupID: u64(50)}, nil),
		groupRepo.EXPECT().GetByID(gomock.Any(), uint64(50)).Return(winner, nil),
	)

	got, err := svc.ResolveActive(context.Background(), 1, nil)
	require.NoError(t, err)
	assert.Equal(t, winner, got)
}

func TestGroupService_Update_ForeignDomainRejected(t *testing.T) {
	svc, groupRepo, _, domainRepo := newGroupServiceWithDomains(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(5)).
		Return(&model.Group{ID: 5, UserID: 1, Name: "g"}, nil)
	domainRepo.EXPECT().GetByID(gomock.Any(), uint64(8)).
		Return(&model.Domain{ID: 8, UserID: 2, VerificationStatus: model.DomainVerificationOK}, nil)

	_, err := svc.Update(context.Background(), 1, 5, service.UpdateGroupInput{DefaultDomainID: u64(8)})
	assert.ErrorIs(t, err, service.ErrNotDomainOwner)
}

func TestGroupService_Update_MissingDomainRejected(t *testing.T) {
	svc, groupRepo, _, domainRepo := newGroupServiceWithDomains(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(5)).
		Return(&model.Group{ID: 5, UserID: 1, Name: "g"}, nil)
	domainRepo.EXPECT().GetByID(gomock.Any(), uint64(8)).
		Return(nil, repository.ErrDomainNotFound)

	_, err := svc.Update(context.Background(), 1, 5, service.UpdateGroupInput{DefaultDomainID: u64(8)})
	assert.ErrorIs(t, err, service.ErrNotDomainOwner)
}

func TestGroupService_Update_UnverifiedDomainRejected(t *testing.T) {
	svc, groupRepo, _, domainRepo := newGroupServiceWithDomains(t)
	groupRepo.EXPECT().GetByID(gomock.Any(), uint64(5)).
		Return(&model.Group{ID: 5, UserID: 1, Name: "g"}, nil)
	domainRepo.EXPECT().GetByID(gomock.Any(), uint64(8)).
		Return(&model.Domain{ID: 8, UserID: 1, VerificationStatus: model.DomainVerificationPending}, nil)

	_, err := svc.Update(context.Background(), 1, 5, service.UpdateGroupInput{DefaultDomainID: u64(8)})
	assert.ErrorIs(t, err, service.ErrDomainNotVerified)
}
