package issueopscli

import (
	"encoding/json"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestRunIssueOpsStartNewCreatesIndependentBranchlessCycles(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	start := func() issueopscontract.IssueOpsRecord {
		t.Helper()
		output := captureStdoutForContract(t, func() error {
			return runIssueOps([]string{"start", "--repo", repo, "--new", "--json"})
		})
		var record issueopscontract.IssueOpsRecord
		if err := json.Unmarshal([]byte(output), &record); err != nil {
			t.Fatalf("decode start output %q: %v", output, err)
		}
		return record
	}

	first := start()
	second := start()
	if first.ID == second.ID {
		t.Fatalf("--new reused branchless lifecycle %q", first.ID)
	}
	if first.Repo != repo || second.Repo != repo || first.Branch != "" || second.Branch != "" {
		t.Fatalf("--new changed source identity: first=%+v second=%+v", first, second)
	}
}
