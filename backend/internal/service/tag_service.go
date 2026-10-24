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
}

func NewTagService(tagRepo repository.TagRepository) TagService {
	return &tagService{tagRepo: tagRepo}
}

func (s *tagService) ListByGroup(ctx context.Context, groupID uint64, typ string) ([]string, error) {
	switch typ {
	case "", TagTypeLink:
		tags, err := s.tagRepo.ListByGroup(ctx, groupID)
		if err != nil {
			return nil, err
		}
		if tags == nil {
			tags = []string{}
		}
		return tags, nil
	case TagTypeQR:
		// QR codes arrive in phase 2; until then they have no tags.
		return []string{}, nil
	default:
		return nil, ErrInvalidTagType
	}
}
