package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jose/urlshortener/internal/service"
)

// mockShortCodeRepository is a mock implementation of ShortCodeRepository for testing.
type mockShortCodeRepository struct {
	existsCodes map[string]bool
	err         error
	callCount   int
}

func newMockRepo() *mockShortCodeRepository {
	return &mockShortCodeRepository{
		existsCodes: make(map[string]bool),
	}
}

func (m *mockShortCodeRepository) ShortCodeExists(ctx context.Context, code string) (bool, error) {
	m.callCount++
	if m.err != nil {
		return false, m.err
	}
	return m.existsCodes[code], nil
}

func TestShortCodeService_IsValid(t *testing.T) {
	svc := service.NewShortCodeService(newMockRepo())

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
			if got := svc.IsValid(tt.code); got != tt.valid {
				t.Errorf("IsValid(%q) = %v, want %v", tt.code, got, tt.valid)
			}
		})
	}
}

func TestShortCodeService_Generate(t *testing.T) {
	t.Run("generates unique code on first attempt", func(t *testing.T) {
		mockRepo := newMockRepo()
		svc := service.NewShortCodeService(mockRepo)

		code, err := svc.Generate(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(code) != 7 {
			t.Errorf("expected code length 7, got %d", len(code))
		}
		if mockRepo.callCount != 1 {
			t.Errorf("expected 1 call to ShortCodeExists, got %d", mockRepo.callCount)
		}
	})

	t.Run("retries when code exists", func(t *testing.T) {
		// Create a custom mock that returns exists for first 3 calls
		customMock := &countingMockRepo{existsUntil: 3}
		svc := service.NewShortCodeService(customMock)
		code, err := svc.Generate(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(code) != 7 {
			t.Errorf("expected code length 7, got %d", len(code))
		}
		if customMock.callCount != 4 {
			t.Errorf("expected 4 calls to ShortCodeExists, got %d", customMock.callCount)
		}
	})

	t.Run("increases length after max attempts", func(t *testing.T) {
		// Mock that always returns exists for 7-char codes
		customMock := &lengthBasedMockRepo{existsForLength: 7}
		svc := service.NewShortCodeService(customMock)

		code, err := svc.Generate(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(code) != 8 {
			t.Errorf("expected code length 8 after exhausting attempts, got %d", len(code))
		}
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		mockRepo := newMockRepo()
		mockRepo.err = errors.New("database error")
		svc := service.NewShortCodeService(mockRepo)

		_, err := svc.Generate(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "database error" {
			t.Errorf("expected 'database error', got %q", err.Error())
		}
	})
}

func TestShortCodeService_IsAvailable(t *testing.T) {
	t.Run("returns true when code does not exist", func(t *testing.T) {
		mockRepo := newMockRepo()
		svc := service.NewShortCodeService(mockRepo)

		available, err := svc.IsAvailable(context.Background(), "abc123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !available {
			t.Error("expected code to be available")
		}
	})

	t.Run("returns false when code exists", func(t *testing.T) {
		mockRepo := newMockRepo()
		mockRepo.existsCodes["existing"] = true
		svc := service.NewShortCodeService(mockRepo)

		available, err := svc.IsAvailable(context.Background(), "existing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if available {
			t.Error("expected code to be unavailable")
		}
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		mockRepo := newMockRepo()
		mockRepo.err = errors.New("database error")
		svc := service.NewShortCodeService(mockRepo)

		_, err := svc.IsAvailable(context.Background(), "anycode")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// countingMockRepo returns exists=true for the first N calls, then exists=false
type countingMockRepo struct {
	callCount   int
	existsUntil int
}

func (m *countingMockRepo) ShortCodeExists(ctx context.Context, code string) (bool, error) {
	m.callCount++
	return m.callCount <= m.existsUntil, nil
}

// lengthBasedMockRepo returns exists=true for codes of a specific length
type lengthBasedMockRepo struct {
	callCount       int
	existsForLength int
}

func (m *lengthBasedMockRepo) ShortCodeExists(ctx context.Context, code string) (bool, error) {
	m.callCount++
	return len(code) == m.existsForLength, nil
}
