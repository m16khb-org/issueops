package mcpcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	authoritycontract "issueops/internal/contract/authority"
)

func TestRequestScopeConfinesAPIDocFilesToWorkspace(t *testing.T) {
	repo, outside := canonicalTempDir(t), canonicalTempDir(t)
	sub := filepath.Join(repo, "sub")
	for _, dir := range []string{sub, filepath.Join(repo, "real")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{filepath.Join(repo, "in.md"), filepath.Join(sub, "in.md"), filepath.Join(outside, "out.md")} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{
		filepath.Join(repo, "dir-escape"):      outside,
		filepath.Join(repo, "file-escape.md"):  filepath.Join(outside, "out.md"),
		filepath.Join(repo, "dangling.md"):     filepath.Join(outside, "missing.md"),
		filepath.Join(repo, "inside-link.md"):  filepath.Join(repo, "in.md"),
		filepath.Join(repo, "real-alias"):      filepath.Join(repo, "real"),
		filepath.Join(repo, "real", "link.md"): filepath.Join(repo, "in.md"),
	} {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	scope := NewRequestScope(fakeScopeResolver{}, nil)
	for _, tc := range []struct {
		name    string
		tool    string
		args    map[string]any
		allowed bool
	}{
		{"prompt inside", "api_doc_review", map[string]any{"repo": repo, "prompt_file": "in.md"}, true},
		{"prompt nested cwd", "api_doc_review", map[string]any{"repo": repo, "cwd": sub, "prompt_file": "in.md"}, true},
		{"prompt absolute inside", "api_doc_review", map[string]any{"repo": repo, "prompt_file": filepath.Join(repo, "in.md")}, true},
		{"prompt inside symlink", "api_doc_review", map[string]any{"repo": repo, "prompt_file": "inside-link.md"}, true},
		{"prompt through inside dir alias", "api_doc_review", map[string]any{"repo": repo, "prompt_file": "real-alias/link.md"}, true},
		{"prompt missing inside", "api_doc_review", map[string]any{"repo": repo, "prompt_file": "missing.md"}, true},
		{"result and files inside", "api_doc_review", map[string]any{"repo": repo, "result_file": "in.md", "files": []any{"in.md", "sub/in.md"}}, true},
		{"static files inside", "api_doc_static_check", map[string]any{"repo": repo, "files": []any{"in.md"}}, true},
		{"prompt dotdot", "api_doc_review", map[string]any{"repo": repo, "prompt_file": "../" + filepath.Base(outside) + "/out.md"}, false},
		{"prompt dotdot from cwd", "api_doc_review", map[string]any{"repo": repo, "cwd": sub, "prompt_file": "../../x/out.md"}, false},
		{"prompt absolute outside", "api_doc_review", map[string]any{"repo": repo, "prompt_file": filepath.Join(outside, "out.md")}, false},
		{"prompt missing outside", "api_doc_review", map[string]any{"repo": repo, "prompt_file": filepath.Join(outside, "missing.md")}, false},
		{"prompt dir symlink escape", "api_doc_review", map[string]any{"repo": repo, "prompt_file": "dir-escape/out.md"}, false},
		{"prompt file symlink escape", "api_doc_review", map[string]any{"repo": repo, "prompt_file": "file-escape.md"}, false},
		{"prompt dangling symlink escape", "api_doc_review", map[string]any{"repo": repo, "prompt_file": "dangling.md"}, false},
		{"diff absolute outside", "api_doc_review", map[string]any{"repo": repo, "diff_file": filepath.Join(outside, "out.md")}, false},
		{"result symlink escape", "api_doc_review", map[string]any{"repo": repo, "result_file": "file-escape.md"}, false},
		{"files dotdot", "api_doc_review", map[string]any{"repo": repo, "files": []any{"in.md", "../x.ts"}}, false},
		{"static files absolute outside", "api_doc_static_check", map[string]any{"repo": repo, "files": []any{filepath.Join(outside, "out.md")}}, false},
		{"static files symlink escape", "api_doc_static_check", map[string]any{"repo": repo, "files": []any{"file-escape.md"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := scope(context.Background(), tc.tool, tc.args)
			if tc.allowed {
				if err != nil {
					t.Fatalf("inside file rejected: %v", err)
				}
				return
			}
			if payload := authorityErrorPayload(err); err == nil || payload["error_code"] != authoritycontract.CodeInvalid {
				t.Fatalf("outside file accepted or wrong code: err=%v", err)
			}
		})
	}
	other, err := scope(context.Background(), "gates_check", map[string]any{"workspace_root": repo, "files": []any{filepath.Join(outside, "out.md")}})
	if err != nil || other.WorkspaceRoot != repo {
		t.Fatalf("tools without confinement changed behavior: %+v %v", other, err)
	}
}
