package searcher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	server := "http://opendata.cern.ch"
	client := NewClient(server)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.server != server {
		t.Errorf("client.server = %q, want %q", client.server, server)
	}

	if client.client == nil {
		t.Error("client.client is nil")
	}
}

func TestGetRecord(t *testing.T) {
	tests := []struct {
		name    string
		recid   int
		handler http.HandlerFunc
		want    *RecordResponse
		wantErr bool
	}{
		{
			name:  "successful record fetch",
			recid: 3005,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				metadata := map[string]any{
					"recid": 3005,
					"title": "Test Record",
					"doi":   "10.7483/record/3005",
				}
				resp := RecordResponse{
					ID:       "3005",
					Metadata: metadata,
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			want: &RecordResponse{
				ID: "3005",
				Metadata: map[string]any{
					"recid": 3005,
					"title": "Test Record",
					"doi":   "10.7483/record/3005",
				},
			},
			wantErr: false,
		},
		{
			name:  "record not found",
			recid: 999999,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:  "server error",
			recid: 3005,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			record, err := client.GetRecord(tt.recid)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetRecord() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if record == nil {
					t.Fatal("GetRecord() returned nil record")
				}

				recid, err := getMetadataFieldAsInt(record.Metadata, "recid")
				if err != nil {
					t.Fatal(err)
				}
				wantRecid, err := getMetadataFieldAsInt(tt.want.Metadata, "recid")
				if err != nil {
					t.Fatal(err)
				}

				if recid != wantRecid {
					t.Errorf("GetRecord() recid = %d, want %d", recid, wantRecid)
				}

				title, err := getMetadataFieldAsString(record.Metadata, "title")
				if err != nil {
					t.Fatal(err)
				}
				wantTitle, err := getMetadataFieldAsString(tt.want.Metadata, "title")
				if err != nil {
					t.Fatal(err)
				}

				if title != wantTitle {
					t.Errorf("GetRecord() title = %q, want %q", title, wantTitle)
				}
			}
		})
	}
}

func TestGetFilesList(t *testing.T) {
	tests := []struct {
		name     string
		record   *RecordResponse
		protocol string
		expand   bool
		wantLen  int
	}{
		{
			name: "http protocol without expand",
			record: &RecordResponse{
				Metadata: map[string]any{
					"recid": 3005,
					"files": []any{
						map[string]any{
							"uri":      "http://opendata.cern.ch/test.txt",
							"size":     100,
							"checksum": "adler32:12345678",
						},
					},
					"_file_indices": []any{},
				},
			},
			protocol: "http",
			expand:   false,
			wantLen:  1,
		},
		{
			name: "root protocol without expand",
			record: &RecordResponse{
				Metadata: map[string]any{
					"recid": 3005,
					"_file_indices": []any{
						map[string]any{
							"key":      "index1",
							"size":     100,
							"checksum": "adler32:87654321",
							"files": []any{
								map[string]any{
									"uri":      "root://eospublic.cern.ch//eos/opendata/cms/inner.txt",
									"size":     50,
									"checksum": "adler32:11111111",
								},
							},
						},
					},
				},
			},
			protocol: "http",
			expand:   false,
			wantLen:  1,
		},
		{
			name: "https protocol without expand",
			record: &RecordResponse{
				Metadata: map[string]any{
					"recid": 3005,
					"files": []any{
						map[string]any{
							"uri":      "https://opendata.cern.ch/test.txt",
							"size":     100,
							"checksum": "adler32:12345678",
						},
					},
					"_file_indices": []any{},
				},
			},
			protocol: "https",
			expand:   false,
			wantLen:  1,
		},
		{
			name: "expand file indices",
			record: &RecordResponse{
				Metadata: map[string]any{
					"recid": 3005,
					"files": []any{
						map[string]any{
							"uri":      "http://opendata.cern.ch/test.txt",
							"size":     100,
							"checksum": "adler32:12345678",
						},
					},
					"_file_indices": []any{
						map[string]any{
							"key":      "index1",
							"size":     100,
							"checksum": "adler32:87654321",
							"files": []any{
								map[string]any{
									"uri":      "http://opendata.cern.ch/inner1.txt",
									"size":     50,
									"checksum": "adler32:11111111",
								},
								map[string]any{
									"uri":      "http://opendata.cern.ch/inner2.txt",
									"size":     50,
									"checksum": "adler32:22222222",
								},
							},
						},
					},
				},
			},
			protocol: "http",
			expand:   true,
			wantLen:  3,
		},
		{
			name: "xrootd protocol (no conversion)",
			record: &RecordResponse{
				Metadata: map[string]any{
					"recid": 3005,
					"files": []any{
						map[string]any{
							"uri":      "http://opendata.cern.ch/test.txt",
							"size":     100,
							"checksum": "adler32:12345678",
						},
					},
					"_file_indices": []any{},
				},
			},
			protocol: "xrootd",
			expand:   false,
			wantLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := "http://test.server"
			client := NewClient(server)
			files, err := client.GetFilesList(tt.record, tt.protocol, tt.expand)

			if err != nil {
				t.Errorf("GetFilesList() error = %v", err)
				return
			}

			if len(files) != tt.wantLen {
				t.Errorf("GetFilesList() length = %d, want %d", len(files), tt.wantLen)
			}

			if tt.protocol == "http" || tt.protocol == "https" {
				for _, file := range files {
					if len(files) > 0 {
						if !strings.HasPrefix(file.URI, "http://test.server") {
							t.Logf("GetFilesList() first URI = %q", file.URI)
						}
					}
				}
			}
		})
	}
}

