package issueopsapp

import (
	core "issueops/internal/adapter/issueops"
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
	originalRun := core.GitCmd
	t.Cleanup(func() { core.GitCmd = originalRun })
	core.GitCmd = func(string, ...string) (int, string, string) {
		t.Error("captured readiness used ambient Git runner")
		return 1, "", "ambient"
	}
	_, changes := readiness.ObserveLocalPR(model.IssueOpsRecord{Repo: repo, WorktreePath: repo})
	if !changes.Verified || changes.Fingerprint == "" {
		t.Fatalf("captured readiness lost actual snapshot: %+v", changes)
	}
}
