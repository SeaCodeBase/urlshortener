package service

import (
	"context"
	"errors"
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
)

// Compile-time check: AuthServiceImpl implements AuthService
var _ AuthService = (*AuthServiceImpl)(nil)

type AuthServiceImpl struct {
	userRepo  repository.UserRepository
	jwtSecret []byte
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string) *AuthServiceImpl {
	return &AuthServiceImpl{
		userRepo:  userRepo,
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
