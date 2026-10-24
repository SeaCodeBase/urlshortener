# Passkey Duplicate Prevention Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent users from registering the same physical YubiKey/authenticator multiple times

**Architecture:** Use WebAuthn's `excludeCredentials` option during registration to tell the browser/authenticator to reject credential creation if any of the user's existing credentials are already present on the device. Also add user-friendly error handling for when this rejection occurs.

**Tech Stack:** Go (go-webauthn/webauthn), TypeScript (@simplewebauthn/browser), Next.js

---

## Task 1: Backend - Add excludeCredentials to BeginRegistration

**Files:**
- Modify: `backend/internal/service/passkey_service.go:105-135`

**Step 1: Write the failing test**

Create test file `backend/internal/service/passkey_service_test.go`:

```go
package service

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPasskeyRepository is a mock for PasskeyRepository
type MockPasskeyRepository struct {
	mock.Mock
}

func (m *MockPasskeyRepository) Create(ctx context.Context, passkey *model.Passkey) error {
	args := m.Called(ctx, passkey)
	return args.Error(0)
}

func (m *MockPasskeyRepository) GetByCredentialID(ctx context.Context, credentialID []byte) (*model.Passkey, error) {
	args := m.Called(ctx, credentialID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Passkey), args.Error(1)
}

func (m *MockPasskeyRepository) ListByUserID(ctx context.Context, userID uint64) ([]model.Passkey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Passkey), args.Error(1)
}

func (m *MockPasskeyRepository) UpdateCounter(ctx context.Context, id uint64, counter uint32) error {
	args := m.Called(ctx, id, counter)
	return args.Error(0)
}

func (m *MockPasskeyRepository) UpdateLastUsedAt(ctx context.Context, id uint64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPasskeyRepository) UpdateName(ctx context.Context, id uint64, name string) error {
	args := m.Called(ctx, id, name)
	return args.Error(0)
}

func (m *MockPasskeyRepository) Delete(ctx context.Context, id uint64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPasskeyRepository) CountByUserID(ctx context.Context, userID uint64) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

// MockUserRepository is a mock for UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepository) UpdateDisplayName(ctx context.Context, id uint64, displayName string) error {
	args := m.Called(ctx, id, displayName)
	return args.Error(0)
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, id uint64, passwordHash string) error {
	args := m.Called(ctx, id, passwordHash)
	return args.Error(0)
}

func TestBeginRegistration_ExcludesExistingCredentials(t *testing.T) {
	ctx := context.Background()
	mockPasskeyRepo := new(MockPasskeyRepository)
	mockUserRepo := new(MockUserRepository)

	// Create service with test config
	svc, err := NewPasskeyService(mockPasskeyRepo, mockUserRepo, "localhost", "http://localhost:3000", "Test App")
	assert.NoError(t, err)

	userID := uint64(1)
	existingCredentialID := []byte("existing-credential-id")

	// Mock user lookup
	mockUserRepo.On("GetByID", ctx, userID).Return(&model.User{
		ID:    userID,
		Email: "test@example.com",
	}, nil)

	// Mock existing passkeys - user already has one passkey registered
	mockPasskeyRepo.On("ListByUserID", ctx, userID).Return([]model.Passkey{
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
	options, sessionData, err := svc.BeginRegistration(ctx, userID)

	// Verify no error
	assert.NoError(t, err)
	assert.NotNil(t, options)
	assert.NotEmpty(t, sessionData)

	// Verify excludeCredentials contains the existing credential
	assert.NotNil(t, options.Response.ExcludeCredentials, "ExcludeCredentials should not be nil")
	assert.Len(t, options.Response.ExcludeCredentials, 1, "Should have 1 excluded credential")
	assert.Equal(t, existingCredentialID, []byte(options.Response.ExcludeCredentials[0].CredentialID), "Should exclude the existing credential ID")

	mockUserRepo.AssertExpectations(t)
	mockPasskeyRepo.AssertExpectations(t)
}

func TestBeginRegistration_NoExistingCredentials(t *testing.T) {
	ctx := context.Background()
	mockPasskeyRepo := new(MockPasskeyRepository)
	mockUserRepo := new(MockUserRepository)

	svc, err := NewPasskeyService(mockPasskeyRepo, mockUserRepo, "localhost", "http://localhost:3000", "Test App")
	assert.NoError(t, err)

	userID := uint64(1)

	mockUserRepo.On("GetByID", ctx, userID).Return(&model.User{
		ID:    userID,
		Email: "test@example.com",
	}, nil)

	// No existing passkeys
	mockPasskeyRepo.On("ListByUserID", ctx, userID).Return([]model.Passkey{}, nil)

	options, sessionData, err := svc.BeginRegistration(ctx, userID)

	assert.NoError(t, err)
	assert.NotNil(t, options)
	assert.NotEmpty(t, sessionData)

	// ExcludeCredentials should be empty (nil or zero length)
	assert.True(t, options.Response.ExcludeCredentials == nil || len(options.Response.ExcludeCredentials) == 0,
		"ExcludeCredentials should be empty when user has no passkeys")

	mockUserRepo.AssertExpectations(t)
	mockPasskeyRepo.AssertExpectations(t)
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test -v ./internal/service/... -run TestBeginRegistration`
Expected: FAIL - `ExcludeCredentials should not be nil` or similar assertion failure

