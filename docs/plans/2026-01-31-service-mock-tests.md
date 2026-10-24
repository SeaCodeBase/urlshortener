# Service Layer Mock Tests Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace all database-based service tests with proper unit tests using mockgen-generated mocks. Delete tests that use `testutil.SetupTestDB(t)`.

**Architecture:** Unit tests mock repository interfaces using mockgen. No database connections in service tests.

**Tech Stack:** Go, mockgen (`go.uber.org/mock`), testify/assert

---

## Current State

| File | Status | Action |
|------|--------|--------|
| `auth_service_test.go` | Uses `testutil.SetupTestDB` | DELETE |
| `auth_service_unit_test.go` | Uses mockgen mocks | KEEP (rename to `auth_service_test.go`) |
| `link_service_test.go` | Missing | CREATE with mocks |
| `stats_service_test.go` | Missing | CREATE with mocks |
| `shortcode_service_test.go` | Uses manual mocks | REFACTOR to use mockgen |

---

## Task 1: Delete Old Auth Tests and Rename Unit Tests

**Files:**
- Delete: `backend/internal/service/auth_service_test.go`
- Rename: `backend/internal/service/auth_service_unit_test.go` → `auth_service_test.go`

### Step 1: Delete the DB-based test file

Run:
```bash
rm /Users/jose/GolandProjects/urlshortener/backend/internal/service/auth_service_test.go
```

### Step 2: Rename unit test file

Run:
```bash
mv /Users/jose/GolandProjects/urlshortener/backend/internal/service/auth_service_unit_test.go /Users/jose/GolandProjects/urlshortener/backend/internal/service/auth_service_test.go
```

### Step 3: Add missing test for ValidateToken

Add to `backend/internal/service/auth_service_test.go`:

```go
func TestAuthServiceImpl_ValidateToken_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	// Register to get a valid token
	mockUserRepo.EXPECT().
		EmailExists(gomock.Any(), "test@example.com").
		Return(false, nil)

	mockUserRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, user *model.User) error {
			user.ID = 123
			return nil
		})

	resp, err := svc.Register(context.Background(), service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	})
	assert.NoError(t, err)

	// Validate the token
	userID, err := svc.ValidateToken(resp.Token)
	assert.NoError(t, err)
	assert.Equal(t, uint64(123), userID)
}

func TestAuthServiceImpl_ValidateToken_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	_, err := svc.ValidateToken("invalid-token")
	assert.Error(t, err)
}

func TestAuthServiceImpl_GetUserByID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	expectedUser := &model.User{ID: 1, Email: "test@example.com"}

	mockUserRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(expectedUser, nil)

	user, err := svc.GetUserByID(context.Background(), uint64(1))
	assert.NoError(t, err)
	assert.Equal(t, expectedUser.Email, user.Email)
}

func TestAuthServiceImpl_GetUserByID_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	mockUserRepo.EXPECT().
		GetByID(gomock.Any(), uint64(999)).
		Return(nil, repository.ErrUserNotFound)

	user, err := svc.GetUserByID(context.Background(), uint64(999))
	assert.ErrorIs(t, err, repository.ErrUserNotFound)
	assert.Nil(t, user)
}

func TestAuthServiceImpl_ChangePassword_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	mockUserRepo.EXPECT().
		GetByID(gomock.Any(), uint64(999)).
		Return(nil, repository.ErrUserNotFound)

	err := svc.ChangePassword(context.Background(), uint64(999), service.ChangePasswordInput{
		OldPassword: "old",
		NewPassword: "new12345",
	})
	assert.ErrorIs(t, err, repository.ErrUserNotFound)
}

func TestAuthServiceImpl_Login_WrongPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	user := &model.User{ID: 1, Email: "test@example.com", PasswordHash: string(hash)}

	mockUserRepo.EXPECT().
		GetByEmail(gomock.Any(), "test@example.com").
		Return(user, nil)

	resp, err := svc.Login(context.Background(), service.LoginInput{
		Email:    "test@example.com",
		Password: "wrongpassword",
	})
	assert.ErrorIs(t, err, service.ErrInvalidCredentials)
	assert.Nil(t, resp)
}
```

