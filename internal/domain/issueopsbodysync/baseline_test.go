package issueopsbodysync

import (
	"testing"
	"time"

	contract "issueops/internal/contract/issueopsbodysync"
)

func TestSelectBaselinePrefersLastRecordedSync(t *testing.T) {
	issueURL := "https://gitlab.com/group/repo/-/issues/4"
	snapshot := BaselineSnapshot{
		Entries:        []BaselineEntry{{URL: issueURL, SHA256: "last", SyncedAt: "2026-09-27T00:00:00Z"}},
		IssueCreateURL: issueURL, IssueCreateSHA256: "created", IssueCreateAt: "2026-09-25T00:00:00Z",
	}
	sha, at := SelectBaseline(snapshot, contract.KindIssue, issueURL+"/")
	if sha != "last" || at != "2026-09-27T00:00:00Z" {
		t.Fatalf("sync baseline = %q %q", sha, at)
	}
	snapshot.Entries = nil
	sha, at = SelectBaseline(snapshot, contract.KindIssue, issueURL)
	if sha != "created" || at != "2026-09-25T00:00:00Z" {
		t.Fatalf("issue-create baseline = %q %q", sha, at)
	}
	snapshot.ArtifactVerifiedAt = "2026-09-26T00:00:00Z"
	sha, at = SelectBaseline(snapshot, contract.KindMR, "https://gitlab.com/group/repo/-/merge_requests/8")
	if sha != "" || at != snapshot.ArtifactVerifiedAt {
		t.Fatalf("publication baseline = %q %q", sha, at)
	}
}

func TestRetainedBaselineIndicesReplacesAliasAndCapsHistory(t *testing.T) {
	urls := make([]string, 0, 16)
	for index := 0; index < 16; index++ {
		urls = append(urls, "https://example.com/"+string(rune('a'+index)))
	}
	urls[0] = "https://gitlab.com/group/repo/-/work_items/4"
	kept := RetainedBaselineIndices(urls, "https://gitlab.com/group/repo/-/issues/4", 16)
	if len(kept) != 15 || kept[0] != 1 || kept[len(kept)-1] != 15 {
		t.Fatalf("retained indices = %v", kept)
	}
	kept = RetainedBaselineIndices(urls, "https://example.com/new", 16)
	if len(kept) != 15 || kept[0] != 1 || kept[len(kept)-1] != 15 {
		t.Fatalf("capped indices = %v", kept)
	}
}

func TestBaselineAgeDaysClampsFutureAndInvalidDates(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	if got := AgeDays("2026-09-25T00:00:00Z", now); got != 3 {
		t.Fatalf("age = %d, want 3", got)
	}
	if got := AgeDays("2026-09-29T00:00:00Z", now); got != 0 {
		t.Fatalf("future age = %d", got)
	}
	if got := AgeDays("invalid", now); got != 0 {
		t.Fatalf("invalid age = %d", got)
	}
}