func TestGetRecordByDOI(t *testing.T) {
	tests := []struct {
		name      string
		doi       string
		handler   http.HandlerFunc
		wantRecid int
		wantErr   bool
	}{
		{
			name: "successful DOI search",
			doi:  "10.7483/record/3005",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.Contains(r.URL.Path, "records/") {
					metadata := map[string]any{
						"recid": 3005,
						"title": "Test Record",
						"doi":   "10.7483/record/3005",
					}
					resp := RecordResponse{
						ID:       "3005",
						Metadata: metadata,
					}
					_ = json.NewEncoder(w).Encode(resp)
				} else {
					searchResp := SearchResponse{
						Hits: SearchHits{
							Total: 1,
							Hits: []SearchHit{
								{ID: "3005"},
							},
						},
					}
					_ = json.NewEncoder(w).Encode(searchResp)
				}
			},
			wantRecid: 3005,
			wantErr:   false,
		},
		{
			name: "DOI not found",
			doi:  "10.7483/record/nonexistent",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				searchResp := SearchResponse{
					Hits: SearchHits{
						Total: 0,
						Hits:  []SearchHit{},
					},
				}
				_ = json.NewEncoder(w).Encode(searchResp)
			},
			wantRecid: 0,
			wantErr:   true,
		},
		{
			name: "multiple records found",
			doi:  "10.7483/record/duplicate",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				searchResp := SearchResponse{
					Hits: SearchHits{
						Total: 2,
						Hits: []SearchHit{
							{ID: "3005"},
							{ID: "3006"},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(searchResp)
			},
			wantRecid: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			record, err := client.GetRecordByDOI(tt.doi)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetRecordByDOI() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && record != nil {
				recid, err := getMetadataFieldAsInt(record.Metadata, "recid")
				if err != nil {
					t.Fatal(err)
				}
				if recid != tt.wantRecid {
					t.Errorf("GetRecordByDOI() recid = %d, want %d", recid, tt.wantRecid)
				}
			}
		})
	}
}

func TestGetRecordByTitle(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		handler   http.HandlerFunc
		wantRecid int
		wantErr   bool
	}{
		{
			name:  "successful title search",
			title: "Test Record Title",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.Contains(r.URL.Path, "records/") {
					metadata := map[string]any{
						"recid": 3005,
						"title": "Test Record",
						"doi":   "10.7483/record/3005",
					}
					resp := RecordResponse{
						ID:       "3005",
						Metadata: metadata,
					}
					_ = json.NewEncoder(w).Encode(resp)
				} else {
					searchResp := SearchResponse{
						Hits: SearchHits{
							Total: 1,
							Hits: []SearchHit{
								{ID: "3005"},
							},
						},
					}
					_ = json.NewEncoder(w).Encode(searchResp)
				}
			},
			wantRecid: 3005,
			wantErr:   false,
		},
		{
			name:  "title not found",
			title: "Nonexistent Title",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				searchResp := SearchResponse{
					Hits: SearchHits{
						Total: 0,
						Hits:  []SearchHit{},
					},
				}
				_ = json.NewEncoder(w).Encode(searchResp)
			},
			wantRecid: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			record, err := client.GetRecordByTitle(tt.title)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetRecordByTitle() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && record != nil {
				recid, err := getMetadataFieldAsInt(record.Metadata, "recid")
				if err != nil {
					t.Fatal(err)
				}
				if recid != tt.wantRecid {
					t.Errorf("GetRecordByTitle() recid = %d, want %d", recid, tt.wantRecid)
				}
			}
		})
	}
}