### Step 4: Run tests

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/service/... -run TestAuthService -v`
Expected: All tests PASS

### Step 5: Commit

```bash
git add -A
git commit -m "refactor(test): replace auth_service DB tests with mock tests"
```

---

## Task 2: Create LinkService Mock Tests

**Files:**
- Create: `backend/internal/service/link_service_test.go`

### Step 1: Create LinkService tests

Create `backend/internal/service/link_service_test.go`:

```go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	repoMocks "github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	svcMocks "github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestLinkServiceImpl_Create_AutoGeneratedCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	mockShortCode.EXPECT().
		Generate(gomock.Any()).
		Return("abc1234", nil)

	mockLinkRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, link *model.Link) error {
			link.ID = 1
			return nil
		})

	input := service.CreateLinkInput{
		OriginalURL: "https://example.com",
	}

	link, err := svc.Create(context.Background(), uint64(1), input)
	assert.NoError(t, err)
	assert.Equal(t, "abc1234", link.ShortCode)
	assert.Equal(t, "https://example.com", link.OriginalURL)
	assert.True(t, link.IsActive)
}

func TestLinkServiceImpl_Create_CustomCode_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	mockShortCode.EXPECT().
		IsValid("mycode").
		Return(true)

	mockShortCode.EXPECT().
		IsAvailable(gomock.Any(), "mycode").
		Return(true, nil)

	mockLinkRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, link *model.Link) error {
			link.ID = 1
			return nil
		})

	input := service.CreateLinkInput{
		OriginalURL: "https://example.com",
		CustomCode:  "mycode",
	}

	link, err := svc.Create(context.Background(), uint64(1), input)
	assert.NoError(t, err)
	assert.Equal(t, "mycode", link.ShortCode)
}

func TestLinkServiceImpl_Create_CustomCode_Invalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	mockShortCode.EXPECT().
		IsValid("ab").
		Return(false)

	input := service.CreateLinkInput{
		OriginalURL: "https://example.com",
		CustomCode:  "ab", // too short
	}

	link, err := svc.Create(context.Background(), uint64(1), input)
	assert.ErrorIs(t, err, service.ErrInvalidShortCode)
	assert.Nil(t, link)
}

func TestLinkServiceImpl_Create_CustomCode_Taken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	mockShortCode.EXPECT().
		IsValid("taken1").
		Return(true)

	mockShortCode.EXPECT().
		IsAvailable(gomock.Any(), "taken1").
		Return(false, nil)

	input := service.CreateLinkInput{
		OriginalURL: "https://example.com",
		CustomCode:  "taken1",
	}

	link, err := svc.Create(context.Background(), uint64(1), input)
	assert.ErrorIs(t, err, service.ErrShortCodeTaken)
	assert.Nil(t, link)
}

func TestLinkServiceImpl_Create_WithTitleAndExpiry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	mockShortCode.EXPECT().
		Generate(gomock.Any()).
		Return("abc1234", nil)

	mockLinkRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, link *model.Link) error {
			link.ID = 1
			assert.NotNil(t, link.Title)
			assert.Equal(t, "My Link", *link.Title)
			assert.True(t, link.ExpiresAt.Valid)
			return nil
		})

	expiresAt := time.Now().Add(24 * time.Hour)
	input := service.CreateLinkInput{
		OriginalURL: "https://example.com",
		Title:       "My Link",
		ExpiresAt:   &expiresAt,
	}

	link, err := svc.Create(context.Background(), uint64(1), input)
	assert.NoError(t, err)
	assert.NotNil(t, link)
}

func TestLinkServiceImpl_GetByID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	expectedLink := &model.Link{ID: 1, UserID: 1, ShortCode: "abc1234"}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(expectedLink, nil)

	link, err := svc.GetByID(context.Background(), uint64(1), uint64(1))
	assert.NoError(t, err)
	assert.Equal(t, expectedLink.ShortCode, link.ShortCode)
}

func TestLinkServiceImpl_GetByID_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(999)).
		Return(nil, repository.ErrLinkNotFound)

	link, err := svc.GetByID(context.Background(), uint64(1), uint64(999))
	assert.ErrorIs(t, err, service.ErrLinkNotFound)
	assert.Nil(t, link)
}

