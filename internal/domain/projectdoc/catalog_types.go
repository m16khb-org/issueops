package projectdoc

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

type ProjectDocCatalogEntry struct {
	RelPath     string `json:"rel_path"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// CatalogOmissions counts what a catalog discovery pass left out so the omission
// is visible instead of silent. Oversize files exceed the per-file size cap,
// Unreadable files could not be opened or read, OverCap names lost the
// deterministic top-N selection, and HeaderTruncated documents were larger than
// the metadata header window and the window did not contain their full metadata.
// ScanTruncated marks a directory read that failed, so the result is partial.
type CatalogOmissions struct {
	Oversize        int  `json:"oversize,omitempty"`
	Unreadable      int  `json:"unreadable,omitempty"`
	OverCap         int  `json:"over_cap,omitempty"`
	HeaderTruncated int  `json:"header_truncated,omitempty"`
	ScanTruncated   bool `json:"scan_truncated,omitempty"`
}

// CatalogStats is the deterministic cost of one discovery pass: documents
// opened, body bytes read, and directory batches that returned entries.
type CatalogStats struct {
	FilesOpened int
	BytesRead   int
	DirBatches  int
}

func (o CatalogOmissions) Any() bool {
	return o != CatalogOmissions{}
}

// Summary lists only the non-zero omission counters as stable key=value tokens,
// or "" when nothing was omitted.
func (o CatalogOmissions) Summary() string {
	parts := []string{}
	for _, counter := range []struct {
		key   string
		value int
	}{{"oversize", o.Oversize}, {"unreadable", o.Unreadable}, {"over_cap", o.OverCap}, {"header_truncated", o.HeaderTruncated}} {
		if counter.value > 0 {
			parts = append(parts, counter.key+"="+strconv.Itoa(counter.value))
		}
	}
	if o.ScanTruncated {
		parts = append(parts, "scan_truncated")
	}
	return strings.Join(parts, " ")
}

func SHA256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