func TestGetRecid(t *testing.T) {
	tests := []struct {
		name    string
		server  string
		doi     string
		title   string
		recid   int
		want    int
		wantErr bool
	}{
		{
			name:    "recid provided",
			recid:   3005,
			want:    3005,
			wantErr: false,
		},
		{
			name:    "zero recid (invalid)",
			recid:   0,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetRecid(tt.server, tt.doi, tt.title, tt.recid)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetRecid() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("GetRecid() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFileInfo(t *testing.T) {
	tests := []struct {
		name   string
		file   FileInfo
		checks func(t *testing.T, fi FileInfo)
	}{
		{
			name: "complete file info",
			file: FileInfo{
				URI:      "http://example.com/file.txt",
				Size:     1024,
				Checksum: "adler32:12345678",
			},
			checks: func(t *testing.T, fi FileInfo) {
				if fi.URI != "http://example.com/file.txt" {
					t.Errorf("FileInfo.URI = %q, want %q", fi.URI, "http://example.com/file.txt")
				}
				if fi.Size != 1024 {
					t.Errorf("FileInfo.Size = %d, want %d", fi.Size, 1024)
				}
				if fi.Checksum != "adler32:12345678" {
					t.Errorf("FileInfo.Checksum = %q, want %q", fi.Checksum, "adler32:12345678")
				}
			},
		},
		{
			name: "zero size file",
			file: FileInfo{
				URI:      "http://example.com/empty.txt",
				Size:     0,
				Checksum: "adler32:00000000",
			},
			checks: func(t *testing.T, fi FileInfo) {
				if fi.Size != 0 {
					t.Errorf("FileInfo.Size = %d, want 0", fi.Size)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.checks != nil {
				tt.checks(t, tt.file)
			}
		})
	}
}

func TestSearchRecords(t *testing.T) {
	tests := []struct {
		name      string
		q         string
		facets    map[string]string
		page      int
		size      int
		sort      string
		handler   http.HandlerFunc
		wantTotal int
		wantLen   int
		wantErr   bool
	}{
		{
			name:   "basic search",
			q:      "Higgs",
			facets: nil,
			page:   1,
			size:   10,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.RawQuery, "q=Higgs") {
					t.Errorf("expected q=Higgs in query, got %s", r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				resp := SearchResponse{
					Hits: SearchHits{
						Total: 5,
						Hits: []SearchHit{
							{ID: "1", Metadata: map[string]any{"title": "Higgs Boson"}},
							{ID: "2", Metadata: map[string]any{"title": "Higgs Search"}},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantTotal: 5,
			wantLen:   2,
			wantErr:   false,
		},
		{
			name:   "search with facets",
			q:      "muon",
			facets: map[string]string{"experiment": "CMS"},
			page:   1,
			size:   10,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.RawQuery, "f=experiment") {
					t.Errorf("expected facet in query, got %s", r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				resp := SearchResponse{
					Hits: SearchHits{
						Total: 3,
						Hits: []SearchHit{
							{ID: "1", Metadata: map[string]any{"title": "CMS Muon Data"}},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantTotal: 3,
			wantLen:   1,
			wantErr:   false,
		},
		{
			name:   "empty results",
			q:      "nonexistent",
			facets: nil,
			page:   1,
			size:   10,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				resp := SearchResponse{
					Hits: SearchHits{
						Total: 0,
						Hits:  []SearchHit{},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantTotal: 0,
			wantLen:   0,
			wantErr:   false,
		},
		{
			name:   "server error",
			q:      "test",
			facets: nil,
			page:   1,
			size:   10,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			resp, err := client.SearchRecords(tt.q, tt.facets, tt.page, tt.size, tt.sort)

			if (err != nil) != tt.wantErr {
				t.Errorf("SearchRecords() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if resp.Hits.Total != tt.wantTotal {
					t.Errorf("SearchRecords() total = %d, want %d", resp.Hits.Total, tt.wantTotal)
				}
				if len(resp.Hits.Hits) != tt.wantLen {
					t.Errorf("SearchRecords() hits = %d, want %d", len(resp.Hits.Hits), tt.wantLen)
				}
			}
		})
	}
}

func TestGetFacets(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		wantFacets  int
		wantBuckets map[string]int
		wantErr     bool
	}{
		{
			name: "successful facets fetch",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				resp := map[string]any{
					"hits": map[string]any{
						"total": 100,
						"hits":  []any{},
					},
					"aggregations": map[string]any{
						"experiment": map[string]any{
							"buckets": []any{
								map[string]any{"key": "CMS", "doc_count": 50},
								map[string]any{"key": "ATLAS", "doc_count": 30},
							},
						},
						"type": map[string]any{
							"buckets": []any{
								map[string]any{"key": "Dataset", "doc_count": 80},
							},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantFacets:  2,
			wantBuckets: map[string]int{"experiment": 2, "type": 1},
			wantErr:     false,
		},
		{
			name: "empty aggregations",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				resp := map[string]any{
					"hits": map[string]any{
						"total": 0,
						"hits":  []any{},
					},
					"aggregations": map[string]any{},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantFacets: 0,
			wantErr:    false,
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			facets, err := client.GetFacets()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetFacets() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(facets) != tt.wantFacets {
					t.Errorf("GetFacets() returned %d facets, want %d", len(facets), tt.wantFacets)
				}

				for name, expectedBuckets := range tt.wantBuckets {
					if agg, ok := facets[name]; ok {
						if len(agg.Buckets) != expectedBuckets {
							t.Errorf("GetFacets()[%s] has %d buckets, want %d", name, len(agg.Buckets), expectedBuckets)
						}
					} else {
						t.Errorf("GetFacets() missing expected facet: %s", name)
					}
				}
			}
		})
	}
}

func TestSearchRecordsWithAggregations(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"hits": map[string]any{
				"total": 5,
				"hits": []any{
					map[string]any{"id": "1", "metadata": map[string]any{"title": "Test"}},
				},
			},
			"aggregations": map[string]any{
				"experiment": map[string]any{
					"buckets": []any{
						map[string]any{"key": "CMS", "doc_count": 5},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.SearchRecords("test", nil, 1, 10, "")

	if err != nil {
		t.Fatalf("SearchRecords() error = %v", err)
	}

	if resp.Aggregations == nil {
		t.Error("SearchRecords() Aggregations is nil")
	}

	if agg, ok := resp.Aggregations["experiment"]; !ok {
		t.Error("SearchRecords() missing experiment aggregation")
	} else if len(agg.Buckets) != 1 {
		t.Errorf("SearchRecords() experiment has %d buckets, want 1", len(agg.Buckets))
	}
}

func TestSearchAllRecords(t *testing.T) {
	tests := []struct {
		name         string
		totalRecords int
		batchSize    int
		wantPages    int
		wantTotal    int
	}{
		{
			name:         "single page",
			totalRecords: 10,
			batchSize:    50,
			wantPages:    1,
			wantTotal:    10,
		},
		{
			name:         "multiple pages",
			totalRecords: 125,
			batchSize:    50,
			wantPages:    3,
			wantTotal:    125,
		},
		{
			name:         "empty results",
			totalRecords: 0,
			batchSize:    50,
			wantPages:    1,
			wantTotal:    0,
		},
		{
			name:         "exact batch boundary",
			totalRecords: 100,
			batchSize:    50,
			wantPages:    2,
			wantTotal:    100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pageRequests := 0
			handler := func(w http.ResponseWriter, r *http.Request) {
				pageRequests++
				w.Header().Set("Content-Type", "application/json")

				// Calculate how many hits to return for this page
				startIdx := (pageRequests - 1) * 50
				remaining := tt.totalRecords - startIdx
				hitsThisPage := min(remaining, 50)
				if hitsThisPage < 0 {
					hitsThisPage = 0
				}

				hits := make([]any, hitsThisPage)
				for i := range hits {
					hits[i] = map[string]any{
						"id":       string(rune('0' + startIdx + i)), // #nosec G115 -- bounded test fixture IDs
						"metadata": map[string]any{"title": "Record"},
					}
				}

				resp := map[string]any{
					"hits": map[string]any{
						"total": tt.totalRecords,
						"hits":  hits,
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			}

			server := httptest.NewServer(http.HandlerFunc(handler))
			defer server.Close()

			client := NewClient(server.URL)
			resp, err := client.SearchAllRecords("test", nil, "")

			if err != nil {
				t.Fatalf("SearchAllRecords() error = %v", err)
			}

			if resp.Hits.Total != tt.wantTotal {
				t.Errorf("SearchAllRecords() total = %d, want %d", resp.Hits.Total, tt.wantTotal)
			}

			if len(resp.Hits.Hits) != tt.wantTotal {
				t.Errorf("SearchAllRecords() hits count = %d, want %d", len(resp.Hits.Hits), tt.wantTotal)
			}

			if pageRequests != tt.wantPages {
				t.Errorf("SearchAllRecords() made %d page requests, want %d", pageRequests, tt.wantPages)
			}
		})
	}
}

func TestSearchAllRecordsError(t *testing.T) {
	requestCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 2 {
			// Fail on second page
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"hits": map[string]any{
				"total": 100, // Indicates more pages needed
				"hits": []any{
					map[string]any{"id": "1"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.SearchAllRecords("test", nil, "")

	if err == nil {
		t.Error("SearchAllRecords() expected error on second page, got nil")
	}
}

func TestGetRecidWithDOI(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/api/records/") {
			metadata := map[string]any{
				"recid": 3005,
				"title": "Test Record",
				"doi":   "10.7483/OPENDATA.TEST",
			}
			resp := RecordResponse{
				ID:       "3005",
				Metadata: metadata,
			}
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			resp := SearchResponse{
				Hits: SearchHits{
					Total: 1,
					Hits: []SearchHit{
						{ID: "3005"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	recid, err := GetRecid(server.URL, "10.7483/OPENDATA.TEST", "", 0)
	if err != nil {
		t.Fatalf("GetRecid() with DOI error = %v", err)
	}

	if recid != 3005 {
		t.Errorf("GetRecid() with DOI = %d, want 3005", recid)
	}
}

func TestGetRecidWithTitle(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/api/records/") {
			metadata := map[string]any{
				"recid": 1234,
				"title": "My Test Title",
			}
			resp := RecordResponse{
				ID:       "1234",
				Metadata: metadata,
			}
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			resp := SearchResponse{
				Hits: SearchHits{
					Total: 1,
					Hits: []SearchHit{
						{ID: "1234"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	recid, err := GetRecid(server.URL, "", "My Test Title", 0)
	if err != nil {
		t.Fatalf("GetRecid() with title error = %v", err)
	}

	if recid != 1234 {
		t.Errorf("GetRecid() with title = %d, want 1234", recid)
	}
}

func TestFilterFilesByAvailability(t *testing.T) {
	files := []FileInfo{
		{URI: "file1", Availability: "online"},
		{URI: "file2", Availability: "on demand"},
		{URI: "file3", Availability: "online"},
		{URI: "file4", Availability: ""}, // Should be treated as unknown, but checked for offline
	}

	tests := []struct {
		name              string
		inputFiles        []FileInfo
		availability      string
		wantLen           int
		wantHasOffline    bool
		wantFilteredFiles []string // URIs
	}{
		{
			name:              "filter online",
			inputFiles:        files,
			availability:      "online",
			wantLen:           2,
			wantHasOffline:    true,
			wantFilteredFiles: []string{"file1", "file3"},
		},
		{
			name:              "filter all (explicit)",
			inputFiles:        files,
			availability:      "all",
			wantLen:           4,
			wantHasOffline:    true,
			wantFilteredFiles: []string{"file1", "file2", "file3", "file4"},
		},
		{
			name:              "filter empty (default)",
			inputFiles:        files,
			availability:      "",
			wantLen:           4,
			wantHasOffline:    true,
			wantFilteredFiles: []string{"file1", "file2", "file3", "file4"},
		},
		{
			name: "all online files",
			inputFiles: []FileInfo{
				{URI: "file1", Availability: "online"},
				{URI: "file2", Availability: "online"},
			},
			availability:      "",
			wantLen:           2,
			wantHasOffline:    false,
			wantFilteredFiles: []string{"file1", "file2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered, hasOffline := FilterFilesByAvailability(tt.inputFiles, tt.availability)

			if len(filtered) != tt.wantLen {
				t.Errorf("FilterFilesByAvailability() length = %d, want %d", len(filtered), tt.wantLen)
			}

			if hasOffline != tt.wantHasOffline {
				t.Errorf("FilterFilesByAvailability() hasOffline = %v, want %v", hasOffline, tt.wantHasOffline)
			}

			for i, f := range filtered {
				if f.URI != tt.wantFilteredFiles[i] {
					t.Errorf("FilterFilesByAvailability() file[%d] = %q, want %q", i, f.URI, tt.wantFilteredFiles[i])
				}
			}
		})
	}
}
