package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthServiceImpl_ChangePassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, mocks.NewMockGroupRepository(ctrl), "test-secret")

	oldPass := "oldpassword123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(oldPass), bcrypt.DefaultCost)
	user := &model.User{ID: 1, Email: "test@example.com", PasswordHash: string(hash)}

	mockUserRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(user, nil)

	mockUserRepo.EXPECT().
		UpdatePassword(gomock.Any(), uint64(1), gomock.Any()).
		Return(nil)

	input := service.ChangePasswordInput{
		OldPassword: oldPass,
		NewPassword: "newpassword456",
	}

	err := svc.ChangePassword(context.Background(), uint64(1), input)
	assert.NoError(t, err)
}

func TestAuthServiceImpl_ChangePassword_WrongPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, mocks.NewMockGroupRepository(ctrl), "test-secret")

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	user := &model.User{ID: 1, Email: "test@example.com", PasswordHash: string(hash)}

	mockUserRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(user, nil)

	input := service.ChangePasswordInput{
		OldPassword: "wrongpassword",
		NewPassword: "newpassword456",
	}

	err := svc.ChangePassword(context.Background(), uint64(1), input)
	assert.ErrorIs(t, err, service.ErrWrongPassword)
}

func TestAuthServiceImpl_Login_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, mocks.NewMockGroupRepository(ctrl), "test-secret")

	password := "testpassword123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := &model.User{ID: 1, Email: "test@example.com", PasswordHash: string(hash)}

	mockUserRepo.EXPECT().
		GetByEmail(gomock.Any(), "test@example.com").
		Return(user, nil)

	input := service.LoginInput{
		Email:    "test@example.com",
		Password: password,
	}

	resp, err := svc.Login(context.Background(), input)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, user.Email, resp.User.Email)
}

func TestAuthServiceImpl_Login_InvalidCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, mocks.NewMockGroupRepository(ctrl), "test-secret")

	mockUserRepo.EXPECT().
		GetByEmail(gomock.Any(), "notfound@example.com").
		Return(nil, repository.ErrUserNotFound)

	input := service.LoginInput{
		Email:    "notfound@example.com",
		Password: "anypassword",
	}

	resp, err := svc.Login(context.Background(), input)
	assert.ErrorIs(t, err, service.ErrInvalidCredentials)
	assert.Nil(t, resp)
}

func TestAuthServiceImpl_Register_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	mockGroupRepo := mocks.NewMockGroupRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, mockGroupRepo, "test-secret")

	mockUserRepo.EXPECT().
		EmailExists(gomock.Any(), "new@example.com").
		Return(false, nil)

	mockUserRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, user *model.User) error {
			user.ID = 1
			return nil
		})
	mockGroupRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mockUserRepo.EXPECT().UpdateDefaultGroup(gomock.Any(), uint64(1), gomock.Any()).Return(nil)
	createdAt := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	mockUserRepo.EXPECT().GetByID(gomock.Any(), uint64(1)).
		Return(&model.User{ID: 1, Email: "new@example.com", CreatedAt: createdAt}, nil)

	input := service.RegisterInput{
		Email:    "new@example.com",
		Password: "password123",
	}

	resp, err := svc.Register(context.Background(), input)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "new@example.com", resp.User.Email)
	assert.True(t, resp.User.CreatedAt.Equal(createdAt), "created_at comes from the stored row")
}

func TestAuthServiceImpl_Register_EmailTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, mocks.NewMockGroupRepository(ctrl), "test-secret")

	mockUserRepo.EXPECT().
		EmailExists(gomock.Any(), "taken@example.com").
		Return(true, nil)

	input := service.RegisterInput{
		Email:    "taken@example.com",
		Password: "password123",
	}

	resp, err := svc.Register(context.Background(), input)
	assert.ErrorIs(t, err, service.ErrEmailTaken)
	assert.Nil(t, resp)
}

func TestAuthServiceImpl_Register_CreatesDefaultGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	mockGroupRepo := mocks.NewMockGroupRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, mockGroupRepo, "test-secret")

	mockUserRepo.EXPECT().EmailExists(gomock.Any(), "jane.doe@example.com").Return(false, nil)
	gomock.InOrder(
		mockUserRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, user *model.User) error {
				user.ID = 5
				return nil
			}),
		mockGroupRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, g *model.Group) error {
				assert.Equal(t, uint64(5), g.UserID)
				assert.Equal(t, "jane.doe", g.Name)
				g.ID = 77
				return nil
			}),
		mockUserRepo.EXPECT().UpdateDefaultGroup(gomock.Any(), uint64(5), uint64(77)).Return(nil),
		mockUserRepo.EXPECT().GetByID(gomock.Any(), uint64(5)).
			Return(&model.User{ID: 5, Email: "jane.doe@example.com", DefaultGroupID: u64(77)}, nil),
	)

	resp, err := svc.Register(context.Background(), service.RegisterInput{
		Email:    "jane.doe@example.com",
		Password: "password123",
	})
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		if assert.NotNil(t, resp.User.DefaultGroupID) {
			assert.Equal(t, uint64(77), *resp.User.DefaultGroupID)
		}
	}
}
