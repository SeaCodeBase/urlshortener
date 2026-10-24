package service_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))))
	return buf.Bytes()
}

func newUploadService(t *testing.T) (service.UploadService, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "uploads")
	svc, err := service.NewUploadService(dir)
	require.NoError(t, err)
	return svc, dir
}

func TestUploadService_RejectsSVG(t *testing.T) {
	svc, dir := newUploadService(t)
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10"/></svg>`)
	_, err := svc.Save(context.Background(), 1, bytes.NewReader(data), int64(len(data)))
	assert.ErrorIs(t, err, service.ErrUploadType)
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries)
}

func TestUploadService_RejectsTooLarge(t *testing.T) {
	svc, dir := newUploadService(t)
	data := append(tinyPNG(t), bytes.Repeat([]byte{0}, service.MaxUploadBytes+1)...)
	// Declared size and unknown size (-1, read path) are both rejected.
	for _, size := range []int64{int64(len(data)), -1} {
		_, err := svc.Save(context.Background(), 1, bytes.NewReader(data), size)
		assert.ErrorIs(t, err, service.ErrUploadTooLarge, size)
	}
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries)
}

func TestUploadService_SavesPNG(t *testing.T) {
	svc, dir := newUploadService(t)
	data := tinyPNG(t)
	up, err := svc.Save(context.Background(), 1, bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{32}\.png$`), up.Name)
	assert.Equal(t, "/uploads/"+up.Name, up.URL)
	assert.Equal(t, "image/png", up.ContentType)
	assert.Equal(t, int64(len(data)), up.Size)

	saved, err := os.ReadFile(filepath.Join(dir, up.Name))
	require.NoError(t, err)
	assert.Equal(t, data, saved)
	info, err := os.Stat(filepath.Join(dir, up.Name))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}
