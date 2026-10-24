package qrimage_test

import (
	"bytes"
	"errors"
	"image/png"
	"strings"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/qrimage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQRImage_PNG_DecodesToSize(t *testing.T) {
	data, err := qrimage.PNG("https://sho.rt/abc?r=qr", qrimage.Options{Size: 256})
	require.NoError(t, err)
	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, 256, img.Bounds().Dx())
	assert.Equal(t, 256, img.Bounds().Dy())
}

func TestQRImage_SVG_UsesColors(t *testing.T) {
	data, err := qrimage.SVG("https://sho.rt/abc?r=qr", qrimage.Options{Size: 300, Foreground: "#2A5BD7", Background: "#FFFFFF", Level: "M"})
	require.NoError(t, err)
	s := string(data)
	assert.True(t, strings.HasPrefix(s, "<svg"), s[:min(len(s), 40)])
	assert.Contains(t, s, `fill="#2A5BD7"`)
	assert.Contains(t, s, `fill="#FFFFFF"`)
	assert.Contains(t, s, `width="300"`)
	assert.Contains(t, s, `height="300"`)
	assert.Contains(t, s, `shape-rendering="crispEdges"`)
	assert.Contains(t, s, `viewBox="0 0 `)
	assert.Equal(t, 1, strings.Count(s, "<rect"))
	assert.Equal(t, 1, strings.Count(s, "<path"))
}

func TestQRImage_InvalidColor(t *testing.T) {
	for _, o := range []qrimage.Options{{Foreground: "red"}, {Background: "#FFF"}, {Foreground: "#GGGGGG"}} {
		_, err := qrimage.PNG("x", o)
		assert.True(t, errors.Is(err, qrimage.ErrInvalidColor), "%+v: %v", o, err)
		_, err = qrimage.SVG("x", o)
		assert.True(t, errors.Is(err, qrimage.ErrInvalidColor), "%+v: %v", o, err)
	}
}

func TestQRImage_InvalidLevel(t *testing.T) {
	_, err := qrimage.PNG("x", qrimage.Options{Level: "X"})
	assert.ErrorIs(t, err, qrimage.ErrInvalidLevel)
	_, err = qrimage.SVG("x", qrimage.Options{Level: "low"})
	assert.ErrorIs(t, err, qrimage.ErrInvalidLevel)
}

func TestQRImage_InvalidSize(t *testing.T) {
	for _, size := range []int{63, 2049, -1} {
		_, err := qrimage.PNG("x", qrimage.Options{Size: size})
		assert.ErrorIs(t, err, qrimage.ErrInvalidSize, size)
	}
}
