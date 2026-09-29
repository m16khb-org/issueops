package docs

import (
	docscontract "issueops/internal/contract/docs"
	"reflect"
	"testing"
)

func TestSelectPreservesAuthoringExclusionsAndFallback(t *testing.T) {
	paths := []string{"AGENTS.md", ".issueops/NEW.md", ".issueops/testing/unit.md", ".issueops/documentation/guide.md", ".issueops/research/메모.md", ".issueops/research/scratch.md", ".issueops/evidence/report.md", ".issueops/draft-wiki/draft.md", ".issueops/testing-other/scratch.md"}
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
		{"git unavailable", nil, false, nil, []int{1, 3, 5, 4, 8, 2, 0}},
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
