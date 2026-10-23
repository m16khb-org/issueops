package docs

import (
	docscontract "issueops/internal/contract/docs"
	"reflect"
	"testing"
)

func TestSelectPreservesAuthoringExclusionsAndFallback(t *testing.T) {
	paths := []string{"AGENTS.md", ".issueops/NEW.md", ".issueops/testing/unit.md", ".issueops/documentation/guide.md", ".issueops/research/메모.md", ".issueops/research/scratch.md", ".issueops/evidence/report.md", ".issueops/testing-other/scratch.md"}
	var candidates []docscontract.Candidate
	for _, p := range paths {
		candidates = append(candidates, docscontract.Candidate{Path: "/literal/symlink/" + p, RelPath: p})
	}
	for _, tc := range []struct {
		name      string
		tracked   map[string]bool
		available bool
		dirs      []string
		want      []int
	}{
		{"tracked and canonical", map[string]bool{"AGENTS.md": true, ".issueops/research/메모.md": true, ".issueops/evidence/report.md": true}, true, []string{".issueops/testing", "../escape", ".issueops/../../escape"}, []int{1, 3, 4, 2, 0}},
		{"git unavailable", nil, false, nil, []int{1, 3, 5, 4, 7, 2, 0}},
		{"invalid module", map[string]bool{"AGENTS.md": true}, true, []string{".issueops/testing/../../else"}, []int{1, 3, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := make([]string, 0, len(tc.want))
			for _, i := range tc.want {
				want = append(want, candidates[i].Path)
			}
			got := Select(candidates, tc.tracked, tc.available, tc.dirs)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v want %v", got, want)
			}
		})
	}
	t.Run("zero matches fallback", func(t *testing.T) {
		c := []docscontract.Candidate{{Path: "/literal/skills/self-verify/local.md", RelPath: "skills/self-verify/local.md"}}
		if got := Select(c, map[string]bool{"elsewhere": true}, true, nil); len(got) != 1 || got[0] != c[0].Path {
			t.Fatalf("fallback lost: %v", got)
		}
	})
}

func TestEmptySelectionPreservesGitAvailabilityShape(t *testing.T) {
	if got := Select(nil, nil, false, nil); got != nil {
		t.Fatalf("standalone empty list must be nil: %#v", got)
	}
	if got := Select(nil, map[string]bool{"unrelated": true}, true, nil); got == nil || len(got) != 0 {
		t.Fatalf("tracked empty list must be allocated: %#v", got)
	}
}

func TestHistoryRecordSeparatesDatedRecordsFromRules(t *testing.T) {
	for path, want := range map[string]bool{
		".issueops/issues/561/plan.md":        true,
		".issueops/research/agent-updates.md": true,
		".issueops/plans/brief.md":            true,
		".issueops/archive/issueops-audit.md": true,
		".issueops/verified-execution/548.md": true,
		// Dated decision and caution records are counted, not listed; their
		// family modules (overview, security, …) stay in the index.
		".issueops/cautions/2026-10-24-x.md":   true,
		".issueops/adr/2026-10-02-http.md":     true,
		".issueops/cautions/security.md":       false,
		".issueops/adr/roadmap.md":             false,
		".issueops/adr/2026-13-02-not-date.md": false,
		".issueops/adr/sub/2026-10-02-x.md":    false,
		".issueops/testing/unit.md":            false,
		".issueops/CAUTIONS.md":                false,
		".issueops/issues-guide.md":            false,
		"AGENTS.md":                            false,
		"skills/self-verify/SKILL.md":          false,
	} {
		if got := HistoryRecord(path); got != want {
			t.Errorf("HistoryRecord(%q) = %v, want %v", path, got, want)
		}
	}
}
