package reviewfiles

import (
	domain "issueops/internal/domain/apidoc"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtraPromptUsesExplicitFileThenRepoDefault(t *testing.T) {
	repo := t.TempDir()
	explicit := filepath.Join(repo, "prompt.md")
	if err := os.WriteFile(explicit, []byte("explicit"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ExtraPrompt(repo, explicit)
	if err != nil || got != "explicit" {
		t.Fatalf("explicit ExtraPrompt = %q, %v", got, err)
	}
	specDir := filepath.Join(repo, ".issueops")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "OPEN_API_SPEC.md"), []byte("default"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = ExtraPrompt(repo, "")
	if err != nil || got != "default" {
		t.Fatalf("default ExtraPrompt = %q, %v", got, err)
	}
}

func TestInputAndFullContentUseDiffFileOrSafeFullContent(t *testing.T) {
	repo := t.TempDir()
	diff := filepath.Join(repo, "diff.patch")
	if err := os.WriteFile(diff, []byte("diff"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := (Files{}).Input(repo, []string{"api.ts"}, diff, false)
	if err != nil || got != "diff" {
		t.Fatalf("diff input = %q, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(repo, "api.ts"), []byte("export {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = (Files{}).Input(repo, []string{"api.ts"}, "", true)
	if err != nil {
		t.Fatalf("full input returned error: %v", err)
	}
	if !strings.Contains(got, "--- FILE: api.ts ---") || !strings.Contains(got, "export {}") {
		t.Fatalf("unexpected full content: %q", got)
	}
	if _, err := FullContent(repo, []string{"../outside.ts"}); err == nil {
		t.Fatal("expected unsafe path error")
	}
}

func TestNormalizeAndCandidateFiltering(t *testing.T) {
	repo := t.TempDir()
	abs := filepath.Join(repo, "src", "user.controller.ts")
	got := Normalize(repo, []string{" README.md ", "api/schema.json", abs, "../escape.ts", "api/schema.json", "package-lock.json"})
	want := []string{"api/schema.json", "src/user.controller.ts"}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
	if !domain.IsCandidate("src/user.controller.ts") || !domain.IsCandidate("openapi.yaml") {
		t.Fatal("expected API-like files to be candidates")
	}
	if domain.IsCandidate("README.md") || domain.IsCandidate("package.json") || domain.IsCandidate("pnpm-lock.yaml") {
		t.Fatal("expected docs/package metadata to be filtered")
	}
	lines := splitLines(" a \n\n b \n")
	if len(lines) != 2 || lines[0] != "a" || lines[1] != "b" {
		t.Fatalf("unexpected splitLines result %#v", lines)
	}
}

func TestFileReadersKeepTheirGitRunners(t *testing.T) {
	makeReader := func(name string) Files {
		return Files{GitCmd: func(repo string, args ...string) (int, string, string) {
			if repo != name {
				t.Fatalf("%s reader received repo %s", name, repo)
			}
			if len(args) == 1 || len(args) > 2 && args[2] == "--name-only" {
				return 0, name + ".openapi.yaml\n", ""
			}
			return 0, name + "-diff", ""
		}}
	}
	a, b := makeReader("first"), makeReader("second")
	for _, tc := range []struct {
		name   string
		reader Files
	}{{"first", a}, {"second", b}, {"first", a}} {
		for _, list := range []func(string) []string{tc.reader.Staged, tc.reader.Tracked} {
			got := list(tc.name)
			if len(got) != 1 || got[0] != tc.name+".openapi.yaml" {
				t.Fatalf("%s files=%v", tc.name, got)
			}
		}
		got, err := tc.reader.Input(tc.name, []string{tc.name + ".openapi.yaml"}, "", false)
		if err != nil || got != tc.name+"-diff" {
			t.Fatalf("%s input=%s err=%v", tc.name, got, err)
		}
	}
}
