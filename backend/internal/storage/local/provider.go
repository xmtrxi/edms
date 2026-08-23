package local

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type LocalProvider struct {
	path string
}

func New(path string) *LocalProvider {
	return &LocalProvider{
		path: path,
	}
}

func (s *LocalProvider) Name() string {
	return "local"
}

func (s *LocalProvider) resolve(path string) string {
	return filepath.Join(s.path, filepath.Clean("/"+path))
}

func (s *LocalProvider) Upload(ctx context.Context, path string, file io.Reader) error {
	dstPath := s.resolve(path)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, file)
	return err
}

func (s *LocalProvider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	return os.Open(s.resolve(path))
}

func (s *LocalProvider) Move(ctx context.Context, previousPath, newPath string) error {
	if err := os.MkdirAll(filepath.Dir(s.resolve(newPath)), 0o755); err != nil {
		return err
	}
	return os.Rename(s.resolve(previousPath), s.resolve(newPath))
}

func (s *LocalProvider) Delete(ctx context.Context, path string) error {
	return os.Remove(s.resolve(path))
}

func (s *LocalProvider) HealthCheck(ctx context.Context) error {
	if s.path == "" {
		return fmt.Errorf("local storage path is empty")
	}
	if err := os.MkdirAll(s.path, 0o755); err != nil {
		return err
	}
	return nil
}

func (s *LocalProvider) Exists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(s.resolve(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *LocalProvider) GetUrl(ctx context.Context, path string) (string, error) {
	return s.resolve(path), nil
}

func (s *LocalProvider) List(ctx context.Context, prefix string, cursor *string, limit int) ([]string, *string, error) {
	root := s.resolve(prefix)
	var files []string
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(s.path, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(files)
	start := 0
	if cursor != nil && *cursor != "" {
		if parsed, err := strconv.Atoi(*cursor); err == nil && parsed > 0 {
			start = parsed
		}
	}
	if start >= len(files) {
		return []string{}, nil, nil
	}
	end := len(files)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	var next *string
	if end < len(files) {
		value := strconv.Itoa(end)
		next = &value
	}
	return files[start:end], next, nil
}
