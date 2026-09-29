package issueopsreview

import (
	contract "issueops/internal/contract/issueopsreview"
	"reflect"
	"strings"
	"testing"
)

func TestChangeBasePriorityAndEvidencePolicy(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name        string
		prepared    bool
		sha, branch string
		want        []contract.ChangeBaseCandidate
	}{
		{"unprepared", false, sha, "main", nil},
		{"prepared", true, " " + sha + " ", " main ", []contract.ChangeBaseCandidate{{Ref: sha, LiteralObject: true}, {Ref: "origin/main"}, {Ref: "main"}}},
		{"short hash", true, "abcdef", "main", []contract.ChangeBaseCandidate{{Ref: "origin/main"}, {Ref: "main"}}},
		{"no branch", true, sha, "", []contract.ChangeBaseCandidate{{Ref: sha, LiteralObject: true}}},
		{"no references", true, "", "", []contract.ChangeBaseCandidate{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ChangeBaseCandidates(tc.prepared, tc.sha, tc.branch); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got=%v want=%v", got, tc.want)
			}
		})
	}
	for _, value := range []string{strings.Repeat("0", 40), strings.Repeat("f", 64)} {
		if !FullGitObjectID(value) {
			t.Fatalf("valid object rejected %s", value)
		}
	}
	for _, value := range []string{"", strings.Repeat("A", 40), strings.Repeat("a", 39), strings.Repeat("g", 64)} {
		if FullGitObjectID(value) {
			t.Fatalf("invalid object accepted %s", value)
		}
	}
	if ImplementationChange("plan.md", true) || ImplementationChange("", false) || !ImplementationChange("src.go", false) {
		t.Fatal("plan-only evidence policy violated")
	}
	if ImplementationEvidenceMissing(true) != "" || ImplementationEvidenceMissing(false) != "implementation_changes" {
		t.Fatal("implementation evidence gate violated")
	}
	if !SameChangeSnapshot([]string{"a"}, []string{"a"}, "hash", "hash") || SameChangeSnapshot([]string{"a"}, []string{"b"}, "hash", "hash") || SameChangeSnapshot([]string{"a"}, []string{"a"}, "before", "after") {
		t.Fatal("snapshot must bind paths and content")
	}
}
