package service

import (
	"context"
	"crypto/rand"
	"math/big"

	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

const (
	// Base62 alphabet for short codes
	alphabet            = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	defaultLen          = 7
	maxGenerateAttempts = 10
)

// ShortCodeRepository defines the interface for short code existence checks.
type ShortCodeRepository interface {
	ShortCodeExistsInDomain(ctx context.Context, domainID *uint64, code string) (bool, error)
}

// Compile-time check: ShortCodeServiceImpl implements ShortCodeService
var _ ShortCodeService = (*ShortCodeServiceImpl)(nil)

type ShortCodeServiceImpl struct {
	linkRepo ShortCodeRepository
}

func NewShortCodeService(linkRepo ShortCodeRepository) *ShortCodeServiceImpl {
	return &ShortCodeServiceImpl{linkRepo: linkRepo}
}

func (s *ShortCodeServiceImpl) Generate(ctx context.Context, domainID *uint64) (string, error) {
	return s.generateWithLength(ctx, domainID, defaultLen)
}

func (s *ShortCodeServiceImpl) generateWithLength(ctx context.Context, domainID *uint64, length int) (string, error) {
	for attempts := 0; attempts < maxGenerateAttempts; attempts++ {
		code, err := generateRandomCode(length)
		if err != nil {
			logger.Error(ctx, "shortcode-service: failed to generate random bytes",
				zap.Error(err),
			)
			return "", err
		}

		exists, err := s.linkRepo.ShortCodeExistsInDomain(ctx, domainID, code)
		if err != nil {
			logger.Error(ctx, "shortcode-service: failed to check code availability",
				zap.String("code", code),
				zap.Error(err),
			)
			return "", err
		}
		if !exists {
			return code, nil
		}
	}

	// If we exhausted attempts, try with longer code (recursively check for collisions)
	return s.generateWithLength(ctx, domainID, length+1)
}

func (s *ShortCodeServiceImpl) IsValid(code string) bool {
	if len(code) < 3 || len(code) > 16 {
		return false
	}
	for _, c := range code {
		if !isAlphanumeric(c) {
			return false
		}
	}
	return true
}

func (s *ShortCodeServiceImpl) IsAvailable(ctx context.Context, domainID *uint64, code string) (bool, error) {
	exists, err := s.linkRepo.ShortCodeExistsInDomain(ctx, domainID, code)
	if err != nil {
		logger.Error(ctx, "shortcode-service: failed to check code availability",
			zap.String("code", code),
			zap.Error(err),
		)
		return false, err
	}
	return !exists, nil
}

func generateRandomCode(length int) (string, error) {
	result := make([]byte, length)
	alphabetLen := big.NewInt(int64(len(alphabet)))

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", err
		}
		result[i] = alphabet[num.Int64()]
	}

	return string(result), nil
}

func isAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
