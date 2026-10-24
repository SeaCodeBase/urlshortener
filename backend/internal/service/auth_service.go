package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already taken")
	ErrWrongPassword      = errors.New("current password is incorrect")
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrInvalidAccountName = errors.New("account name must not be empty")
	ErrSameEmail          = errors.New("new email is the same as the current email")
)

// emailPattern is a deliberately simple local@domain.tld check, in the
// spirit of the registration form's binding:"email".
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@.]+(\.[^\s@.]+)+$`)

// maxAccountNameLen matches users.account_name VARCHAR(255).
const maxAccountNameLen = 255

// UpdateAccountInput is a partial profile/account update: nil fields are
// left unchanged.
type UpdateAccountInput struct {
	DisplayName            *string `json:"display_name"`
	AccountName            *string `json:"account_name"`
	DefaultGroupID         *uint64 `json:"default_group_id"`
	HomeChecklistDismissed *bool   `json:"home_checklist_dismissed"`
}

// Compile-time check: AuthServiceImpl implements AuthService
var _ AuthService = (*AuthServiceImpl)(nil)

type AuthServiceImpl struct {
	userRepo  repository.UserRepository
	groupRepo repository.GroupRepository
	jwtSecret []byte
}

func NewAuthService(userRepo repository.UserRepository, groupRepo repository.GroupRepository, jwtSecret string) *AuthServiceImpl {
	return &AuthServiceImpl{
		userRepo:  userRepo,
		groupRepo: groupRepo,
		jwtSecret: []byte(jwtSecret),
	}
}

type RegisterInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required,min=8"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

func (s *AuthServiceImpl) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	exists, err := s.userRepo.EmailExists(ctx, input.Email)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to check email existence",
			zap.String("email", input.Email),
			zap.Error(err),
		)
		return nil, err
	}
	if exists {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to hash password",
			zap.Error(err),
		)
		return nil, err
	}

	user := &model.User{
		Email:        input.Email,
		PasswordHash: string(hash),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		logger.Error(ctx, "auth-service: failed to create user",
			zap.String("email", input.Email),
			zap.Error(err),
		)
		return nil, err
	}

	group, err := createDefaultGroup(ctx, s.groupRepo, s.userRepo, user)
	if err != nil {
		return nil, err
	}
	user.DefaultGroupID = &group.ID

	token, err := s.generateToken(user.ID)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to generate token",
			zap.Uint64("user_id", user.ID),
			zap.Error(err),
		)
		return nil, err
	}

	return &AuthResponse{Token: token, User: user}, nil
}

func (s *AuthServiceImpl) Login(ctx context.Context, input LoginInput) (*AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		logger.Error(ctx, "auth-service: failed to get user by email",
			zap.String("email", input.Email),
			zap.Error(err),
		)
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to generate token",
			zap.Uint64("user_id", user.ID),
			zap.Error(err),
		)
		return nil, err
	}

	return &AuthResponse{Token: token, User: user}, nil
}

func (s *AuthServiceImpl) GetUserByID(ctx context.Context, userID uint64) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to get user by ID",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	return user, nil
}

func (s *AuthServiceImpl) ChangePassword(ctx context.Context, userID uint64, input ChangePasswordInput) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to get user for password change",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.OldPassword)); err != nil {
		return ErrWrongPassword
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to hash new password",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, string(newHash)); err != nil {
		logger.Error(ctx, "auth-service: failed to update password",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	return nil
}

func (s *AuthServiceImpl) UpdateDisplayName(ctx context.Context, userID uint64, displayName string) error {
	if err := s.userRepo.UpdateDisplayName(ctx, userID, displayName); err != nil {
		logger.Error(ctx, "auth-service: failed to update display name",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	return nil
}

func (s *AuthServiceImpl) UpdateEmail(ctx context.Context, userID uint64, password, newEmail string) error {
	newEmail = strings.TrimSpace(newEmail)
	if !validEmail(newEmail) {
		return ErrInvalidEmail
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to get user for email change",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return ErrWrongPassword
	}
	if strings.EqualFold(newEmail, user.Email) {
		return ErrSameEmail
	}
	exists, err := s.userRepo.EmailExists(ctx, newEmail)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to check email existence",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	if exists {
		return ErrEmailTaken
	}
	if err := s.userRepo.UpdateEmail(ctx, userID, newEmail); err != nil {
		if errors.Is(err, repository.ErrEmailExists) {
			return ErrEmailTaken
		}
		logger.Error(ctx, "auth-service: failed to update email",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	return nil
}

// validEmail accepts a bare local@domain.tld address of at most 255 bytes.
func validEmail(email string) bool {
	return len(email) <= 255 && emailPattern.MatchString(email)
}

func (s *AuthServiceImpl) UpdateAccount(ctx context.Context, userID uint64, input UpdateAccountInput) error {
	var accountName *string
	if input.AccountName != nil {
		name := strings.TrimSpace(*input.AccountName)
		if name == "" {
			return ErrInvalidAccountName
		}
		name = truncateRunes(name, maxAccountNameLen)
		accountName = &name
	}
	if input.DefaultGroupID != nil {
		group, err := s.groupRepo.GetByID(ctx, *input.DefaultGroupID)
		if errors.Is(err, repository.ErrGroupNotFound) {
			return ErrGroupNotFound
		}
		if err != nil {
			logger.Error(ctx, "auth-service: failed to get group",
				zap.Uint64("group_id", *input.DefaultGroupID),
				zap.Error(err),
			)
			return err
		}
		if group.UserID != userID {
			return ErrGroupNotFound
		}
	}

	if input.DisplayName != nil {
		if err := s.UpdateDisplayName(ctx, userID, *input.DisplayName); err != nil {
			return err
		}
	}
	if accountName != nil || input.HomeChecklistDismissed != nil {
		if err := s.userRepo.UpdateAccount(ctx, userID, accountName, input.HomeChecklistDismissed); err != nil {
			logger.Error(ctx, "auth-service: failed to update account",
				zap.Uint64("user_id", userID),
				zap.Error(err),
			)
			return err
		}
	}
	if input.DefaultGroupID != nil {
		if err := s.userRepo.UpdateDefaultGroup(ctx, userID, *input.DefaultGroupID); err != nil {
			logger.Error(ctx, "auth-service: failed to update default group",
				zap.Uint64("user_id", userID),
				zap.Error(err),
			)
			return err
		}
	}
	return nil
}

func (s *AuthServiceImpl) GenerateToken(userID uint64) (string, error) {
	return s.generateToken(userID)
}

func (s *AuthServiceImpl) generateToken(userID uint64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthServiceImpl) ValidateToken(tokenString string) (uint64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, errors.New("invalid token")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errors.New("invalid token: missing user_id")
	}

	return uint64(userIDFloat), nil
}
