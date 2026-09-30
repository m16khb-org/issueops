package issueopsapp

import (
	model "issueops/internal/contract/issueops"
	"os"
	"path/filepath"
	"testing"
)

func TestCycleReadinessKeepsCapturedGitCapabilities(t *testing.T) {
	repo := makeGitRepoForContract(t)
	if err := os.WriteFile(filepath.Join(repo, "change.go"), []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	readiness := newCycleReadiness()
	t.Chdir(t.TempDir())
	_, changes := readiness.ObserveLocalPR(model.IssueOpsRecord{Repo: repo, WorktreePath: repo})
	if !changes.Verified || changes.Fingerprint == "" {
		t.Fatalf("captured readiness lost actual snapshot: %+v", changes)
	}
}
