package issueopsapp

import (
	"os"
	"path/filepath"
	"testing"

	"issueops/cmd/issueops/hookcli"
	"issueops/cmd/issueops/hookcli/hookcatalog"
	pathutil "issueops/cmd/issueops/pathutil"
	hookcontract "issueops/internal/contract/hookprompt"
)

func TestResolveTarget(t *testing.T) {
	tmp := t.TempDir()
	got := pathutil.ResolveTarget(tmp)
	absTmp, _ := filepath.Abs(tmp)
	if got != absTmp {
		t.Fatalf("resolveTarget(%q) = %q, want %q", tmp, got, absTmp)
	}

	t.Run("with CLAUDE_PROJECT_DIR", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", tmp)
		t.Setenv("PWD", "")
		if res := pathutil.ResolveTarget(""); res != absTmp {
			t.Fatalf("expected %q, got %q", absTmp, res)
		}
	})

	t.Run("with PWD", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", "")
		t.Setenv("PWD", tmp)
		if res := pathutil.ResolveTarget(""); res != absTmp {
			t.Fatalf("expected %q, got %q", absTmp, res)
		}
	})

	t.Run("with neither env var", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", "")
		t.Setenv("PWD", "")
		cwd, _ := os.Getwd()
		absCwd, _ := filepath.Abs(cwd)
		if res := pathutil.ResolveTarget(""); res != absCwd {
			t.Fatalf("expected %q, got %q", absCwd, res)
		}
	})
}

func TestHookConfigsKeepIndependentDefaultRepositories(t *testing.T) {
	t.Setenv("ISSUEOPS_DISABLE_HOOKS", "")
	makeRepo := func(name string) string {
		repo := t.TempDir()
		if err := os.Mkdir(filepath.Join(repo, ".issueops"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".issueops", name), []byte("# Test document\n"), 0600); err != nil {
			t.Fatal(err)
		}
		return repo
	}
	aRepo, bRepo := makeRepo("ARCHITECTURE.md"), makeRepo("CONVENTIONS.md")
	t.Setenv("CLAUDE_PROJECT_DIR", aRepo)
	a := newHookConfig()
	t.Setenv("CLAUDE_PROJECT_DIR", bRepo)
	b := newHookConfig()
	t.Setenv("CLAUDE_PROJECT_DIR", t.TempDir())
	input, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	previous := os.Stdin
	os.Stdin = input
	t.Cleanup(func() { os.Stdin = previous })
	for _, tc := range []struct {
		config hookcatalog.Config
		want   string
	}{{a, ".issueops/ARCHITECTURE.md"}, {b, ".issueops/CONVENTIONS.md"}, {a, ".issueops/ARCHITECTURE.md"}} {
		var got hookcontract.ProjectDocCatalogContext
		config := tc.config
		config.PrintJSON = func(v any) error { got = v.(hookcontract.ProjectDocCatalogContext); return nil }
		for _, event := range []string{"session-start", "post-compact"} {
			if err := hookcli.RunHook([]string{event, "--json"}, config); err != nil {
				t.Fatal(err)
			}
			if !got.ShouldInject || len(got.ProjectDocs) != 1 || got.ProjectDocs[0].RelPath != tc.want {
				t.Fatalf("%s catalog = %+v, want %s", event, got, tc.want)
			}
		}
	}
}

// A session started in a subdirectory gets the repository root's catalog,
// byte for byte, so its system prefix (and prompt cache) matches root sessions.
func TestHookCatalogFromASubdirectoryMatchesTheRoot(t *testing.T) {
	t.Setenv("ISSUEOPS_DISABLE_HOOKS", "")
	repo := t.TempDir()
	for _, dir := range []string{".git", ".issueops", "src/pkg"} {
		if err := os.MkdirAll(filepath.Join(repo, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, ".issueops", "ARCHITECTURE.md"), []byte("# Test document\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	build := newHookConfig().BuildCatalog
	root, sub := build(repo), build(filepath.Join(repo, "src", "pkg"))
	if !sub.ShouldInject || sub.Compact != root.Compact || sub.UserView != root.UserView {
		t.Fatalf("subdirectory catalog = %+v, root = %+v", sub, root)
	}
}
