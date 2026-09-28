package issueopsbodysync

import (
	"strings"
	"time"

	contract "issueops/internal/contract/issueopsbodysync"
)

type BaselineEntry struct {
	URL      string
	SHA256   string
	SyncedAt string
}

type BaselineSnapshot struct {
	Entries            []BaselineEntry
	IssueCreateURL     string
	IssueCreateSHA256  string
	IssueCreateAt      string
	ArtifactVerifiedAt string
}

func SelectBaseline(snapshot BaselineSnapshot, kind, url string) (sha, at string) {
	for _, entry := range snapshot.Entries {
		if SameArtifactURL(entry.URL, url) {
			return entry.SHA256, entry.SyncedAt
		}
	}
	if kind == contract.KindIssue && snapshot.IssueCreateURL != "" && SameArtifactURL(snapshot.IssueCreateURL, url) {
		return snapshot.IssueCreateSHA256, snapshot.IssueCreateAt
	}
	if IsPublicationKind(kind) && snapshot.ArtifactVerifiedAt != "" {
		return "", snapshot.ArtifactVerifiedAt
	}
	return "", ""
}

func RetainedBaselineIndices(urls []string, newURL string, limit int) []int {
	kept := make([]int, 0, len(urls))
	for index, url := range urls {
		if !SameArtifactURL(url, newURL) {
			kept = append(kept, index)
		}
	}
	if limit <= 1 {
		return nil
	}
	if len(kept) >= limit {
		kept = kept[len(kept)-(limit-1):]
	}
	return kept
}

func AgeDays(at string, now time.Time) int {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(at))
	if err != nil {
		return 0
	}
	days := int(now.Sub(parsed).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
