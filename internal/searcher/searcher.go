package searcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cernopendata/cernopendata-client-go/internal/config"
)

type RecordResponse struct {
	Metadata map[string]any `json:"metadata"`
	ID       string         `json:"id"`
}

type FileInfo struct {
	URI          string `json:"uri"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum"`
	Availability string `json:"availability,omitempty"` // "online" or "on demand"
}

type SearchResponse struct {
	Hits         SearchHits             `json:"hits"`
	Aggregations map[string]Aggregation `json:"aggregations"`
}

// Aggregation represents a facet with its buckets of values
type Aggregation struct {
	Buckets []AggregationBucket `json:"buckets"`
}

// AggregationBucket represents a single facet value with its count
type AggregationBucket struct {
	Key      any `json:"key"`
	DocCount int `json:"doc_count"`
}

type SearchHits struct {
	Total int         `json:"total"`
	Hits  []SearchHit `json:"hits"`
}

type SearchHit struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"metadata"`
}

type Client struct {
	server string
	client *http.Client
}

func cleanupMetadata(metadata map[string]any) error {
	if metadata == nil {
		return fmt.Errorf("metadata is nil")
	}

	delete(metadata, "_files")

	if files, ok := metadata["files"].([]any); ok {
		for _, file := range files {
			if fileMap, ok := file.(map[string]any); ok {
				delete(fileMap, "bucket")
				delete(fileMap, "version_id")
			}
		}
	}

	if indices, ok := metadata["_file_indices"].([]any); ok {
		for _, idx := range indices {
			if idxMap, ok := idx.(map[string]any); ok {
				delete(idxMap, "bucket")
				if files, ok := idxMap["files"].([]any); ok {
					for _, file := range files {
						if fileMap, ok := file.(map[string]any); ok {
							delete(fileMap, "bucket")
							delete(fileMap, "version_id")
						}
					}
				}
			}
		}
	}

	return nil
}

func getMetadataFieldAsString(metadata map[string]any, field string) (string, error) {
	value, ok := metadata[field]
	if !ok {
		return "", fmt.Errorf("field '%s' not found", field)
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("field '%s' is not a string, got %T", field, value)
	}

	return strValue, nil
}

func getMetadataFieldAsInt(metadata map[string]any, field string) (int, error) {
	value, ok := metadata[field]
	if !ok {
		return 0, fmt.Errorf("field '%s' not found", field)
	}

	if intValue, ok := value.(int); ok {
		return intValue, nil
	}

	if strValue, ok := value.(string); ok {
		intValue, err := strconv.Atoi(strValue)
		if err != nil {
			return 0, fmt.Errorf("field '%s' is not a valid integer string: %w", field, err)
		}
		return intValue, nil
	}

	if floatValue, ok := value.(float64); ok {
		return int(floatValue), nil
	}

	return 0, fmt.Errorf("field '%s' is not an integer, got %T", field, value)
}

func getMetadataFieldAsInt64(metadata map[string]any, field string) (int64, error) {
	value, ok := metadata[field]
	if !ok {
		return 0, fmt.Errorf("field '%s' not found", field)
	}

	if intValue, ok := value.(int); ok {
		return int64(intValue), nil
	}

	if strValue, ok := value.(string); ok {
		intValue, err := strconv.ParseInt(strValue, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("field '%s' is not a valid integer string: %w", field, err)
		}
		return intValue, nil
	}

	if floatValue, ok := value.(float64); ok {
		return int64(floatValue), nil
	}

	return 0, fmt.Errorf("field '%s' is not an integer, got %T", field, value)
}

func NewClient(server string) *Client {
	return &Client{
		server: server,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) GetRecord(recid int) (*RecordResponse, error) {
	url := fmt.Sprintf("%s/api/records/%d", c.server, recid)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get record %d: %w", recid, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d for record %d", resp.StatusCode, recid)
	}

	var recordResp RecordResponse
	if err := json.NewDecoder(resp.Body).Decode(&recordResp); err != nil {
		return nil, fmt.Errorf("failed to decode record response: %w", err)
	}

	if err := cleanupMetadata(recordResp.Metadata); err != nil {
		return nil, fmt.Errorf("failed to cleanup metadata: %w", err)
	}

	return &recordResp, nil
}

func (c *Client) GetRecordByDOI(doi string) (*RecordResponse, error) {
	return c.getRecordBySearch("doi", doi)
}

func (c *Client) GetRecordByTitle(title string) (*RecordResponse, error) {
	return c.getRecordBySearch("title", title)
}

func (c *Client) getRecordBySearch(field, value string) (*RecordResponse, error) {
	searchURL := fmt.Sprintf("%s/api/records?page=1&size=1&q=%s:%s",
		c.server,
		field,
		url.QueryEscape(fmt.Sprintf("\"%s\"", value)),
	)

	resp, err := c.client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search for %s: %w", field, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d for search", resp.StatusCode)
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	if searchResp.Hits.Total == 0 {
		return nil, fmt.Errorf("no record found with %s: %s", field, value)
	}

	if searchResp.Hits.Total > 1 {
		return nil, fmt.Errorf("more than one record found with %s: %s", field, value)
	}

	return c.GetRecordByID(searchResp.Hits.Hits[0].ID)
}

func (c *Client) GetRecordByID(id string) (*RecordResponse, error) {
	recordID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid record ID: %w", err)
	}
	return c.GetRecord(recordID)
}

// convertURI transforms a URI based on protocol settings
func convertURI(uri, serverRoot, serverURI, protocol string) string {
	if !strings.HasPrefix(uri, serverRoot) {
		return uri
	}
	switch protocol {
	case "http":
		return strings.Replace(uri, serverRoot, serverURI+"/", 1)
	case "https":
		return strings.Replace(uri, serverRoot, config.ServerHTTPSURI+"/", 1)
	default:
		return uri
	}
}

func (c *Client) GetFilesList(record *RecordResponse, protocol string, expand bool) ([]FileInfo, error) {
	var files []FileInfo

	if record.Metadata == nil {
		return nil, fmt.Errorf("metadata is nil")
	}

	recidInt, err := getMetadataFieldAsInt(record.Metadata, "recid")
	if err != nil {
		return nil, fmt.Errorf("failed to get recid from metadata: %w", err)
	}

	serverRoot := config.ServerRootURI
	serverURI := c.server

	if filesArray, ok := record.Metadata["files"].([]any); ok {
		for _, file := range filesArray {
			fileMap, ok := file.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("file entry is not a map")
			}

			uri, err := getMetadataFieldAsString(fileMap, "uri")
			if err != nil {
				return nil, fmt.Errorf("failed to get uri: %w", err)
			}

			size, err := getMetadataFieldAsInt64(fileMap, "size")
			if err != nil {
				return nil, fmt.Errorf("failed to get size: %w", err)
			}

			checksum, _ := getMetadataFieldAsString(fileMap, "checksum")

			files = append(files, FileInfo{
				URI:          convertURI(uri, serverRoot, serverURI, protocol),
				Size:         size,
				Checksum:     checksum,
				Availability: "online",
			})
		}
	}

	if expand {
		if indices, ok := record.Metadata["_file_indices"].([]any); ok {
			for _, index := range indices {
				indexMap, ok := index.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("index entry is not a map")
				}

				if indexFiles, ok := indexMap["files"].([]any); ok {
					for _, innerFile := range indexFiles {
						fileMap, ok := innerFile.(map[string]any)
						if !ok {
							return nil, fmt.Errorf("inner file entry is not a map")
						}

						uri, err := getMetadataFieldAsString(fileMap, "uri")
						if err != nil {
							return nil, fmt.Errorf("failed to get uri: %w", err)
						}

						size, err := getMetadataFieldAsInt64(fileMap, "size")
						if err != nil {
							return nil, fmt.Errorf("failed to get size: %w", err)
						}

						checksum, _ := getMetadataFieldAsString(fileMap, "checksum")

						availability, _ := getMetadataFieldAsString(fileMap, "availability")
						if availability == "" {
							availability = "online"
						}

						files = append(files, FileInfo{
							URI:          convertURI(uri, serverRoot, serverURI, protocol),
							Size:         size,
							Checksum:     checksum,
							Availability: availability,
						})
					}
				}
			}
		}
	} else {
		if indices, ok := record.Metadata["_file_indices"].([]any); ok {
			for _, index := range indices {
				indexMap, ok := index.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("index entry is not a map")
				}

				key, err := getMetadataFieldAsString(indexMap, "key")
				if err != nil {
					return nil, fmt.Errorf("failed to get key: %w", err)
				}

				size, err := getMetadataFieldAsInt64(indexMap, "size")
				if err != nil {
					return nil, fmt.Errorf("failed to get size: %w", err)
				}

				var uri string
				if protocol == "xrootd" {
					uri = fmt.Sprintf("%s/record/%d/file_index/%s", serverRoot, recidInt, key)
				} else {
					uri = fmt.Sprintf("%s/record/%d/file_index/%s", serverURI, recidInt, key)
				}

				files = append(files, FileInfo{
					URI:      uri,
					Size:     size,
					Checksum: "",
				})
			}
		}
	}

	return files, nil
}

// FilterFilesByAvailability filters files by their availability status.
// If availability is empty/nil, returns all files (default behavior).
// Returns (filtered files, has offline files warning).
func FilterFilesByAvailability(files []FileInfo, availability string) ([]FileInfo, bool) {
	hasOfflineFiles := false
	for _, f := range files {
		if f.Availability != "" && f.Availability != "online" {
			hasOfflineFiles = true
			break
		}
	}

	if availability == "" {
		return files, hasOfflineFiles
	}

	if availability == "online" {
		var filtered []FileInfo
		for _, f := range files {
			if f.Availability == "online" {
				filtered = append(filtered, f)
			}
		}
		return filtered, hasOfflineFiles
	}

	// For "all" or any other value, return all files
	return files, hasOfflineFiles
}

func GetRecid(server, doi string, title string, recid int) (int, error) {
	if recid > 0 {
		return recid, nil
	}

	client := NewClient(server)

	if doi != "" {
		record, err := client.GetRecordByDOI(doi)
		if err != nil {
			return 0, err
		}
		recidValue, err := getMetadataFieldAsInt(record.Metadata, "recid")
		if err != nil {
			return 0, fmt.Errorf("failed to extract recid from metadata: %w", err)
		}
		return recidValue, nil
	}

	if title != "" {
		record, err := client.GetRecordByTitle(title)
		if err != nil {
			return 0, err
		}
		recidValue, err := getMetadataFieldAsInt(record.Metadata, "recid")
		if err != nil {
			return 0, fmt.Errorf("failed to extract recid from metadata: %w", err)
		}
		return recidValue, nil
	}

	return 0, fmt.Errorf("please provide recid, doi, or title")
}

// SearchRecords searches for records using a query string and optional facets.
// page and size control pagination, sort controls ordering.
// Returns records with metadata included.
func (c *Client) SearchRecords(q string, facets map[string]string, page, size int, sort string) (*SearchResponse, error) {
	params := url.Values{}
	if q != "" {
		params.Set("q", q)
	}
	for key, value := range facets {
		params.Add("f", fmt.Sprintf("%s:%s", key, value))
	}
	params.Set("page", strconv.Itoa(page))
	params.Set("size", strconv.Itoa(size))
	if sort != "" {
		params.Set("sort", sort)
	}
	params.Set("skip_files", "1")

	searchURL := fmt.Sprintf("%s/api/records/?%s", c.server, params.Encode())

	resp, err := c.client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search records: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d for search", resp.StatusCode)
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &searchResp, nil
}

// SearchAllRecords fetches all matching records by paginating through results
// in batches of 50. Returns a combined SearchResponse with all hits.
func (c *Client) SearchAllRecords(q string, facets map[string]string, sort string) (*SearchResponse, error) {
	const batchSize = 50
	var allHits []SearchHit
	page := 1
	totalRecords := 0

	for {
		resp, err := c.SearchRecords(q, facets, page, batchSize, sort)
		if err != nil {
			return nil, err
		}

		if page == 1 {
			totalRecords = resp.Hits.Total
		}

		allHits = append(allHits, resp.Hits.Hits...)

		// Check if we've fetched all records
		if len(allHits) >= totalRecords {
			break
		}

		page++
	}

	return &SearchResponse{
		Hits: SearchHits{
			Total: totalRecords,
			Hits:  allHits,
		},
	}, nil
}

// GetFacets fetches available facets (aggregations) from the API.
// This makes a minimal search request to get the aggregation data.
func (c *Client) GetFacets() (map[string]Aggregation, error) {
	// Make a minimal search to get aggregations
	resp, err := c.SearchRecords("", nil, 1, 1, "")
	if err != nil {
		return nil, err
	}
	return resp.Aggregations, nil
}
