package downloader

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

type filePathInfo struct {
	fileMap  map[string]any
	baseName string
	segments []string
}

// AssignLocalPaths computes a collision-free relative destination for each file.
// When multiple URIs share the same basename, it preserves the shortest unique
// trailing directory suffix needed to distinguish them.
func AssignLocalPaths(files []any) {
	groups := make(map[string][]filePathInfo)

	for _, file := range files {
		fileMap, ok := file.(map[string]any)
		if !ok {
			continue
		}

		if localPath, ok := fileMap["local_path"].(string); ok && sanitizeLocalPath(localPath) != "" {
			fileMap["local_path"] = sanitizeLocalPath(localPath)
			continue
		}

		uri, _ := fileMap["uri"].(string)
		segments := uriPathSegments(uri)
		if len(segments) == 0 {
			baseName := filepath.Base(uri)
			if baseName == "." || baseName == string(filepath.Separator) || baseName == "" {
				baseName = "downloaded-file"
			}
			fileMap["local_path"] = baseName
			continue
		}

		baseName := segments[len(segments)-1]
		groups[baseName] = append(groups[baseName], filePathInfo{
			fileMap:  fileMap,
			baseName: baseName,
			segments: segments,
		})
	}

	for _, group := range groups {
		if len(group) == 1 {
			group[0].fileMap["local_path"] = group[0].baseName
			continue
		}

		assignGroupLocalPaths(group)
	}
}

func DestinationPath(baseDir string, fileMap map[string]any) string {
	return filepath.Join(baseDir, getLocalPath(fileMap))
}

func DisplayPath(fileMap map[string]any) string {
	localPath := getLocalPath(fileMap)
	if localPath != "" {
		return filepath.ToSlash(localPath)
	}

	uri, _ := fileMap["uri"].(string)
	return filepath.Base(uri)
}

func getLocalPath(fileMap map[string]any) string {
	if fileMap == nil {
		return ""
	}

	if localPath, ok := fileMap["local_path"].(string); ok {
		return sanitizeLocalPath(localPath)
	}

	uri, _ := fileMap["uri"].(string)
	segments := uriPathSegments(uri)
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
		group[i].fileMap["local_path"] = uniqueCandidate
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
