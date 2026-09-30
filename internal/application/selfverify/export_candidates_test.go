package selfverify

import (
	"testing"
	"time"
)

func TestExportCandidatesObservesSourceOnceAndKeepsCatalogWhenMissing(t *testing.T) {
	for _, exists := range []bool{false, true} {
		calls := 0
		now := time.Date(2026, 9, 29, 12, 34, 56, 789, time.FixedZone("KST", 9*60*60))
		result := ExportCandidates("/repo", ExportCandidatesDeps{
			Source: func(root string) (string, bool) {
				calls++
				if root != "/repo" {
					t.Fatalf("root=%q", root)
				}
				return "/opaque/source", exists
			},
			Now: func() time.Time { return now },
		})
		if calls != 1 || result.SourcePath != "/opaque/source" || result.SourceExists != exists || result.IssueOpsRoot != "/repo" || result.GeneratedAt != now.UTC().Format(time.RFC3339Nano) {
			t.Fatalf("observation metadata drift: %+v calls=%d", result, calls)
		}
		if !result.OK || result.CandidateCount < 10 || len(result.Candidates) != result.CandidateCount || result.OpenCandidateIDs == nil || result.SatisfiedCandidateIDs == nil || result.Warnings == nil {
			t.Fatalf("export contract lost: %+v", result)
		}
		if exists {
			if len(result.Warnings) != 0 {
				t.Fatal(result.Warnings)
			}
		} else if len(result.Warnings) != 1 || result.Warnings[0] != "skills/self-verify/CANDIDATES.md not found; using built-in candidate export catalog" {
			t.Fatal(result.Warnings)
		}
	}
}
