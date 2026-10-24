package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// MaxUploadBytes is the largest accepted upload.
const MaxUploadBytes = 5 << 20

var (
	ErrUploadTooLarge = errors.New("file must be 5 MB or smaller")
	ErrUploadType     = errors.New("only PNG and JPEG images are allowed")
)

// uploadExt maps the accepted sniffed content types to file extensions.
var uploadExt = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
}

// Upload is a stored file, served at URL.
type Upload struct {
	Name        string
	URL         string
	ContentType string
	Size        int64
}

// UploadService stores user images (QR code logos) on disk.
type UploadService interface {
	// Save stores r if it is a PNG or JPEG of at most MaxUploadBytes;
	// ErrUploadTooLarge, ErrUploadType. size is the declared size (advisory).
	Save(ctx context.Context, userID uint64, r io.Reader, size int64) (*Upload, error)
}

type uploadService struct {
	dir string
}

// NewUploadService stores uploads in dir, creating it if needed.
func NewUploadService(dir string) (UploadService, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	return &uploadService{dir: dir}, nil
}

func (s *uploadService) Save(_ context.Context, _ uint64, r io.Reader, size int64) (*Upload, error) {
	if size > MaxUploadBytes {
		return nil, ErrUploadTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if len(data) > MaxUploadBytes {
		return nil, ErrUploadTooLarge
	}
	contentType := http.DetectContentType(data)
	ext, ok := uploadExt[contentType]
	if !ok {
		return nil, ErrUploadType
	}

	var rnd [16]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return nil, fmt.Errorf("generate upload name: %w", err)
	}
	name := hex.EncodeToString(rnd[:]) + ext
	if err := writeFileAtomic(filepath.Join(s.dir, name), data); err != nil {
		return nil, err
	}
	return &Upload{Name: name, URL: "/uploads/" + name, ContentType: contentType, Size: int64(len(data))}, nil
}

// writeFileAtomic writes data to a temp file in the same directory and renames
// it into place so a partial file is never served.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return fmt.Errorf("create upload file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write upload file: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod upload file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close upload file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("store upload file: %w", err)
	}
	return nil
}
