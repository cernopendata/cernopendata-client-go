package downloader

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/cernopendata/cernopendata-client-go/internal/filemetadata"
)

type filePathInfo struct {
	file     *filemetadata.File
	baseName string
	segments []string
}

// AssignLocalPaths computes a collision-free relative destination for each file.
// When multiple URIs share the same basename, it preserves the shortest unique
// trailing directory suffix needed to distinguish them.
func AssignLocalPaths(files []filemetadata.File) {
	groups := make(map[string][]filePathInfo)

	for i := range files {
		file := &files[i]
		if sanitizeLocalPath(file.LocalPath) != "" {
			file.LocalPath = sanitizeLocalPath(file.LocalPath)
			continue
		}

		segments := uriPathSegments(file.URI)
		if len(segments) == 0 {
			baseName := filepath.Base(file.URI)
			if baseName == "." || baseName == string(filepath.Separator) || baseName == "" {
				baseName = "downloaded-file"
			}
			file.LocalPath = baseName
			continue
		}

		baseName := segments[len(segments)-1]
		groups[baseName] = append(groups[baseName], filePathInfo{
			file:     file,
			baseName: baseName,
			segments: segments,
		})
	}

	for _, group := range groups {
		if len(group) == 1 {
			group[0].file.LocalPath = group[0].baseName
			continue
		}

		assignGroupLocalPaths(group)
	}
}

func DestinationPath(baseDir string, file filemetadata.File) string {
	return filepath.Join(baseDir, getLocalPath(file))
}

func DisplayPath(file filemetadata.File) string {
	localPath := getLocalPath(file)
	if localPath != "" {
		return filepath.ToSlash(localPath)
	}

	return filepath.Base(file.URI)
}

func getLocalPath(file filemetadata.File) string {
	if file.LocalPath != "" {
		return sanitizeLocalPath(file.LocalPath)
	}

	segments := uriPathSegments(file.URI)
	if len(segments) == 0 {
		return ""
	}

	return segments[len(segments)-1]
}

func assignGroupLocalPaths(group []filePathInfo) {
	resolved := make([]string, len(group))
	unresolved := make(map[int]struct{}, len(group))
	maxDepth := 0

	for i, item := range group {
		unresolved[i] = struct{}{}
		if len(item.segments) > maxDepth {
			maxDepth = len(item.segments)
		}
	}

	for depth := 2; depth <= maxDepth; depth++ {
		candidates := make(map[string][]int)
		for idx := range unresolved {
			candidate := suffixPath(group[idx].segments, depth)
			candidates[candidate] = append(candidates[candidate], idx)
		}

		for candidate, indices := range candidates {
			if len(indices) != 1 {
				continue
			}
			idx := indices[0]
			resolved[idx] = candidate
			delete(unresolved, idx)
		}

		if len(unresolved) == 0 {
			break
		}
	}

	for idx := range unresolved {
		resolved[idx] = strings.Join(group[idx].segments, "/")
	}

	seen := make(map[string]struct{})
	for i, candidate := range resolved {
		candidate = sanitizeLocalPath(candidate)
		if candidate == "" {
			candidate = group[i].baseName
		}

		uniqueCandidate := candidate
		for suffix := 2; ; suffix++ {
			if _, exists := seen[uniqueCandidate]; !exists {
				break
			}
			uniqueCandidate = addNumericSuffix(candidate, suffix)
		}

		seen[uniqueCandidate] = struct{}{}
		group[i].file.LocalPath = uniqueCandidate
	}
}

func suffixPath(segments []string, depth int) string {
	if len(segments) == 0 {
		return ""
	}
	if depth > len(segments) {
		depth = len(segments)
	}
	return strings.Join(segments[len(segments)-depth:], "/")
}

func addNumericSuffix(localPath string, n int) string {
	ext := path.Ext(localPath)
	base := strings.TrimSuffix(localPath, ext)
	return fmt.Sprintf("%s-%d%s", base, n, ext)
}

func uriPathSegments(uri string) []string {
	cleaned := strings.TrimSpace(uri)
	if cleaned == "" {
		return nil
	}

	if parsed, err := url.Parse(cleaned); err == nil && parsed.Path != "" {
		cleaned = parsed.Path
	}

	cleaned = path.Clean(cleaned)
	if cleaned == "." || cleaned == "/" {
		return nil
	}

	rawSegments := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		segments = append(segments, segment)
	}

	return segments
}

func sanitizeLocalPath(localPath string) string {
	cleaned := path.Clean(strings.TrimSpace(localPath))
	if cleaned == "." || cleaned == "/" || cleaned == "" {
		return ""
	}

	rawSegments := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		segments = append(segments, segment)
	}

	return strings.Join(segments, "/")
}