func TestLinkServiceImpl_GetByID_NotOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	// Link belongs to user 2, but user 1 is trying to access
	linkOfOtherUser := &model.Link{ID: 1, UserID: 2, ShortCode: "abc1234"}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(linkOfOtherUser, nil)

	link, err := svc.GetByID(context.Background(), uint64(1), uint64(1))
	assert.ErrorIs(t, err, service.ErrNotLinkOwner)
	assert.Nil(t, link)
}

func TestLinkServiceImpl_List_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	links := []model.Link{
		{ID: 1, UserID: 1, ShortCode: "abc1234"},
		{ID: 2, UserID: 1, ShortCode: "def5678"},
	}

	mockLinkRepo.EXPECT().
		ListByUserID(gomock.Any(), uint64(1), 20, 0).
		Return(links, nil)

	mockLinkRepo.EXPECT().
		CountByUserID(gomock.Any(), uint64(1)).
		Return(int64(2), nil)

	result, err := svc.List(context.Background(), uint64(1), service.ListLinksParams{Page: 1, Limit: 20})
	assert.NoError(t, err)
	assert.Len(t, result.Links, 2)
	assert.Equal(t, int64(2), result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 1, result.TotalPages)
}

func TestLinkServiceImpl_List_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	links := []model.Link{
		{ID: 11, UserID: 1, ShortCode: "page2a"},
	}

	mockLinkRepo.EXPECT().
		ListByUserID(gomock.Any(), uint64(1), 10, 10). // page 2, limit 10
		Return(links, nil)

	mockLinkRepo.EXPECT().
		CountByUserID(gomock.Any(), uint64(1)).
		Return(int64(25), nil)

	result, err := svc.List(context.Background(), uint64(1), service.ListLinksParams{Page: 2, Limit: 10})
	assert.NoError(t, err)
	assert.Len(t, result.Links, 1)
	assert.Equal(t, int64(25), result.Total)
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 3, result.TotalPages)
}

func TestLinkServiceImpl_List_DefaultParams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	mockLinkRepo.EXPECT().
		ListByUserID(gomock.Any(), uint64(1), 20, 0). // default limit 20, page 1
		Return([]model.Link{}, nil)

	mockLinkRepo.EXPECT().
		CountByUserID(gomock.Any(), uint64(1)).
		Return(int64(0), nil)

	result, err := svc.List(context.Background(), uint64(1), service.ListLinksParams{})
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Page)
}

func TestLinkServiceImpl_Update_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	existingLink := &model.Link{ID: 1, UserID: 1, ShortCode: "abc1234", OriginalURL: "https://old.com", IsActive: true}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(existingLink, nil)

	mockLinkRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, link *model.Link) error {
			assert.Equal(t, "https://new.com", link.OriginalURL)
			return nil
		})

	input := service.UpdateLinkInput{
		OriginalURL: "https://new.com",
	}

	link, err := svc.Update(context.Background(), uint64(1), uint64(1), input)
	assert.NoError(t, err)
	assert.Equal(t, "https://new.com", link.OriginalURL)
}

func TestLinkServiceImpl_Update_NotOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	linkOfOtherUser := &model.Link{ID: 1, UserID: 2, ShortCode: "abc1234"}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(linkOfOtherUser, nil)

	input := service.UpdateLinkInput{
		OriginalURL: "https://new.com",
	}

	link, err := svc.Update(context.Background(), uint64(1), uint64(1), input)
	assert.ErrorIs(t, err, service.ErrNotLinkOwner)
	assert.Nil(t, link)
}

func TestLinkServiceImpl_Update_DeactivateLink(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	existingLink := &model.Link{ID: 1, UserID: 1, ShortCode: "abc1234", IsActive: true}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(existingLink, nil)

	mockLinkRepo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, link *model.Link) error {
			assert.False(t, link.IsActive)
			return nil
		})

	isActive := false
	input := service.UpdateLinkInput{
		IsActive: &isActive,
	}

	link, err := svc.Update(context.Background(), uint64(1), uint64(1), input)
	assert.NoError(t, err)
	assert.False(t, link.IsActive)
}

