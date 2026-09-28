package issueops

import (
	"reflect"
	"testing"
)

func TestScopeGateLedgersJudgesOwnAndAnonymousPaths(t *testing.T) {
	root := "/repo"
	files := []string{
		"/repo/.issueops/issues/21/gates.md", "/repo/.issueops/issues/021/gates.md",
		"/repo/.issueops/issues/_unnumbered/gates.md", "/repo/.issueops/gates/issue-21.md",
		"/repo/.issueops/gates/248-other.md", "/repo/GATES.md",
	}
	judged, skipped := ScopeGateLedgers(root, files, "21")
	if want := []string{files[0], files[2], files[3], files[5]}; !reflect.DeepEqual(judged, want) {
		t.Fatalf("judged=%v, want %v", judged, want)
	}
	if want := []string{files[1], files[4]}; !reflect.DeepEqual(skipped, want) {
		t.Fatalf("skipped=%v, want %v", skipped, want)
	}
	if all, none := ScopeGateLedgers(root, files, ""); !reflect.DeepEqual(all, files) || len(none) != 0 {
		t.Fatalf("unknown issue judged=%v skipped=%v", all, none)
	}
}

func TestLegacyGateLedgerIssueNumberHonorsSchemaAndPrefix(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"issue-21.md", "21"}, {"21-cleanup.md", "21"}, {"021-plan.md", "021"},
		{"notes.md", ""}, {"21other.md", ""},
	} {
		if got := LegacyGateLedgerIssueNumber(tc.name, 1); got != tc.want {
			t.Fatalf("name %q: got %q want %q", tc.name, got, tc.want)
		}
	}
	if got := LegacyGateLedgerIssueNumber("issue-21.md", 2); got != "" {
		t.Fatalf("future schema compatibility=%q", got)
	}
}
