package filemetadata

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFileJSONPreservesPublicShape(t *testing.T) {
	file := File{
		URI:          "root://example/data.root",
		Size:         42,
		Checksum:     "adler32:12345678",
		Availability: "online",
		LocalPath:    "private/data.root",
	}

	data, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	jsonOutput := string(data)
	for _, expected := range []string{
		`"uri":"root://example/data.root"`,
		`"size":42`,
		`"checksum":"adler32:12345678"`,
		`"availability":"online"`,
	} {
		if !strings.Contains(jsonOutput, expected) {
			t.Errorf("JSON %s does not contain %s", jsonOutput, expected)
		}
	}
	if strings.Contains(jsonOutput, "local") || strings.Contains(jsonOutput, "private") {
		t.Errorf("internal local path leaked into JSON: %s", jsonOutput)
	}
}