func TestLinkServiceImpl_Delete_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	existingLink := &model.Link{ID: 1, UserID: 1, ShortCode: "abc1234"}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(existingLink, nil)

	mockLinkRepo.EXPECT().
		Delete(gomock.Any(), uint64(1)).
		Return(nil)

	err := svc.Delete(context.Background(), uint64(1), uint64(1))
	assert.NoError(t, err)
}

func TestLinkServiceImpl_Delete_NotOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	linkOfOtherUser := &model.Link{ID: 1, UserID: 2, ShortCode: "abc1234"}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(linkOfOtherUser, nil)

	err := svc.Delete(context.Background(), uint64(1), uint64(1))
	assert.ErrorIs(t, err, service.ErrNotLinkOwner)
}

func TestLinkServiceImpl_Delete_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := repoMocks.NewMockLinkRepository(ctrl)
	mockShortCode := svcMocks.NewMockShortCodeService(ctrl)
	svc := service.NewLinkService(mockLinkRepo, mockShortCode)

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(999)).
		Return(nil, repository.ErrLinkNotFound)

	err := svc.Delete(context.Background(), uint64(1), uint64(999))
	assert.ErrorIs(t, err, service.ErrLinkNotFound)
}
```

### Step 2: Run tests

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/service/... -run TestLinkService -v`
Expected: All tests PASS

### Step 3: Commit

```bash
git add backend/internal/service/link_service_test.go
git commit -m "test(service): add LinkService mock tests"
```

---

## Task 3: Create StatsService Mock Tests

**Files:**
- Create: `backend/internal/service/stats_service_test.go`

### Step 1: Create StatsService tests

Create `backend/internal/service/stats_service_test.go`:

