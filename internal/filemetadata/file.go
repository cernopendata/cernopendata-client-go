// Package filemetadata defines the file representation shared by discovery,
// filtering, downloading, and verification.
package filemetadata

// File describes one remote data file and its collision-free local path.
// LocalPath is an internal download concern and is intentionally excluded from
// user-facing JSON output.
type File struct {
	URI          string `json:"uri"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum"`
	Availability string `json:"availability,omitempty"`
	LocalPath    string `json:"-"`
}
