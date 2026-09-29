package issueopsapp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/cmd/issueops/mcpcli"
	commitmodel "issueops/internal/contract/commitsuggest"
	inspectmodel "issueops/internal/contract/inspect"
	lintmodel "issueops/internal/contract/lintdiagnose"
)

func TestMCPAssistantServicesKeepCapturedRepositoryAndState(t *testing.T) {
	roots := []string{makeGitRepoForContract(t), makeGitRepoForContract(t)}
	deps := make([]mcpcli.MCPDependencies, 2)
	states := []string{t.TempDir(), t.TempDir()}
	for i, root := range roots {
		t.Chdir(root)
		t.Setenv("ISSUEOPS_ROOT", root)
		t.Setenv("ISSUEOPS_STATE_DIR", states[i])
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(fmt.Sprintf("instance-%d\n", i)), 0600); err != nil {
			t.Fatal(err)
		}
		deps[i] = issueOpsMCPDependencies()
	}
	t.Chdir(t.TempDir())
	t.Setenv("ISSUEOPS_ROOT", t.TempDir())
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	for i, d := range deps {
		if d.Resources.IssueOpsRoot != roots[i] || !strings.HasPrefix(d.Execution.IssueOpsStateRoot(), states[i]) {
			t.Fatalf("instance %d root=%s state=%s", i, d.Resources.IssueOpsRoot, d.Execution.IssueOpsStateRoot())
		}
		info := d.Inspect("").(inspectmodel.InspectInfo)
		if info.IssueOpsRoot != roots[i] || info.TargetRepo != roots[i] {
			t.Fatalf("instance %d inspect=%+v", i, info)
		}
		result, err := d.Commit.Suggest(commitmodel.CommitSuggestRequest{RepoRoot: "."})
		if err != nil || !result.Executed || result.RepoRoot != roots[i] || !strings.Contains(result.Prompt, fmt.Sprintf("instance-%d", i)) {
			t.Fatalf("instance %d commit=%+v err=%v", i, result, err)
		}
		lint, err := d.Lint.Diagnose(lintmodel.LintDiagnoseRequest{RepoRoot: ".", CommandArgv: []string{"/bin/sh", "-c", "cat README.md; exit 2"}})
		if err != nil || !lint.Failed || !strings.Contains(lint.Prompt, fmt.Sprintf("instance-%d", i)) {
			t.Fatalf("instance %d lint=%+v err=%v", i, lint, err)
		}
	}
}
