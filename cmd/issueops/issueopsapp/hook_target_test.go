package issueopsapp

import (
	"issueops/cmd/issueops/hookcli"
	"issueops/cmd/issueops/hookcli/hookcatalog"
	hookcontract "issueops/internal/contract/hookprompt"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTarget(t *testing.T) {
	tmp := t.TempDir()
	got := resolveTarget(tmp)
	absTmp, _ := filepath.Abs(tmp)
	if got != absTmp {
		t.Fatalf("resolveTarget(%q) = %q, want %q", tmp, got, absTmp)
	}

	t.Run("with CLAUDE_PROJECT_DIR", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", tmp)
		t.Setenv("PWD", "")
		if res := resolveTarget(""); res != absTmp {
			t.Fatalf("expected %q, got %q", absTmp, res)
		}
	})

	t.Run("with PWD", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", "")
		t.Setenv("PWD", tmp)
		if res := resolveTarget(""); res != absTmp {
			t.Fatalf("expected %q, got %q", absTmp, res)
		}
	})

	t.Run("with neither env var", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", "")
		t.Setenv("PWD", "")
		cwd, _ := os.Getwd()
		absCwd, _ := filepath.Abs(cwd)
		if res := resolveTarget(""); res != absCwd {
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
