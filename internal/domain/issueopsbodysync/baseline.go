package issueopsbodysync

import (
	"strings"
	"time"

	contract "issueops/internal/contract/issueopsbodysync"
)

// BaselineEntry records the SHA-256 and sync time of the last body written to one remote artifact URL.
type BaselineEntry struct {
	URL      string
	SHA256   string
	SyncedAt string
}

// BaselineSnapshot is the stored baseline state of a cycle: per-URL entries, the issue body recorded at creation, and the time the PR/MR artifact was verified.
type BaselineSnapshot struct {
	Entries            []BaselineEntry
	IssueCreateURL     string
	IssueCreateSHA256  string
	IssueCreateAt      string
	ArtifactVerifiedAt string
}

// SelectBaseline returns the SHA-256 and time of the body last known for url. It prefers a per-URL entry, then the issue-creation record for the issue kind. For a PR or MR with no entry it returns only the artifact verification time with an empty hash; otherwise both are empty.
func SelectBaseline(snapshot BaselineSnapshot, kind, url string) (sha, at string) {
	for _, entry := range snapshot.Entries {
		if sameArtifactURL(entry.URL, url) {
			return entry.SHA256, entry.SyncedAt
		}
	}
	if kind == contract.KindIssue && snapshot.IssueCreateURL != "" && sameArtifactURL(snapshot.IssueCreateURL, url) {
		return snapshot.IssueCreateSHA256, snapshot.IssueCreateAt
	}
	if IsPublicationKind(kind) && snapshot.ArtifactVerifiedAt != "" {
		return "", snapshot.ArtifactVerifiedAt
	}
	return "", ""
}

// RetainedBaselineIndices returns the indices of urls to keep after recording newURL. It drops entries for newURL and keeps the newest ones so that, with newURL, at most limit remain; it returns nil when limit is 1 or less.
func RetainedBaselineIndices(urls []string, newURL string, limit int) []int {
	kept := make([]int, 0, len(urls))
	for index, url := range urls {
		if !sameArtifactURL(url, newURL) {
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

// AgeDays returns the whole days between the RFC 3339 timestamp at and now. It returns 0 for an unparsable or future timestamp.
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
