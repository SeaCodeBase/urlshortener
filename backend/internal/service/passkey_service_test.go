package service_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestBeginRegistration_ExcludesExistingCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPasskeyRepo := mocks.NewMockPasskeyRepository(ctrl)
	mockUserRepo := mocks.NewMockUserRepository(ctrl)

	// Create service with test config
	svc, err := service.NewPasskeyService(mockPasskeyRepo, mockUserRepo, "localhost", "http://localhost:3000", "Test App")
	assert.NoError(t, err)

	userID := uint64(1)
	existingCredentialID := []byte("existing-credential-id")

	// Mock user lookup
	mockUserRepo.EXPECT().
		GetByID(gomock.Any(), userID).
		Return(&model.User{
			ID:    userID,
			Email: "test@example.com",
		}, nil)

	// Mock existing passkeys - user already has one passkey registered
	mockPasskeyRepo.EXPECT().
		ListByUserID(gomock.Any(), userID).
		Return([]model.Passkey{
			{
				ID:           1,
				UserID:       userID,
				Name:         "Existing YubiKey",
				CredentialID: existingCredentialID,
				PublicKey:    []byte("public-key"),
				Counter:      0,
			},
		}, nil)

	// Call BeginRegistration
	options, sessionData, err := svc.BeginRegistration(context.Background(), userID)

	// Verify no error
	assert.NoError(t, err)
	assert.NotNil(t, options)
	assert.NotEmpty(t, sessionData)

	// Verify excludeCredentials contains the existing credential
	assert.NotNil(t, options.Response.CredentialExcludeList, "CredentialExcludeList should not be nil")
	if assert.Len(t, options.Response.CredentialExcludeList, 1, "Should have 1 excluded credential") {
		assert.Equal(t, existingCredentialID, []byte(options.Response.CredentialExcludeList[0].CredentialID), "Should exclude the existing credential ID")
	}
}

func TestBeginRegistration_NoExistingCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPasskeyRepo := mocks.NewMockPasskeyRepository(ctrl)
	mockUserRepo := mocks.NewMockUserRepository(ctrl)

	svc, err := service.NewPasskeyService(mockPasskeyRepo, mockUserRepo, "localhost", "http://localhost:3000", "Test App")
	assert.NoError(t, err)

	userID := uint64(1)

	mockUserRepo.EXPECT().
		GetByID(gomock.Any(), userID).
		Return(&model.User{
			ID:    userID,
			Email: "test@example.com",
		}, nil)

	// No existing passkeys
	mockPasskeyRepo.EXPECT().
		ListByUserID(gomock.Any(), userID).
		Return([]model.Passkey{}, nil)

	options, sessionData, err := svc.BeginRegistration(context.Background(), userID)

	assert.NoError(t, err)
	assert.NotNil(t, options)
	assert.NotEmpty(t, sessionData)

	// CredentialExcludeList should be empty (nil or zero length)
	assert.True(t, options.Response.CredentialExcludeList == nil || len(options.Response.CredentialExcludeList) == 0,
		"CredentialExcludeList should be empty when user has no passkeys")
}