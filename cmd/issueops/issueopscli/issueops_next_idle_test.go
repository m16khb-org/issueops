package issueopscli

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

// TestCLIIssueOpsNextWarnsIdleCycle는 실제 next 명령 경로가 record에 8일째 쓰기가
// 없으면 정체 경고를 내고 6일이면 내지 않음을 잠근다.
func TestCLIIssueOpsNextWarnsIdleCycle(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := issueOpsStateRootForTest()
	repo := makeIssueOpsCLIRepoForTest(t, "example")
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	const idleMarker = "has had no record write for"

	for _, row := range []struct {
		id      string
		ago     time.Duration
		wantMsg string
	}{
		{"io-idle-8", 8 * 24 * time.Hour, idleMarker + " 8 days"},
		{"io-idle-6", 6 * 24 * time.Hour, ""},
	} {
		// CreatedAt 등은 LastRecordAt 후보가 아니다. UpdatedAt만 후보다.
		seedListRecord(t, root, issueopscontract.Record{
			SchemaVersion: issueopscontract.SchemaVersion,
			ID:            row.id, Repo: repo, Phase: issueopscontract.PhaseProblem,
			UpdatedAt: time.Now().UTC().Add(-row.ago).Format(time.RFC3339Nano),
		})
		var result struct {
			Warnings []string `json:"warnings"`
			Selected struct {
				ID string `json:"id"`
			} `json:"selected"`
		}
		out := captureStdoutForContract(t, func() error {
			return runIssueOps([]string{"next", "--id", row.id, "--cwd", repo, "--json"})
		})
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("next --json is not JSON: %v\n%s", err, out)
		}
		if result.Selected.ID != row.id {
			t.Fatalf("%s: selected = %q, want the seeded cycle:\n%s", row.id, result.Selected.ID, out)
		}
		warned := strings.Contains(strings.Join(result.Warnings, "\n"), row.wantMsg)
		if row.wantMsg != "" && !warned {
			t.Fatalf("%s: warnings lack %q:\n%s", row.id, row.wantMsg, out)
		}
		if row.wantMsg == "" && strings.Contains(strings.Join(result.Warnings, "\n"), idleMarker) {
			t.Fatalf("%s: unexpected idle warning:\n%s", row.id, out)
		}
	}
}