```go
package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestStatsServiceImpl_GetLinkStats_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClickRepo := mocks.NewMockClickRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewStatsService(mockClickRepo, mockLinkRepo)

	link := &model.Link{ID: 1, UserID: 1, ShortCode: "abc123"}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(link, nil)

	mockClickRepo.EXPECT().
		GetStatsByLinkID(gomock.Any(), uint64(1)).
		Return(&repository.ClickStats{TotalClicks: 100, UniqueVisitors: 50}, nil)

	mockClickRepo.EXPECT().
		GetDailyStats(gomock.Any(), uint64(1), 30).
		Return([]repository.DailyClickStats{
			{Date: "2026-01-31", Clicks: 10},
			{Date: "2026-01-30", Clicks: 15},
		}, nil)

	mockClickRepo.EXPECT().
		GetTopReferrers(gomock.Any(), uint64(1), 10).
		Return([]repository.ReferrerStats{
			{Referrer: "google.com", Count: 20},
			{Referrer: "Direct", Count: 30},
		}, nil)

	mockClickRepo.EXPECT().
		GetDeviceStats(gomock.Any(), uint64(1)).
		Return([]repository.DeviceStats{
			{DeviceType: "desktop", Count: 60},
			{DeviceType: "mobile", Count: 40},
		}, nil)

	mockClickRepo.EXPECT().
		GetBrowserStats(gomock.Any(), uint64(1)).
		Return([]repository.BrowserStats{
			{Browser: "Chrome", Count: 40},
			{Browser: "Firefox", Count: 30},
		}, nil)

	resp, err := svc.GetLinkStats(context.Background(), uint64(1), uint64(1))
	assert.NoError(t, err)
	assert.Equal(t, int64(100), resp.TotalClicks)
	assert.Equal(t, int64(50), resp.UniqueVisitors)
	assert.Len(t, resp.DailyStats, 2)
	assert.Len(t, resp.TopReferrers, 2)
	assert.Len(t, resp.DeviceStats, 2)
	assert.Len(t, resp.BrowserStats, 2)
}

func TestStatsServiceImpl_GetLinkStats_NotOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClickRepo := mocks.NewMockClickRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewStatsService(mockClickRepo, mockLinkRepo)

	// Link belongs to user 2
	link := &model.Link{ID: 1, UserID: 2, ShortCode: "abc123"}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(link, nil)

	resp, err := svc.GetLinkStats(context.Background(), uint64(1), uint64(1))
	assert.ErrorIs(t, err, service.ErrNotLinkOwner)
	assert.Nil(t, resp)
}

func TestStatsServiceImpl_GetLinkStats_LinkNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClickRepo := mocks.NewMockClickRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewStatsService(mockClickRepo, mockLinkRepo)

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(999)).
		Return(nil, repository.ErrLinkNotFound)

	resp, err := svc.GetLinkStats(context.Background(), uint64(1), uint64(999))
	assert.ErrorIs(t, err, repository.ErrLinkNotFound)
	assert.Nil(t, resp)
}

func TestStatsServiceImpl_GetLinkStats_ClickRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClickRepo := mocks.NewMockClickRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewStatsService(mockClickRepo, mockLinkRepo)

	link := &model.Link{ID: 1, UserID: 1, ShortCode: "abc123"}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(link, nil)

	mockClickRepo.EXPECT().
		GetStatsByLinkID(gomock.Any(), uint64(1)).
		Return(nil, errors.New("database error"))

	resp, err := svc.GetLinkStats(context.Background(), uint64(1), uint64(1))
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestStatsServiceImpl_GetLinkStats_EmptyStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClickRepo := mocks.NewMockClickRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewStatsService(mockClickRepo, mockLinkRepo)

	link := &model.Link{ID: 1, UserID: 1, ShortCode: "abc123"}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(link, nil)

	mockClickRepo.EXPECT().
		GetStatsByLinkID(gomock.Any(), uint64(1)).
		Return(&repository.ClickStats{TotalClicks: 0, UniqueVisitors: 0}, nil)

	mockClickRepo.EXPECT().
		GetDailyStats(gomock.Any(), uint64(1), 30).
		Return([]repository.DailyClickStats{}, nil)

	mockClickRepo.EXPECT().
		GetTopReferrers(gomock.Any(), uint64(1), 10).
		Return([]repository.ReferrerStats{}, nil)

	mockClickRepo.EXPECT().
		GetDeviceStats(gomock.Any(), uint64(1)).
		Return([]repository.DeviceStats{}, nil)

	mockClickRepo.EXPECT().
		GetBrowserStats(gomock.Any(), uint64(1)).
		Return([]repository.BrowserStats{}, nil)

	resp, err := svc.GetLinkStats(context.Background(), uint64(1), uint64(1))
	assert.NoError(t, err)
	assert.Equal(t, int64(0), resp.TotalClicks)
	assert.Equal(t, int64(0), resp.UniqueVisitors)
	assert.Empty(t, resp.DailyStats)
	assert.Empty(t, resp.TopReferrers)
	assert.Empty(t, resp.DeviceStats)
	assert.Empty(t, resp.BrowserStats)
}
```

### Step 2: Run tests

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/service/... -run TestStatsService -v`
Expected: All tests PASS

### Step 3: Commit

```bash
git add backend/internal/service/stats_service_test.go
git commit -m "test(service): add StatsService mock tests"
```

---

## Task 4: Refactor ShortCodeService Tests to Use Mockgen

**Files:**
- Modify: `backend/internal/service/shortcode_service_test.go`

### Step 1: Update ShortCodeService tests

Replace `backend/internal/service/shortcode_service_test.go`:

```go
package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestShortCodeServiceImpl_IsValid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewShortCodeService(mockLinkRepo)

	tests := []struct {
		name  string
		code  string
		valid bool
	}{
		{"valid 6 chars", "abc123", true},
		{"valid mixed case", "Ab3XyZ", true},
		{"valid max length 16 chars", "abcdefgh12345678", true},
		{"valid min length 4 chars", "ab12", true},
		{"too short 3 chars", "abc", false},
		{"too long 17 chars", "abcdefgh123456789", false},
		{"invalid char underscore", "abc_123", false},
		{"invalid char hyphen", "abc-123", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.IsValid(tt.code)
			assert.Equal(t, tt.valid, got, "IsValid(%q) should be %v", tt.code, tt.valid)
		})
	}
}

