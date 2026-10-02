package issueopsapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/cmd/issueops/mcpcli"
	issueopscore "issueops/internal/adapter/issueops"
	statestore "issueops/internal/adapter/outbound/state"
)

const (
	apiDocOutsideMarker = "REVIEW_OUTSIDE_SCOPE_MARKER_7581"
	apiDocInsideMarker  = "REVIEW_INSIDE_SCOPE_MARKER_4420"
)

func TestMCPHTTPAPIDocFilesStayInsideAuthorizedWorkspace(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	serverCWD := chdirSharedServer(t)
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outsidePrompt := filepath.Join(outside, "outside-prompt.md")
	outsideController := filepath.Join(outside, "outside.controller.ts")
	for path, content := range map[string]string{
		outsidePrompt:     apiDocOutsideMarker + "\n",
		outsideController: "// " + apiDocOutsideMarker + "\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repoA := gitRepoForHTTPTest(t)
	for name, content := range map[string]string{
		"api.diff":                 "diff --git a/a.controller.ts b/a.controller.ts\n+// " + apiDocInsideMarker + "\n",
		"inside-prompt.md":         apiDocInsideMarker + "\n",
		"inside.controller.ts":     "// " + apiDocInsideMarker + "\n",
		"sub/inside-prompt.md":     apiDocInsideMarker + "\n",
		"result.json":              `{"verdict":"pass","findings":[]}`,
		"sub/unrelated-readme.txt": "unused\n",
	} {
		full := filepath.Join(repoA, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{
		"escape":                    outside,
		"linked-prompt.md":          outsidePrompt,
		"linked-diff.diff":          outsidePrompt,
		"linked.controller.ts":      outsideController,
		"linked-result.json":        outsidePrompt,
		"inside-link-to-prompt.md":  filepath.Join(repoA, "inside-prompt.md"),
		"inside-link.controller.ts": filepath.Join(repoA, "inside.controller.ts"),
		"inside-linked-result.json": filepath.Join(repoA, "result.json"),
		"inside-linked-api.diff":    filepath.Join(repoA, "api.diff"),
	} {
		if err := os.Symlink(target, filepath.Join(repoA, link)); err != nil {
			t.Fatal(err)
		}
	}
	self, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	grantA := authorizeSessionForTest(t, repoA, "client-a", self)
	url, _ := startProductionHTTPServer(t, issueOpsMCPHTTPDependencies())
	bearer, err := mcpcli.EnsureHTTPBearer(statestore.StateDir())
	if err != nil {
		t.Fatal(err)
	}
	client := connectHTTPClient(t, url, bearer, "")
	relativeOutside, err := filepath.Rel(repoA, outsidePrompt)
	if err != nil {
		t.Fatal(err)
	}
	base := func(extra map[string]any) map[string]any {
		args := map[string]any{"authority_file": grantA, "repo": repoA, "diff_file": "api.diff"}
		for key, value := range extra {
			if value == nil {
				delete(args, key)
				continue
			}
			args[key] = value
		}
		return args
	}

	for _, tc := range []struct {
		name string
		tool string
		args map[string]any
	}{
		{"prompt dotdot", "api_doc_review", base(map[string]any{"prompt_file": "../outside-prompt.md"})},
		{"prompt dotdot to real sibling", "api_doc_review", base(map[string]any{"prompt_file": relativeOutside})},
		{"prompt absolute", "api_doc_review", base(map[string]any{"prompt_file": outsidePrompt})},
		{"prompt through symlink dir", "api_doc_review", base(map[string]any{"prompt_file": "escape/outside-prompt.md"})},
		{"prompt symlink file", "api_doc_review", base(map[string]any{"prompt_file": "linked-prompt.md"})},
		{"prompt absolute through symlink dir", "api_doc_review", base(map[string]any{"prompt_file": filepath.Join(repoA, "escape", "outside-prompt.md")})},
		{"diff dotdot", "api_doc_review", base(map[string]any{"diff_file": relativeOutside})},
		{"diff absolute", "api_doc_review", base(map[string]any{"diff_file": outsidePrompt})},
		{"diff symlink file", "api_doc_review", base(map[string]any{"diff_file": "linked-diff.diff"})},
		{"result dotdot", "api_doc_review", base(map[string]any{"result_file": relativeOutside})},
		{"result absolute", "api_doc_review", base(map[string]any{"result_file": outsidePrompt})},
		{"result symlink file", "api_doc_review", base(map[string]any{"result_file": "linked-result.json"})},
		{"files absolute", "api_doc_review", base(map[string]any{"all": true, "diff_file": nil, "files": []any{outsideController}})},
		{"files dotdot", "api_doc_review", base(map[string]any{"all": true, "diff_file": nil, "files": []any{"../" + filepath.Base(outside) + "/outside.controller.ts"}})},
		{"files symlink file", "api_doc_review", base(map[string]any{"all": true, "diff_file": nil, "files": []any{"linked.controller.ts"}})},
		{"static files absolute", "api_doc_static_check", map[string]any{"authority_file": grantA, "repo": repoA, "files": []any{outsideController}}},
		{"static files dotdot", "api_doc_static_check", map[string]any{"authority_file": grantA, "repo": repoA, "files": []any{"../" + filepath.Base(outside) + "/outside.controller.ts"}}},
		{"static files symlink file", "api_doc_static_check", map[string]any{"authority_file": grantA, "repo": repoA, "files": []any{"linked.controller.ts"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, isError := callHTTPTool(t, client, tc.tool, tc.args)
			encoded := payloadText(payload)
			if strings.Contains(encoded, apiDocOutsideMarker) {
				t.Fatalf("%s returned content from outside the authorized workspace: %s", tc.tool, encoded)
			}
			if !isError || payload["error_code"] != "authority_invalid" {
				t.Fatalf("%s accepted an outside file: isError=%v payload=%s", tc.tool, isError, encoded)
			}
		})
	}

	payload, isError := callHTTPTool(t, client, "api_doc_review", base(map[string]any{"prompt_file": "inside-prompt.md"}))
	if encoded := payloadText(payload); isError || !strings.Contains(encoded, apiDocInsideMarker) || strings.Contains(encoded, apiDocOutsideMarker) {
		t.Fatalf("inside prompt/diff files no longer work: isError=%v payload=%s", isError, encoded)
	}
	payload, isError = callHTTPTool(t, client, "api_doc_review", base(map[string]any{"prompt_file": "../" + filepath.Base(repoA) + "/inside-prompt.md"}))
	if encoded := payloadText(payload); isError || !strings.Contains(encoded, apiDocInsideMarker) {
		t.Fatalf("dotdot that stays inside the workspace was rejected: isError=%v payload=%s", isError, encoded)
	}
	payload, isError = callHTTPTool(t, client, "api_doc_review", base(map[string]any{"prompt_file": filepath.Join(repoA, "sub", "inside-prompt.md"), "diff_file": filepath.Join(repoA, "api.diff")}))
	if encoded := payloadText(payload); isError || !strings.Contains(encoded, apiDocInsideMarker) {
		t.Fatalf("absolute inside files were rejected: isError=%v payload=%s", isError, encoded)
	}
	payload, isError = callHTTPTool(t, client, "api_doc_review", base(map[string]any{"prompt_file": "inside-link-to-prompt.md", "diff_file": "inside-linked-api.diff"}))
	if encoded := payloadText(payload); isError || !strings.Contains(encoded, apiDocInsideMarker) {
		t.Fatalf("symlinks that stay inside the workspace were rejected: isError=%v payload=%s", isError, encoded)
	}
	payload, isError = callHTTPTool(t, client, "api_doc_review", base(map[string]any{"all": true, "diff_file": nil, "files": []any{"inside.controller.ts"}}))
	if encoded := payloadText(payload); isError || !strings.Contains(encoded, apiDocInsideMarker) {
		t.Fatalf("inside files were rejected: isError=%v payload=%s", isError, encoded)
	}
	requireEmptyDir(t, serverCWD)
}
