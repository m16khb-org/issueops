package issueops

import (
	"path/filepath"
	"reflect"
	"testing"

	"issueops/internal/testsupport"
)

func TestScopeGateLedgersJudgesOwnAndAnonymousPaths(t *testing.T) {
	root := testsupport.AbsPath("repo")
	issues := filepath.Join(root, ".issueops", "issues")
	files := []string{
		filepath.Join(issues, "21", "gates.md"), filepath.Join(issues, "021", "gates.md"),
		filepath.Join(issues, "_unnumbered", "gates.md"), filepath.Join(root, ".issueops", "gates", "cleanup.md"),
		filepath.Join(issues, "248", "gates.md"),
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
