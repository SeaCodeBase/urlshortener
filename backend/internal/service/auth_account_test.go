package service_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func newAuthSvc(t *testing.T) (*service.AuthServiceImpl, *mocks.MockUserRepository, *mocks.MockGroupRepository) {
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRepository(ctrl)
	groups := mocks.NewMockGroupRepository(ctrl)
	return service.NewAuthService(users, groups, "test-secret"), users, groups
}

func TestAuthService_UpdateEmail_Success(t *testing.T) {
	svc, users, _ := newAuthSvc(t)
	users.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(userWithPassword(t, 1, "password123"), nil)
	users.EXPECT().EmailExists(gomock.Any(), "new@example.com").Return(false, nil)
	users.EXPECT().UpdateEmail(gomock.Any(), uint64(1), "new@example.com").Return(nil)

	err := svc.UpdateEmail(context.Background(), 1, "password123", " new@example.com ")
	assert.NoError(t, err)
}

func TestAuthService_UpdateEmail_WrongPassword(t *testing.T) {
	svc, users, _ := newAuthSvc(t)
	users.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(userWithPassword(t, 1, "password123"), nil)

	err := svc.UpdateEmail(context.Background(), 1, "nope", "new@example.com")
	assert.ErrorIs(t, err, service.ErrWrongPassword)
}

func TestAuthService_UpdateEmail_InvalidEmail(t *testing.T) {
	svc, _, _ := newAuthSvc(t)
	for _, bad := range []string{"", "not-an-email", "Name <a@b.com>", "a@", "a@localhost", "a b@c.com", "@b.com"} {
		err := svc.UpdateEmail(context.Background(), 1, "password123", bad)
		assert.ErrorIs(t, err, service.ErrInvalidEmail, bad)
	}
}

func TestAuthService_UpdateEmail_Taken(t *testing.T) {
	svc, users, _ := newAuthSvc(t)
	users.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(userWithPassword(t, 1, "password123"), nil).Times(2)
	users.EXPECT().EmailExists(gomock.Any(), "taken@example.com").Return(true, nil)
	err := svc.UpdateEmail(context.Background(), 1, "password123", "taken@example.com")
	assert.ErrorIs(t, err, service.ErrEmailTaken)

	// A concurrent duplicate surfacing from the unique index maps the same way.
	users.EXPECT().EmailExists(gomock.Any(), "race@example.com").Return(false, nil)
	users.EXPECT().UpdateEmail(gomock.Any(), uint64(1), "race@example.com").Return(repository.ErrEmailExists)
	err = svc.UpdateEmail(context.Background(), 1, "password123", "race@example.com")
	assert.ErrorIs(t, err, service.ErrEmailTaken)
}

func TestAuthService_UpdateEmail_SameEmailRejected(t *testing.T) {
	svc, users, _ := newAuthSvc(t)
	users.EXPECT().GetByID(gomock.Any(), uint64(1)).Return(userWithPassword(t, 1, "password123"), nil)
	// Current email is u@example.com; comparison is case-insensitive.
	err := svc.UpdateEmail(context.Background(), 1, "password123", "U@Example.COM")
	assert.ErrorIs(t, err, service.ErrSameEmail)
}

func boolp(b bool) *bool { return &b }

func TestAuthService_UpdateAccount_AllFields(t *testing.T) {
	svc, users, groups := newAuthSvc(t)
	groups.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Group{ID: 5, UserID: 1}, nil)
	users.EXPECT().UpdateDisplayName(gomock.Any(), uint64(1), "Jose").Return(nil)
	users.EXPECT().UpdateAccount(gomock.Any(), uint64(1), sp("Acme"), boolp(true)).Return(nil)
	users.EXPECT().UpdateDefaultGroup(gomock.Any(), uint64(1), uint64(5)).Return(nil)

	err := svc.UpdateAccount(context.Background(), 1, service.UpdateAccountInput{
		DisplayName:            sp("Jose"),
		AccountName:            sp(" Acme "),
		DefaultGroupID:         u64(5),
		HomeChecklistDismissed: boolp(true),
	})
	assert.NoError(t, err)
}

func TestAuthService_UpdateAccount_OnlyDismissed(t *testing.T) {
	svc, users, _ := newAuthSvc(t)
	users.EXPECT().UpdateAccount(gomock.Any(), uint64(1), nil, boolp(false)).Return(nil)
	err := svc.UpdateAccount(context.Background(), 1, service.UpdateAccountInput{HomeChecklistDismissed: boolp(false)})
	assert.NoError(t, err)
}

func TestAuthService_UpdateAccount_DefaultGroupMustBeOwned(t *testing.T) {
	svc, _, groups := newAuthSvc(t)
	groups.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Group{ID: 5, UserID: 2}, nil)
	err := svc.UpdateAccount(context.Background(), 1, service.UpdateAccountInput{DefaultGroupID: u64(5), DisplayName: sp("x")})
	assert.ErrorIs(t, err, service.ErrGroupNotFound)

	groups.EXPECT().GetByID(gomock.Any(), uint64(6)).Return(nil, repository.ErrGroupNotFound)
	err = svc.UpdateAccount(context.Background(), 1, service.UpdateAccountInput{DefaultGroupID: u64(6)})
	assert.ErrorIs(t, err, service.ErrGroupNotFound)
}

func TestAuthService_UpdateAccount_BlankAccountName(t *testing.T) {
	svc, _, _ := newAuthSvc(t)
	err := svc.UpdateAccount(context.Background(), 1, service.UpdateAccountInput{AccountName: sp("   ")})
	assert.ErrorIs(t, err, service.ErrInvalidAccountName)
}
