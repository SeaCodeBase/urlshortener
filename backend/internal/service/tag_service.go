package service

import (
	"context"
	"errors"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
)

// Tag entity types accepted by TagService.ListByGroup.
const (
	TagTypeLink = "link"
	TagTypeQR   = "qr"
)

var ErrInvalidTagType = errors.New("invalid tag type")

// Compile-time check: tagService implements TagService
var _ TagService = (*tagService)(nil)

type tagService struct {
	tagRepo repository.TagRepository
	qrRepo  repository.QRRepository
}

func NewTagService(tagRepo repository.TagRepository, qrRepo repository.QRRepository) TagService {
	return &tagService{tagRepo: tagRepo, qrRepo: qrRepo}
}

func (s *tagService) ListByGroup(ctx context.Context, groupID uint64, typ string) ([]string, error) {
	var tags []string
	var err error
	switch typ {
	case "", TagTypeLink:
		tags, err = s.tagRepo.ListByGroup(ctx, groupID)
	case TagTypeQR:
		tags, err = s.qrRepo.ListTagsByGroup(ctx, groupID)
	default:
		return nil, ErrInvalidTagType
	}
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}
