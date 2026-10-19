package issueopscli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	issueopsdomain "issueops/internal/domain/issueops"
)

// 계획 단계는 아키텍처를 먼저 요구한다. 계획이 먼저 스테이징돼 있어도, 승인한 뒤
// 문서를 고쳐도 next는 plan.architecture에 머문다.
func TestIssueOpsNextWalksArchitectureBeforePlan(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "example")
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.test"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "base")
	head := git("rev-parse", "HEAD")
	var started map[string]any
	if err := json.Unmarshal([]byte(captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-provider-linked-branch", "--json"})
	})), &started); err != nil {
		t.Fatal(err)
	}
	id := started["id"].(string)
	recordIssueOpsCLIIntentForTest(t, id)
	recordIssueOpsCLIPlanPrepForTest(t, id)
	captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-issue", "--id", id, "--issue-url", "https://github.com/example/repo/issues/1", "--json"})
	})
	captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"branch", "prepare", "--id", id, "--provider", "github", "--issue-url", "https://github.com/example/repo/issues/1", "--branch", "1-provider-linked-branch", "--base-branch", "main", "--base-sha", head, "--parent-worktree", repo + ".worktrees/main", "--link-verified", "--json"})
	})

	stage := func(name, body string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), name+".md")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		captureStdoutForContract(t, func() error {
			return runIssueOps([]string{"artifact", "stage", "--id", id, "--name", name, "--file", path, "--json"})
		})
	}
	next := func() (string, []string) {
		t.Helper()
		var result struct {
			Stage   struct{ Key string } `json:"stage"`
			Missing []string             `json:"missing"`
		}
		if err := json.Unmarshal([]byte(captureStdoutForContract(t, func() error {
			return runIssueOps([]string{"next", "--id", id, "--cwd", repo, "--json"})
		})), &result); err != nil {
			t.Fatal(err)
		}
		return result.Stage.Key, result.Missing
	}
	architecture := strings.Join(issueopsdomain.RequiredArchitectureSections, "\n본문\n") + "\n본문\n"

	stage("plan", "# plan\n"+strings.Join(issueopsdomain.RequiredPlanSections, "\n본문\n")+"\n본문\n")
	if key, missing := next(); key != "plan.architecture" || !slices.Equal(missing, []string{"architecture_artifact"}) {
		t.Fatalf("a staged plan must not skip the architecture: stage=%q missing=%v", key, missing)
	}

	stage("architecture", architecture)
	recordIssueOpsCLIArchitectureForTest(t, id)
	if key, missing := next(); key != "plan.review" {
		t.Fatalf("approved architecture plus staged plan must reach plan review: stage=%q missing=%v", key, missing)
	}

	stage("architecture", architecture+"추가한 절충점\n")
	if key, missing := next(); key != "plan.architecture" || !slices.Equal(missing, []string{"architecture_review_stale"}) {
		t.Fatalf("editing the architecture after approval must make it stale: stage=%q missing=%v", key, missing)
	}
}
