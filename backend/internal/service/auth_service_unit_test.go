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
	"golang.org/x/crypto/bcrypt"
)

func TestAuthServiceImpl_ChangePassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

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
	svc := service.NewAuthService(mockUserRepo, "test-secret")

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
	svc := service.NewAuthService(mockUserRepo, "test-secret")

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
	svc := service.NewAuthService(mockUserRepo, "test-secret")

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
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	mockUserRepo.EXPECT().
		EmailExists(gomock.Any(), "new@example.com").
		Return(false, nil)

	mockUserRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, user *model.User) error {
			user.ID = 1
			return nil
		})

	input := service.RegisterInput{
		Email:    "new@example.com",
		Password: "password123",
	}

	resp, err := svc.Register(context.Background(), input)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "new@example.com", resp.User.Email)
}

func TestAuthServiceImpl_Register_EmailTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

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
