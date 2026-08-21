package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type StoredFile struct {
	Key         string
	Size        int64
	ContentType string
}

type AttachmentStore interface {
	Save(ctx context.Context, fileName, contentType string, reader io.Reader) (StoredFile, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

type LocalAttachmentStore struct {
	root       string
	maxBytes   int64
	allowTypes map[string]struct{}
}

func NewLocalAttachmentStore(root string, maxBytes int64) (*LocalAttachmentStore, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("max attachment size must be positive")
	}
	clean := filepath.Clean(root)
	if err := os.MkdirAll(clean, 0o750); err != nil {
		return nil, fmt.Errorf("create attachment root: %w", err)
	}
	return &LocalAttachmentStore{
		root:     clean,
		maxBytes: maxBytes,
		allowTypes: map[string]struct{}{
			"image/jpeg":      {},
			"image/png":       {},
			"application/pdf": {},
			"text/plain":      {},
		},
	}, nil
}

func (s *LocalAttachmentStore) Save(ctx context.Context, fileName, contentType string, reader io.Reader) (StoredFile, error) {
	if err := ctx.Err(); err != nil {
		return StoredFile{}, err
	}
	if _, ok := s.allowTypes[contentType]; !ok {
		return StoredFile{}, fmt.Errorf("unsupported attachment content type: %s", contentType)
	}
	name := filepath.Base(strings.TrimSpace(fileName))
	if name == "." || name == "" {
		return StoredFile{}, fmt.Errorf("invalid attachment name")
	}
	temporary, err := os.CreateTemp(s.root, "upload-*")
	if err != nil {
		return StoredFile{}, fmt.Errorf("create attachment: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(reader, s.maxBytes+1))
	if err != nil {
		return StoredFile{}, fmt.Errorf("write attachment: %w", err)
	}
	if written > s.maxBytes {
		return StoredFile{}, fmt.Errorf("attachment exceeds %d bytes", s.maxBytes)
	}
	if err := temporary.Sync(); err != nil {
		return StoredFile{}, fmt.Errorf("sync attachment: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return StoredFile{}, fmt.Errorf("close attachment: %w", err)
	}
	key := hex.EncodeToString(hash.Sum(nil)) + filepath.Ext(name)
	destination := filepath.Join(s.root, key)
	if _, err := os.Stat(destination); err == nil {
		return StoredFile{Key: key, Size: written, ContentType: contentType}, nil
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return StoredFile{}, fmt.Errorf("commit attachment: %w", err)
	}
	return StoredFile{Key: key, Size: written, ContentType: contentType}, nil
}

func (s *LocalAttachmentStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if key != filepath.Base(key) || strings.Contains(key, "..") {
		return nil, fmt.Errorf("invalid attachment key")
	}
	file, err := os.Open(filepath.Join(s.root, key))
	if err != nil {
		return nil, fmt.Errorf("open attachment: %w", err)
	}
	return file, nil
}
