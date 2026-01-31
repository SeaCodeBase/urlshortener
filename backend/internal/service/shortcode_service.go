package service

import (
	"context"
	"crypto/rand"
	"math/big"
)

const (
	// Base62 alphabet for short codes
	alphabet            = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	defaultLen          = 7
	maxGenerateAttempts = 10
)

// ShortCodeRepository defines the interface for short code existence checks.
type ShortCodeRepository interface {
	ShortCodeExists(ctx context.Context, code string) (bool, error)
}

type ShortCodeService struct {
	linkRepo ShortCodeRepository
}

func NewShortCodeService(linkRepo ShortCodeRepository) *ShortCodeService {
	return &ShortCodeService{linkRepo: linkRepo}
}

func (s *ShortCodeService) Generate(ctx context.Context) (string, error) {
	return s.generateWithLength(ctx, defaultLen)
}

func (s *ShortCodeService) generateWithLength(ctx context.Context, length int) (string, error) {
	for attempts := 0; attempts < maxGenerateAttempts; attempts++ {
		code, err := generateRandomCode(length)
		if err != nil {
			return "", err
		}

		exists, err := s.linkRepo.ShortCodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}

	// If we exhausted attempts, try with longer code (recursively check for collisions)
	return s.generateWithLength(ctx, length+1)
}

func (s *ShortCodeService) IsValid(code string) bool {
	if len(code) < 4 || len(code) > 16 {
		return false
	}
	for _, c := range code {
		if !isAlphanumeric(c) {
			return false
		}
	}
	return true
}

func (s *ShortCodeService) IsAvailable(ctx context.Context, code string) (bool, error) {
	exists, err := s.linkRepo.ShortCodeExists(ctx, code)
	return !exists, err
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
