package service

import (
	"context"
	"errors"
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

var (
	ErrGroupNotFound    = errors.New("group not found")
	ErrNotGroupOwner    = errors.New("group belongs to another user")
	ErrInvalidGroupName = errors.New("group name must not be empty")
	// ErrNotDomainOwner is also returned for a nonexistent domain ID so the
	// API does not reveal which domain IDs exist.
	ErrNotDomainOwner    = errors.New("domain belongs to another user")
	ErrDomainNotVerified = errors.New("domain is not verified")
)

// maxGroupNameLen matches groups.name VARCHAR(255).
const maxGroupNameLen = 255

// Compile-time check: GroupServiceImpl implements GroupService
var _ GroupService = (*GroupServiceImpl)(nil)

type GroupServiceImpl struct {
	groupRepo  repository.GroupRepository
	userRepo   repository.UserRepository
	domainRepo repository.DomainRepository
}

func NewGroupService(groupRepo repository.GroupRepository, userRepo repository.UserRepository, domainRepo repository.DomainRepository) *GroupServiceImpl {
	return &GroupServiceImpl{groupRepo: groupRepo, userRepo: userRepo, domainRepo: domainRepo}
}

// UpdateGroupInput is a partial update. ClearDefaultDomain wins over
// DefaultDomainID when both are set.
type UpdateGroupInput struct {
	Name               *string `json:"name"`
	DefaultDomainID    *uint64 `json:"default_domain_id"`
	ClearDefaultDomain bool    `json:"clear_default_domain"`
}

func (s *GroupServiceImpl) Create(ctx context.Context, userID uint64, name string) (*model.Group, error) {
	name, err := normalizeGroupName(name)
	if err != nil {
		return nil, err
	}
	group := &model.Group{UserID: userID, Name: name}
	if err := s.groupRepo.Create(ctx, group); err != nil {
		logger.Error(ctx, "group-service: failed to create group",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	return group, nil
}

func (s *GroupServiceImpl) List(ctx context.Context, userID uint64) ([]model.Group, error) {
	groups, err := s.groupRepo.ListByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "group-service: failed to list groups",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	return groups, nil
}

func (s *GroupServiceImpl) Get(ctx context.Context, userID, groupID uint64) (*model.Group, error) {
	return s.getOwned(ctx, userID, groupID)
}

func (s *GroupServiceImpl) Update(ctx context.Context, userID, groupID uint64, input UpdateGroupInput) (*model.Group, error) {
	group, err := s.getOwned(ctx, userID, groupID)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		name, err := normalizeGroupName(*input.Name)
		if err != nil {
			return nil, err
		}
		group.Name = name
	}
	if input.ClearDefaultDomain {
		group.DefaultDomainID = nil
	} else if input.DefaultDomainID != nil {
		if err := s.checkUsableDomain(ctx, userID, *input.DefaultDomainID); err != nil {
			return nil, err
		}
		id := *input.DefaultDomainID
		group.DefaultDomainID = &id
	}
	if err := s.groupRepo.Update(ctx, group); err != nil {
		if errors.Is(err, repository.ErrGroupNotFound) {
			return nil, ErrGroupNotFound
		}
		logger.Error(ctx, "group-service: failed to update group",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return nil, err
	}
	return group, nil
}

func (s *GroupServiceImpl) ResolveActive(ctx context.Context, userID uint64, headerGroupID *uint64) (*model.Group, error) {
	if headerGroupID != nil {
		return s.getOwned(ctx, userID, *headerGroupID)
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "group-service: failed to load user for active group",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	if user.DefaultGroupID != nil {
		group, err := s.getOwned(ctx, userID, *user.DefaultGroupID)
		if !errors.Is(err, ErrGroupNotFound) {
			return group, err
		}
		// The default group no longer exists; fall through and recreate it.
	}
	return s.createDefaultGroupIfUnset(ctx, user)
}

// createDefaultGroupIfUnset lazily creates the user's default group. The
// user row is only updated if default_group_id still holds the value we read,
// so concurrent requests cannot orphan groups: the loser deletes its own
// group and returns the winner's.
func (s *GroupServiceImpl) createDefaultGroupIfUnset(ctx context.Context, user *model.User) (*model.Group, error) {
	group := &model.Group{UserID: user.ID, Name: defaultGroupName(user.Email)}
	if err := s.groupRepo.Create(ctx, group); err != nil {
		logger.Error(ctx, "group-service: failed to create default group",
			zap.Uint64("user_id", user.ID),
			zap.Error(err),
		)
		return nil, err
	}
	won, err := s.userRepo.SetDefaultGroupIfUnset(ctx, user.ID, group.ID, user.DefaultGroupID)
	if err != nil {
		logger.Error(ctx, "group-service: failed to set default group",
			zap.Uint64("user_id", user.ID),
			zap.Uint64("group_id", group.ID),
			zap.Error(err),
		)
		return nil, err
	}
	if won {
		return group, nil
	}

	// Lost the race: drop our group and use the one that was stored.
	if err := s.groupRepo.Delete(ctx, group.ID); err != nil && !errors.Is(err, repository.ErrGroupNotFound) {
		logger.Error(ctx, "group-service: failed to delete losing default group",
			zap.Uint64("user_id", user.ID),
			zap.Uint64("group_id", group.ID),
			zap.Error(err),
		)
		return nil, err
	}
	current, err := s.userRepo.GetByID(ctx, user.ID)
	if err != nil {
		logger.Error(ctx, "group-service: failed to reload user after default group race",
			zap.Uint64("user_id", user.ID),
			zap.Error(err),
		)
		return nil, err
	}
	if current.DefaultGroupID == nil {
		return nil, ErrGroupNotFound
	}
	return s.getOwned(ctx, user.ID, *current.DefaultGroupID)
}

// checkUsableDomain requires the domain to exist, belong to userID and be
// verified before it can become a group's default domain.
func (s *GroupServiceImpl) checkUsableDomain(ctx context.Context, userID, domainID uint64) error {
	domain, err := s.domainRepo.GetByID(ctx, domainID)
	if errors.Is(err, repository.ErrDomainNotFound) {
		return ErrNotDomainOwner
	}
	if err != nil {
		logger.Error(ctx, "group-service: failed to get domain",
			zap.Uint64("domain_id", domainID),
			zap.Error(err),
		)
		return err
	}
	if domain.UserID != userID {
		return ErrNotDomainOwner
	}
	if domain.VerificationStatus != model.DomainVerificationOK {
		return ErrDomainNotVerified
	}
	return nil
}

// getOwned loads a group and enforces that it belongs to userID.
func (s *GroupServiceImpl) getOwned(ctx context.Context, userID, groupID uint64) (*model.Group, error) {
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if errors.Is(err, repository.ErrGroupNotFound) {
		return nil, ErrGroupNotFound
	}
	if err != nil {
		logger.Error(ctx, "group-service: failed to get group",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return nil, err
	}
	if group.UserID != userID {
		return nil, ErrNotGroupOwner
	}
	return group, nil
}

// createDefaultGroup creates a group named after the user's email local part
// and records it as the user's default group. Used on registration, where the
// user row is brand new and nobody else can race on default_group_id.
func createDefaultGroup(ctx context.Context, groupRepo repository.GroupRepository, userRepo repository.UserRepository, user *model.User) (*model.Group, error) {
	group := &model.Group{UserID: user.ID, Name: defaultGroupName(user.Email)}
	if err := groupRepo.Create(ctx, group); err != nil {
		logger.Error(ctx, "group-service: failed to create default group",
			zap.Uint64("user_id", user.ID),
			zap.Error(err),
		)
		return nil, err
	}
	if err := userRepo.UpdateDefaultGroup(ctx, user.ID, group.ID); err != nil {
		logger.Error(ctx, "group-service: failed to set default group",
			zap.Uint64("user_id", user.ID),
			zap.Uint64("group_id", group.ID),
			zap.Error(err),
		)
		return nil, err
	}
	return group, nil
}

// defaultGroupName mirrors the migration 002 backfill:
// SUBSTRING_INDEX(email, '@', 1).
func defaultGroupName(email string) string {
	name, _, _ := strings.Cut(email, "@")
	if name == "" {
		name = email
	}
	return truncateRunes(name, maxGroupNameLen)
}

func normalizeGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalidGroupName
	}
	return truncateRunes(name, maxGroupNameLen), nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