func TestShortCodeServiceImpl_Generate_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewShortCodeService(mockLinkRepo)

	mockLinkRepo.EXPECT().
		ShortCodeExists(gomock.Any(), gomock.Any()).
		Return(false, nil)

	code, err := svc.Generate(context.Background())
	assert.NoError(t, err)
	assert.Len(t, code, 7)
}

func TestShortCodeServiceImpl_Generate_RetriesOnCollision(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewShortCodeService(mockLinkRepo)

	// First 3 calls return exists=true, 4th returns false
	gomock.InOrder(
		mockLinkRepo.EXPECT().
			ShortCodeExists(gomock.Any(), gomock.Any()).
			Return(true, nil),
		mockLinkRepo.EXPECT().
			ShortCodeExists(gomock.Any(), gomock.Any()).
			Return(true, nil),
		mockLinkRepo.EXPECT().
			ShortCodeExists(gomock.Any(), gomock.Any()).
			Return(true, nil),
		mockLinkRepo.EXPECT().
			ShortCodeExists(gomock.Any(), gomock.Any()).
			Return(false, nil),
	)

	code, err := svc.Generate(context.Background())
	assert.NoError(t, err)
	assert.Len(t, code, 7)
}

func TestShortCodeServiceImpl_Generate_IncreasesLengthAfterMaxAttempts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewShortCodeService(mockLinkRepo)

	// Return true (exists) for all 7-char codes, false for 8-char
	mockLinkRepo.EXPECT().
		ShortCodeExists(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, code string) (bool, error) {
			return len(code) == 7, nil
		}).
		AnyTimes()

	code, err := svc.Generate(context.Background())
	assert.NoError(t, err)
	assert.Len(t, code, 8)
}

func TestShortCodeServiceImpl_Generate_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewShortCodeService(mockLinkRepo)

	mockLinkRepo.EXPECT().
		ShortCodeExists(gomock.Any(), gomock.Any()).
		Return(false, errors.New("database error"))

	code, err := svc.Generate(context.Background())
	assert.Error(t, err)
	assert.Empty(t, code)
}

func TestShortCodeServiceImpl_IsAvailable_Available(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewShortCodeService(mockLinkRepo)

	mockLinkRepo.EXPECT().
		ShortCodeExists(gomock.Any(), "abc123").
		Return(false, nil)

	available, err := svc.IsAvailable(context.Background(), "abc123")
	assert.NoError(t, err)
	assert.True(t, available)
}

func TestShortCodeServiceImpl_IsAvailable_NotAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewShortCodeService(mockLinkRepo)

	mockLinkRepo.EXPECT().
		ShortCodeExists(gomock.Any(), "taken1").
		Return(true, nil)

	available, err := svc.IsAvailable(context.Background(), "taken1")
	assert.NoError(t, err)
	assert.False(t, available)
}

func TestShortCodeServiceImpl_IsAvailable_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewShortCodeService(mockLinkRepo)

	mockLinkRepo.EXPECT().
		ShortCodeExists(gomock.Any(), "anycode").
		Return(false, errors.New("database error"))

	available, err := svc.IsAvailable(context.Background(), "anycode")
	assert.Error(t, err)
	assert.False(t, available)
}
```

### Step 2: Run tests

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/service/... -run TestShortCodeService -v`
Expected: All tests PASS

### Step 3: Commit

```bash
git add backend/internal/service/shortcode_service_test.go
git commit -m "refactor(test): update ShortCodeService tests to use mockgen"
```

---

## Task 5: Final Verification

### Step 1: Run all service tests

Run:
```bash
cd /Users/jose/GolandProjects/urlshortener/backend
go test ./internal/service/... -v
```
Expected: All tests PASS

### Step 2: Verify no DB tests remain

Run:
```bash
grep -r "testutil.SetupTestDB" /Users/jose/GolandProjects/urlshortener/backend/internal/service/
```
Expected: No matches found

### Step 3: Check test coverage

Run:
```bash
cd /Users/jose/GolandProjects/urlshortener/backend
go test ./internal/service/... -cover
```
Expected: Good coverage for all services

### Step 4: Final commit

```bash
git add -A
git commit -m "test(service): complete migration to mockgen-based tests"
```