**Step 3: Write minimal implementation**

Modify `backend/internal/service/passkey_service.go`, change `BeginRegistration` function:

```go
func (s *passkeyService) BeginRegistration(ctx context.Context, userID uint64) (*protocol.CredentialCreation, string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to get user for registration",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, "", err
	}
	waUser, err := s.toWebAuthnUser(ctx, user)
	if err != nil {
		return nil, "", err
	}

	// Build excludeCredentials from user's existing credentials
	// This prevents the same authenticator from being registered twice
	var registrationOpts []webauthn.RegistrationOption
	if len(waUser.credentials) > 0 {
		excludeList := make([]protocol.CredentialDescriptor, len(waUser.credentials))
		for i, cred := range waUser.credentials {
			excludeList[i] = protocol.CredentialDescriptor{
				Type:         protocol.PublicKeyCredentialType,
				CredentialID: cred.ID,
			}
		}
		registrationOpts = append(registrationOpts, webauthn.WithExclusions(excludeList))
	}

	options, session, err := s.webauthn.BeginRegistration(waUser, registrationOpts...)
	if err != nil {
		logger.Error(ctx, "passkey-service: WebAuthn BeginRegistration failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, "", err
	}
	sessionBytes, err := json.Marshal(session)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to marshal session data",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, "", err
	}
	return options, base64.StdEncoding.EncodeToString(sessionBytes), nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test -v ./internal/service/... -run TestBeginRegistration`
Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/service/passkey_service.go backend/internal/service/passkey_service_test.go
git commit -m "$(cat <<'EOF'
feat(passkey): add excludeCredentials to prevent duplicate authenticator registration

Prevents the same physical YubiKey/authenticator from being registered
multiple times by including existing credential IDs in the WebAuthn
excludeCredentials option during BeginRegistration.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Frontend - Handle InvalidStateError for duplicate authenticator

**Files:**
- Modify: `frontend/src/app/dashboard/settings/page.tsx:147-151`

**Step 1: Understand the error**

When `excludeCredentials` contains a credential that exists on the authenticator, `startRegistration()` from `@simplewebauthn/browser` throws an error with `name: "InvalidStateError"`. This is the standard WebAuthn behavior.

**Step 2: Update error handling in handleAddPasskey**

Modify `frontend/src/app/dashboard/settings/page.tsx`, update the catch block in `handleAddPasskey`:

```tsx
    } catch (error: unknown) {
      console.error('Passkey registration error:', error)

      // Check for InvalidStateError - thrown when authenticator already has a credential
      // from excludeCredentials list (i.e., this YubiKey is already registered)
      if (error instanceof Error && error.name === 'InvalidStateError') {
        toast.error('This security key is already registered to your account')
        return
      }

      // Check for NotAllowedError - user cancelled or timeout
      if (error instanceof Error && error.name === 'NotAllowedError') {
        toast.error('Registration was cancelled or timed out')
        return
      }

      const message = error instanceof Error ? error.message : 'Failed to add passkey'
      toast.error(message)
    } finally {
```

**Step 3: Test manually**

1. Register a YubiKey to a user account
2. Try to register the same YubiKey again
3. Expected: Toast shows "This security key is already registered to your account"

**Step 4: Commit**

```bash
git add frontend/src/app/dashboard/settings/page.tsx
git commit -m "$(cat <<'EOF'
feat(frontend): add user-friendly error for duplicate passkey registration

Shows clear message when user tries to register the same security key
twice, handling the WebAuthn InvalidStateError.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Integration test (manual verification)

**Step 1: Start backend**

Run: `cd backend && go run cmd/server/main.go`

**Step 2: Start frontend**

Run: `cd frontend && npm run dev`

**Step 3: Test scenario**

1. Login to the application
2. Go to Settings → Passkeys
3. Register a YubiKey with name "My YubiKey"
4. Verify it appears in the list
5. Try to register the same YubiKey again with a different name
6. Expected: Error message "This security key is already registered to your account"
7. Verify no duplicate entry was created in the passkeys list

**Step 4: Verify database**

Run: `mysql -u root -p urlshortener -e "SELECT id, user_id, name, HEX(credential_id) as cred_id FROM passkeys WHERE user_id = <your_user_id>;"`

Expected: Only one passkey record per physical YubiKey

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add excludeCredentials to BeginRegistration | `passkey_service.go`, `passkey_service_test.go` |
| 2 | Handle InvalidStateError in frontend | `settings/page.tsx` |
| 3 | Manual integration test | N/A |

Total estimated changes: ~60 lines backend, ~15 lines frontend