package mcpcli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/osfile"
	authoritycontract "issueops/internal/contract/authority"
	"issueops/internal/testsupport"
)

func TestRequestScopeResolvesOnlyFromRequestAndRecord(t *testing.T) {
	repo, worktree, other := canonicalTempDir(t), canonicalTempDir(t), canonicalTempDir(t)
	nested := filepath.Join(worktree, "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(other)
	t.Setenv("PWD", other)
	t.Setenv("CLAUDE_PROJECT_DIR", other)
	scope := NewRequestScope(fakeScopeResolver{}, func(_ context.Context, kind RecordKind, id string) ([]string, error) {
		if kind == RecordIssueOps && id == "io-a" {
			return []string{repo, worktree, filepath.Join(repo, "missing")}, nil
		}
		return nil, errors.New("record not found")
	})
	for _, tc := range []struct {
		name      string
		tool      string
		args      map[string]any
		root, cwd string
		code      string
	}{
		{name: "explicit root", tool: "gates_check", args: map[string]any{"workspace_root": repo}, root: repo, cwd: repo},
		{name: "relative cwd joins root", tool: "gates_check", args: map[string]any{"workspace_root": worktree, "cwd": "pkg"}, root: worktree, cwd: nested},
		{name: "relative repo against absolute cwd", tool: "project_docs_route", args: map[string]any{"repo": ".", "cwd": repo}, root: repo, cwd: repo},
		{name: "relative repo without absolute cwd", tool: "project_docs_route", args: map[string]any{"repo": "."}, code: authoritycontract.CodeInvalid},
		{name: "repo and workspace_root agree", tool: "commit_suggest", args: map[string]any{"repo": repo, "workspace_root": repo}, root: repo, cwd: repo},
		{name: "repo and workspace_root conflict", tool: "commit_suggest", args: map[string]any{"repo": repo, "workspace_root": other}, code: authoritycontract.CodeInvalid},
		{name: "no root input", tool: "worker_run_read_only", args: map[string]any{}, code: authoritycontract.CodeRequired},
		{name: "record defaults to source repo", tool: "issueops_execution", args: map[string]any{"id": "io-a"}, root: repo, cwd: repo},
		{name: "record picks the worktree holding cwd", tool: "issueops_execution", args: map[string]any{"id": "io-a", "cwd": nested}, root: worktree, cwd: nested},
		{name: "explicit root must be a record root", tool: "issueops_execution", args: map[string]any{"id": "io-a", "workspace_root": other}, code: authoritycontract.CodeInvalid},
		{name: "cwd outside record workspaces", tool: "issueops_execution", args: map[string]any{"id": "io-a", "cwd": other}, code: authoritycontract.CodeInvalid},
		{name: "cwd outside explicit root", tool: "gates_check", args: map[string]any{"workspace_root": repo, "cwd": other}, code: authoritycontract.CodeInvalid},
		{name: "server tool has no scope", tool: "docs_index", args: map[string]any{}, code: authoritycontract.CodeInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := scope(t.Context(), tc.tool, tc.args)
			if tc.code != "" {
				if payload := authorityErrorPayload(err); err == nil || payload["error_code"] != tc.code {
					t.Fatalf("scope=%+v err=%v, want %s", got, err, tc.code)
				}
				return
			}
			if err != nil || got.WorkspaceRoot != tc.root || got.CWD != tc.cwd {
				t.Fatalf("scope=%+v err=%v, want root=%s cwd=%s", got, err, tc.root, tc.cwd)
			}
		})
	}
	if _, err := scope(t.Context(), "issueops_execution", map[string]any{"id": "io-missing"}); err == nil {
		t.Fatal("unknown record resolved a scope")
	}
}

func TestScopedArgumentsRewriteRootsCWDAndRelativeFiles(t *testing.T) {
	root := testsupport.AbsPath("repo")
	pkg := filepath.Join(root, "pkg")
	grant, abs, abs2 := testsupport.AbsPath("state", "grant"), testsupport.AbsPath("abs", "prompt.md"), testsupport.AbsPath("b.md")
	scope := authoritycontract.Scope{WorkspaceRoot: root, CWD: pkg}
	got := scopedArguments(map[string]any{
		"authority_file": grant, "repo": ".", "diff_file": "change.diff", "prompt_file": abs, "files": []any{"a.md", abs2},
	}, toolAuthority{rootArgs: []string{"repo"}, pathArgs: []string{"diff_file", "prompt_file", "files"}}, scope)
	if _, leaked := got["authority_file"]; leaked {
		t.Fatal("authority_file reached the tool handler")
	}
	files := got["files"].([]any)
	if got["repo"] != root || got["workspace_root"] != root || got["cwd"] != pkg ||
		got["diff_file"] != filepath.Join(pkg, "change.diff") || got["prompt_file"] != abs || files[0] != filepath.Join(pkg, "a.md") || files[1] != abs2 {
		t.Fatalf("scoped arguments = %v", got)
	}
}

func TestEnsureHTTPBearerIsOwnerOnlyStableAndRejectsUnsafeFiles(t *testing.T) {
	state := canonicalTempDir(t)
	first, err := EnsureHTTPBearer(state)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EnsureHTTPBearer(state)
	if err != nil || second != first || validateHTTPBearer(first) != nil {
		t.Fatalf("second=%q err=%v", second, err)
	}
	path := filepath.Join(state, "mcp-http", "bearer")
	for target, mode := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		info, err := os.Stat(target)
		if err != nil || osfile.Perm(target, info) != mode {
			t.Fatalf("%s mode=%v err=%v", target, osfile.Perm(target, info), err)
		}
	}
	testsupport.MakePermissive(t, path, 0o640)
	if _, err := EnsureHTTPBearer(state); !errors.Is(err, errHTTPBearerFile) {
		t.Fatalf("group-readable bearer err=%v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	decoy := filepath.Join(state, "decoy")
	if err := os.WriteFile(decoy, []byte(first), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(decoy, path); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureHTTPBearer(state); !errors.Is(err, errHTTPBearerFile) {
		t.Fatalf("symlinked bearer err=%v", err)
	}
}

// QA (opencode): the bare "conflicts with another workspace root input" did
// not name the inputs; the refusal names both roots and the remedy.
func TestExplicitRequestRootNamesBothConflictingRoots(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := explicitRequestRoot(map[string]any{"workspace_root": parent, "path": repo}, toolAuthority{rootArgs: []string{"path"}}, "")
	if err == nil || !strings.Contains(err.Error(), "workspace_root ") || !strings.Contains(err.Error(), "path ") || !strings.Contains(err.Error(), "name different roots") {
		t.Fatalf("err = %v", err)
	}
}
