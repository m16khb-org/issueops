package issueops

import (
	"reflect"
	"testing"
)

func TestScopeGateLedgersJudgesOwnAndAnonymousPaths(t *testing.T) {
	root := "/repo"
	files := []string{
		"/repo/.issueops/issues/21/gates.md", "/repo/.issueops/issues/021/gates.md",
		"/repo/.issueops/issues/_unnumbered/gates.md", "/repo/.issueops/gates/cleanup.md",
		"/repo/.issueops/issues/248/gates.md",
	}
	judged, skipped := ScopeGateLedgers(root, files, "21")
	if want := []string{files[0], files[2], files[3]}; !reflect.DeepEqual(judged, want) {
		t.Fatalf("judged=%v, want %v", judged, want)
	}
	if want := []string{files[1], files[4]}; !reflect.DeepEqual(skipped, want) {
		t.Fatalf("skipped=%v, want %v", skipped, want)
	}
	if all, none := ScopeGateLedgers(root, files, ""); !reflect.DeepEqual(all, files) || len(none) != 0 {
		t.Fatalf("unknown issue judged=%v skipped=%v", all, none)
	}
}
