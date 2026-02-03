package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

var (
	ErrLinkNotFound     = errors.New("link not found")
	ErrNotLinkOwner     = errors.New("not the owner of this link")
	ErrInvalidShortCode = errors.New("invalid short code")
	ErrShortCodeTaken   = errors.New("short code already taken")
)

const maxPageSize = 100

// Compile-time check: LinkServiceImpl implements LinkService
var _ LinkService = (*LinkServiceImpl)(nil)

type LinkServiceImpl struct {
	linkRepo  repository.LinkRepository
	shortCode ShortCodeService
}

func NewLinkService(linkRepo repository.LinkRepository, shortCode ShortCodeService) *LinkServiceImpl {
	return &LinkServiceImpl{
		linkRepo:  linkRepo,
		shortCode: shortCode,
	}
}

type CreateLinkInput struct {
	OriginalURL string     `json:"original_url" binding:"required,url"`
	CustomCode  string     `json:"custom_code,omitempty"`
	Title       string     `json:"title,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type UpdateLinkInput struct {
	OriginalURL string     `json:"original_url,omitempty"`
	Title       string     `json:"title,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsActive    *bool      `json:"is_active,omitempty"`
}

type ListLinksParams struct {
	Page  int
	Limit int
}

type ListLinksResult struct {
	Links      []model.Link `json:"links"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	TotalPages int          `json:"total_pages"`
}

func (s *LinkServiceImpl) Create(ctx context.Context, userID uint64, input CreateLinkInput) (*model.Link, error) {
	var shortCode string
	var err error

	if input.CustomCode != "" {
		if !s.shortCode.IsValid(input.CustomCode) {
			return nil, ErrInvalidShortCode
		}
		available, err := s.shortCode.IsAvailable(ctx, input.CustomCode)
		if err != nil {
			logger.Error(ctx, "link-service: failed to check code availability",
				zap.Uint64("user_id", userID),
				zap.String("custom_code", input.CustomCode),
				zap.Error(err),
			)
			return nil, err
		}
		if !available {
			return nil, ErrShortCodeTaken
		}
		shortCode = input.CustomCode
	} else {
		shortCode, err = s.shortCode.Generate(ctx)
		if err != nil {
			logger.Error(ctx, "link-service: failed to generate short code",
				zap.Uint64("user_id", userID),
				zap.Error(err),
			)
			return nil, err
		}
	}

	link := &model.Link{
		UserID:      userID,
		ShortCode:   shortCode,
		OriginalURL: input.OriginalURL,
		IsActive:    true,
	}

	if input.Title != "" {
		link.Title = &input.Title
	}

	if input.ExpiresAt != nil {
		link.ExpiresAt = model.NullTime{NullTime: sql.NullTime{Time: *input.ExpiresAt, Valid: true}}
	}

	if err := s.linkRepo.Create(ctx, link); err != nil {
		if errors.Is(err, repository.ErrShortCodeExists) {
			return nil, ErrShortCodeTaken
		}
		logger.Error(ctx, "link-service: failed to create link",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	return link, nil
}

func (s *LinkServiceImpl) GetByID(ctx context.Context, userID, linkID uint64) (*model.Link, error) {
	link, err := s.linkRepo.GetByID(ctx, linkID)
	if errors.Is(err, repository.ErrLinkNotFound) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		logger.Error(ctx, "link-service: failed to get link by ID",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	if link.UserID != userID {
		return nil, ErrNotLinkOwner
	}

	return link, nil
}

func (s *LinkServiceImpl) List(ctx context.Context, userID uint64, params ListLinksParams) (*ListLinksResult, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	} else if params.Limit > maxPageSize {
		params.Limit = maxPageSize
	}
	if params.Page <= 0 {
		params.Page = 1
	}

	offset := (params.Page - 1) * params.Limit

	links, err := s.linkRepo.ListByUserID(ctx, userID, params.Limit, offset)
	if err != nil {
		logger.Error(ctx, "link-service: failed to list links",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	total, err := s.linkRepo.CountByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "link-service: failed to count links",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	totalPages := int(total) / params.Limit
	if int(total)%params.Limit > 0 {
		totalPages++
	}

	return &ListLinksResult{
		Links:      links,
		Total:      total,
		Page:       params.Page,
		TotalPages: totalPages,
	}, nil
}

func (s *LinkServiceImpl) Update(ctx context.Context, userID, linkID uint64, input UpdateLinkInput) (*model.Link, error) {
	link, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return nil, err
	}

	if input.OriginalURL != "" {
		link.OriginalURL = input.OriginalURL
	}
	if input.Title != "" {
		link.Title = &input.Title
	}
	if input.ExpiresAt != nil {
		link.ExpiresAt = model.NullTime{NullTime: sql.NullTime{Time: *input.ExpiresAt, Valid: true}}
	}
	if input.IsActive != nil {
		link.IsActive = *input.IsActive
	}

	if err := s.linkRepo.Update(ctx, link); err != nil {
		logger.Error(ctx, "link-service: failed to update link",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	return link, nil
}

func (s *LinkServiceImpl) Delete(ctx context.Context, userID, linkID uint64) error {
	link, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return err
	}

	if err := s.linkRepo.Delete(ctx, link.ID); err != nil {
		logger.Error(ctx, "link-service: failed to delete link",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	return nil
}
