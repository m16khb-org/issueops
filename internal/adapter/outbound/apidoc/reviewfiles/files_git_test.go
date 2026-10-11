package reviewfiles

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestExtraPromptFailsOnUnreadableExplicitFileAndIsEmptyWithoutSpec(t *testing.T) {
	repo := t.TempDir()
	if _, err := ExtraPrompt(repo, filepath.Join(repo, "absent.md")); err == nil {
		t.Fatal("an explicit prompt file that cannot be read must be an error, not a silent fallback")
	}
	if got, err := ExtraPrompt(repo, ""); err != nil || got != "" {
		t.Fatalf("no spec ExtraPrompt = %q, %v", got, err)
	}
}

func TestDiffRunsCachedGitDiffForTheFilesAndSurfacesFailures(t *testing.T) {
	var gotRepo string
	var gotArgs []string
	ok := Files{GitCmd: func(dir string, args ...string) (int, string, string) {
		gotRepo, gotArgs = dir, slices.Clone(args)
		return 0, "patch", ""
	}}
	out, err := ok.Diff("/repo", []string{"a.ts", "b.ts"}, "")
	if err != nil || out != "patch" {
		t.Fatalf("Diff = %q, %v", out, err)
	}
	if want := []string{"diff", "--cached", "--", "a.ts", "b.ts"}; gotRepo != "/repo" || !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("git invoked in %q with %v, want %v", gotRepo, gotArgs, want)
	}

	failing := Files{GitCmd: func(string, ...string) (int, string, string) { return 128, "", "fatal: bad revision" }}
	if _, err := failing.Diff("/repo", nil, ""); err == nil || !strings.Contains(err.Error(), "fatal: bad revision") {
		t.Fatalf("failed git diff must carry stderr, got %v", err)
	}
	if _, err := ok.Diff("/repo", nil, filepath.Join(t.TempDir(), "absent.patch")); err == nil {
		t.Fatal("unreadable diff file must be an error")
	}
	if _, err := failing.Input("/repo", nil, "", false); err == nil {
		t.Fatal("Input without --all must go through git and surface its failure")
	}
}

func TestInputAllWithDiffFilePrefersTheDiffFile(t *testing.T) {
	repo := t.TempDir()
	diff := writeRepoFile(t, repo, "d.patch", "the diff")
	got, err := (Files{}).Input(repo, []string{"missing.ts"}, diff, true)
	if err != nil || got != "the diff" {
		t.Fatalf("Input = %q, %v", got, err)
	}
}

func TestFullContentRejectsEscapesAndTerminatesEveryFileWithNewline(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, repo, "a.ts", "no newline")
	writeRepoFile(t, repo, "sub/b.ts", "with newline\n")
	writeRepoFile(t, repo, "empty.ts", "")

	got, err := FullContent(repo, []string{"a.ts", "./sub/b.ts", "empty.ts"})
	if err != nil {
		t.Fatal(err)
	}
	want := "\n--- FILE: a.ts ---\nno newline\n" + "\n--- FILE: " + filepath.Join("sub", "b.ts") + " ---\nwith newline\n" + "\n--- FILE: empty.ts ---\n\n"
	if got != want {
		t.Fatalf("FullContent =\n%q\nwant\n%q", got, want)
	}
	for _, unsafe := range []string{"..", "../x.ts", "/etc/passwd", "a/../../x.ts"} {
		if _, err := FullContent(repo, []string{unsafe}); err == nil || !strings.Contains(err.Error(), "unsafe file path") {
			t.Errorf("FullContent(%q) error = %v, want unsafe file path", unsafe, err)
		}
	}
	if _, err := FullContent(repo, []string{"absent.ts"}); err == nil {
		t.Fatal("missing file must be an error")
	}
	if got, err := FullContent(repo, nil); err != nil || got != "" {
		t.Fatalf("no files = %q, %v", got, err)
	}
}

func TestStagedAndTrackedListCandidatesAndTreatGitFailureAsEmpty(t *testing.T) {
	repo := t.TempDir()
	var stagedArgs []string
	reader := Files{GitCmd: func(_ string, args ...string) (int, string, string) {
		if args[0] == "ls-files" {
			return 0, "z.controller.ts\nREADME.md\n\n a/api.controller.ts \npackage-lock.json\n", ""
		}
		stagedArgs = slices.Clone(args)
		return 0, filepath.Join(repo, "src/u.controller.ts") + "\nREADME.md\nsrc/u.controller.ts\n../out.controller.ts\n", ""
	}}

	if got, want := reader.Tracked(repo), []string{"a/api.controller.ts", "z.controller.ts"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Tracked = %v, want %v", got, want)
	}
	if got, want := reader.Staged(repo), []string{"src/u.controller.ts"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Staged = %v, want %v", got, want)
	}
	if want := []string{"diff", "--cached", "--name-only", "--diff-filter=ACMR", "--"}; !reflect.DeepEqual(stagedArgs, want) {
		t.Fatalf("staged args = %v, want %v", stagedArgs, want)
	}

	failing := Files{GitCmd: func(string, ...string) (int, string, string) { return 1, "x.controller.ts\n", "boom" }}
	if got := failing.Staged(repo); got != nil {
		t.Fatalf("Staged on git failure = %v, want nil", got)
	}
	if got := failing.Tracked(repo); got != nil {
		t.Fatalf("Tracked on git failure = %v, want nil", got)
	}
}
