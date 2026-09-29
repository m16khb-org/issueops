package issueopsapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"issueops/cmd/issueops/issueopscli"
	adapter "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
)

func TestCleanupCLIInstancesKeepCapturedState(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir(), t.TempDir()}
	var deps [2]issueopscli.Dependencies
	var records [2]model.IssueOpsRecord
	var stateRoots [2]string
	repo := makeGitRepoForContract(t)
	for i := range deps {
		t.Setenv("ISSUEOPS_STATE_DIR", roots[i])
		stateRoots[i] = adapter.IssueOpsStateRoot()
		var err error
		records[i], err = startIssueOpsFixture(stateRoots[i], model.IssueOpsStartRequest{Repo: repo, Branch: "991-cleanup-instance"})
		if err != nil {
			t.Fatal(err)
		}
		deps[i] = issueOpsCLIDependencies()
	}
	t.Setenv("ISSUEOPS_STATE_DIR", roots[2])
	for _, i := range []int{0, 1, 0} {
		captureStdoutForContract(t, func() error {
			return issueopscli.RunIssueOpsWithDependencies([]string{"cleanup", "status", "--id", records[i].ID, "--json"}, deps[i])
		})
		raw := captureStdoutForContract(t, func() error {
			return issueopscli.RunIssueOpsWithDependencies([]string{"feedback", "add", "--id", records[i].ID, "--source", "instance", "--body", "captured state", "--classification", "contract_change", "--json"}, deps[i])
		})
		var updated model.IssueOpsRecord
		if err := json.Unmarshal([]byte(raw), &updated); err != nil {
			t.Fatal(err)
		}
		if updated.ID != records[i].ID {
			t.Fatalf("instance %d returned wrong record: %s", i, updated.ID)
		}
		captureStdoutForContract(t, func() error {
			return issueopscli.RunIssueOpsWithDependencies([]string{"feedback", "mark-issue-updated", "--id", records[i].ID, "--json"}, deps[i])
		})
	}
	for i, count := range []int{2, 1} {
		record, err := adapter.ReadIssueOps(stateRoots[i], records[i].ID)
		if err != nil || len(record.Feedback) != count {
			t.Fatalf("instance %d feedback count=%d want=%d err=%v", i, len(record.Feedback), count, err)
		}
		for _, feedback := range record.Feedback {
			if feedback.IssueUpdatedAt == "" {
				t.Fatalf("instance %d feedback not updated", i)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(roots[2], "issueops_v1")); !os.IsNotExist(err) {
		t.Fatalf("wrote into ambient state: %v", err)
	}
}
