package preflight

import (
	"reflect"
	"testing"
)

func TestAnalyzeStatusPreservesGitPorcelainClassification(t *testing.T) {
	got := AnalyzeStatus([]string{
		"## main...origin/main", " M plain.txt", "M  staged.txt", "?? .env", "R  old -> secrets/key.pem", "?? .env", "x",
	})
	if !reflect.DeepEqual(got.Staged, []string{"secrets/key.pem", "staged.txt"}) ||
		!reflect.DeepEqual(got.Unstaged, []string{"plain.txt"}) ||
		!reflect.DeepEqual(got.Untracked, []string{".env"}) ||
		!reflect.DeepEqual(got.SecretLike, []string{".env", "secrets/key.pem"}) {
		t.Fatalf("unexpected status classification: %+v", got)
	}
	if got.IsClean {
		t.Fatal("dirty status classified clean")
	}
}

func TestWarningsPreserveOrder(t *testing.T) {
	got := Warnings("", false, []string{".env"})
	want := []string{"detached_head", "no_upstream", "secret_like_paths_present"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("warnings = %v, want %v", got, want)
	}
	if got := Warnings("main", true, nil); got == nil || len(got) != 0 {
		t.Fatalf("clean warnings = %v", got)
	}
}
