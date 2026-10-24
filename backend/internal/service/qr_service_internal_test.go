package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/SeaCodeBase/urlshortener/internal/model"
)

func TestNormalizeRenderCustomizations_Logo(t *testing.T) {
	for _, rc := range []model.JSONMap{
		{"logo": "/uploads/0123456789abcdef0123456789abcdef.png"},
		{"logo": "/uploads/0123456789abcdef0123456789abcdef.jpg"},
		{"logo": nil},
	} {
		_, err := normalizeRenderCustomizations(rc)
		assert.NoError(t, err, "%v", rc)
	}
	for _, rc := range []model.JSONMap{
		{"logo": "https://evil.example/x.png"},
		{"logo": "/uploads/0123456789abcdef0123456789abcdef.svg"},
		{"logo": "/uploads/../secret.png"},
		{"logo": "data:image/png;base64,AAAA"},
		{"logo": 7},
	} {
		_, err := normalizeRenderCustomizations(rc)
		assert.ErrorIs(t, err, ErrInvalidRenderCustomizations, "%v", rc)
	}
}
