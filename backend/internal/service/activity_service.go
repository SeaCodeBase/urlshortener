package service

import (
	"context"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

// Activity log actions (spec §5.11). Other features add their own
// "<entity>.<verb>" actions as they land.
const (
	ActionLinkCreate = "link.create"
	ActionLinkUpdate = "link.update"
	ActionLinkHide   = "link.hide"
	ActionLinkUnhide = "link.unhide"
	ActionLinkDelete = "link.delete"
	// ActionLinkBackHalf: a back-half edit created a new link.
	ActionLinkBackHalf = "link.back_half"
	ActionTokenCreate  = "token.create"
	ActionTokenRevoke  = "token.revoke"
	ActionDomainCreate = "domain.create"
	ActionDomainUpdate = "domain.update"
	ActionDomainVerify = "domain.verify"
	ActionDomainDelete = "domain.delete"
	ActionGroupCreate  = "group.create"
	ActionGroupUpdate  = "group.update"
	ActionQRCreate     = "qr.create"
	ActionQRUpdate     = "qr.update"
	ActionQRHide       = "qr.hide"
	ActionQRUnhide     = "qr.unhide"
	ActionQRDelete     = "qr.delete"
)

// Activity log entity types.
const (
	EntityLink   = "link"
	EntityToken  = "token"
	EntityDomain = "domain"
	EntityGroup  = "group"
	EntityQRCode = "qr_code"
)

const (
	DefaultActivityLimit = 20
	MaxActivityLimit     = 100
)

// ActivityListResult is one page of the activity log.
type ActivityListResult struct {
	Entries    []model.ActivityEntry `json:"entries"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	TotalPages int                   `json:"total_pages"`
}

// Compile-time check: activityService implements ActivityService
var _ ActivityService = (*activityService)(nil)

type activityService struct {
	repo repository.ActivityRepository
}

func NewActivityService(repo repository.ActivityRepository) ActivityService {
	return &activityService{repo: repo}
}

func (s *activityService) Log(ctx context.Context, userID, groupID uint64, action, entityType string, entityID uint64, meta map[string]any) {
	entry := &model.ActivityEntry{
		UserID:     userID,
		GroupID:    groupID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Metadata:   model.JSONMap(meta),
	}
	if err := s.repo.Insert(ctx, entry); err != nil {
		logger.Warn(ctx, "activity-service: failed to record activity",
			zap.Uint64("user_id", userID),
			zap.Uint64("group_id", groupID),
			zap.String("action", action),
			zap.Uint64("entity_id", entityID),
			zap.Error(err),
		)
	}
}

func (s *activityService) List(ctx context.Context, groupID uint64, from, to *time.Time, page, limit int) (*ActivityListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = DefaultActivityLimit
	}
	if limit > MaxActivityLimit {
		limit = MaxActivityLimit
	}
	entries, total, err := s.repo.List(ctx, groupID, from, to, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []model.ActivityEntry{}
	}
	return &ActivityListResult{
		Entries:    entries,
		Total:      total,
		Page:       page,
		TotalPages: int((total + int64(limit) - 1) / int64(limit)),
	}, nil
}
