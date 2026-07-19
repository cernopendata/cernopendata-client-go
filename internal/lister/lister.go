package lister

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go-hep.org/x/hep/xrootd"
	"go-hep.org/x/hep/xrootd/xrdio"

	"github.com/cernopendata/cernopendata-client-go/internal/config"
)

type FileInfo struct {
	Name    string
	Size    int64
	IsDir   bool
	ModTime string
}

type Lister struct {
}

func NewLister() *Lister {
	return &Lister{}
}

func normalizePath(path string) string {
	if strings.HasPrefix(path, "root://") {
		return path
	}

	fullPath := config.ServerRootURI + path

	return strings.ReplaceAll(fullPath, "///", "//")
}

func (l *Lister) ListDirectory(ctx context.Context, path string) ([]FileInfo, error) {
	normalizedPath := normalizePath(path)
	url, err := xrdio.Parse(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
	}

	client, err := xrootd.NewClient(ctx, url.Addr, url.User)
	if err != nil {
		return nil, fmt.Errorf("failed to create XRootD client: %w", err)
	}
	defer func() { _ = client.Close() }()

	fs := client.FS()

	fi, err := fs.Stat(ctx, url.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %s: %w", url.Path, err)
	}

	if !fi.IsDir() {
		return []FileInfo{
			{
				Name:    fi.Name(),
				Size:    fi.Size(),
				IsDir:   fi.IsDir(),
				ModTime: fi.ModTime().Format("2006-01-02 15:04:05"),
			},
		}, nil
	}

	ents, err := fs.Dirlist(ctx, url.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	var entries []FileInfo
	for _, e := range ents {
		entries = append(entries, FileInfo{
			Name:    e.Name(),
			Size:    e.Size(),
			IsDir:   e.IsDir(),
			ModTime: e.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	return entries, nil
}

func (l *Lister) GetFileSize(ctx context.Context, path string) (int64, error) {
	normalizedPath := normalizePath(path)
	url, err := xrdio.Parse(normalizedPath)
	if err != nil {
		return 0, fmt.Errorf("failed to parse path: %w", err)
	}

	client, err := xrootd.NewClient(ctx, url.Addr, url.User)
	if err != nil {
		return 0, fmt.Errorf("failed to create XRootD client: %w", err)
	}
	defer func() { _ = client.Close() }()

	fs := client.FS()

	info, err := fs.Stat(ctx, url.Path)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	return info.Size(), nil
}

func (l *Lister) DirectoryExists(ctx context.Context, path string) (bool, error) {
	normalizedPath := normalizePath(path)
	url, err := xrdio.Parse(normalizedPath)
	if err != nil {
		return false, fmt.Errorf("failed to parse path: %w", err)
	}

	client, err := xrootd.NewClient(ctx, url.Addr, url.User)
	if err != nil {
		return false, fmt.Errorf("failed to create XRootD client: %w", err)
	}
	defer func() { _ = client.Close() }()

	fs := client.FS()

	info, err := fs.Stat(ctx, url.Path)
	if err != nil {
		return false, nil
	}

	return info.IsDir(), nil
}

func (l *Lister) ListDirectoryRecursive(ctx context.Context, path string) ([]FileInfo, error) {
	return l.listDirectoryRecursive(ctx, path, "")
}

func (l *Lister) listDirectoryRecursive(ctx context.Context, path string, base string) ([]FileInfo, error) {
	normalizedPath := normalizePath(path)
	url, err := xrdio.Parse(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
	}

	client, err := xrootd.NewClient(ctx, url.Addr, url.User)
	if err != nil {
		return nil, fmt.Errorf("failed to create XRootD client: %w", err)
	}
	defer func() { _ = client.Close() }()

	fs := client.FS()

	ents, err := fs.Dirlist(ctx, url.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	var entries []FileInfo
	for _, e := range ents {
		fullPath := filepath.Join(base, e.Name())
		entries = append(entries, FileInfo{
			Name:    fullPath,
			Size:    e.Size(),
			IsDir:   e.IsDir(),
			ModTime: e.ModTime().Format("2006-01-02 15:04:05"),
		})

		if e.IsDir() {
			subPath := path + "/" + e.Name()
			subEntries, err := l.listDirectoryRecursive(ctx, subPath, fullPath)
			if err != nil {
				return entries, err
			}
			entries = append(entries, subEntries...)
		}
	}

	return entries, nil
}
