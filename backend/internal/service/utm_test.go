package service_test

import (
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyUTM_AppendsAndPreservesFragment(t *testing.T) {
	got, err := service.ApplyUTM("https://example.com/path?a=1&b=two#section", service.UTMParams{
		Source:   "newsletter",
		Medium:   "email",
		Campaign: "spring sale",
		Custom:   []service.KV{{Key: "ref", Value: "x&y"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/path?a=1&b=two&utm_source=newsletter&utm_medium=email&utm_campaign=spring+sale&ref=x%26y#section", got)
}

func TestApplyUTM_OverwritesExisting(t *testing.T) {
	got, err := service.ApplyUTM("https://example.com/?utm_source=old&keep=1&utm_source=dup&utm_term=t", service.UTMParams{
		Source:  "new",
		Content: "banner",
		Custom:  []service.KV{{Key: "keep", Value: "2"}},
	})
	require.NoError(t, err)
	// utm_term is untouched because Term is empty; duplicates of an overwritten key are dropped.
	assert.Equal(t, "https://example.com/?utm_source=new&keep=2&utm_term=t&utm_content=banner", got)
}

func TestApplyUTM_NoParamsReturnsURLUnchanged(t *testing.T) {
	got, err := service.ApplyUTM("https://example.com/a?x=1#f", service.UTMParams{})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/a?x=1#f", got)
}

func TestApplyUTM_Errors(t *testing.T) {
	_, err := service.ApplyUTM("not a url", service.UTMParams{Source: "s"})
	assert.ErrorIs(t, err, service.ErrInvalidURL)

	_, err = service.ApplyUTM("https://example.com", service.UTMParams{Custom: []service.KV{{Key: "  ", Value: "v"}}})
	assert.ErrorIs(t, err, service.ErrInvalidUTMParam)
}
