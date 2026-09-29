package issueopsapp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"issueops/cmd/issueops/basiccli"
	adapter "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
)

func TestOperationalHealthInstancesKeepCapturedState(t *testing.T) {
	repo := makeGitRepoForContract(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir()
	if err := os.Symlink(git, filepath.Join(path, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", path)
	roots := []string{t.TempDir(), t.TempDir(), t.TempDir()}
	var commands [2]basiccli.Doctor
	var records [2]model.IssueOpsRecord
	for i := range commands {
		t.Setenv("ISSUEOPS_STATE_DIR", roots[i])
		records[i], err = startIssueOpsFixture(adapter.IssueOpsStateRoot(), model.IssueOpsStartRequest{Repo: repo, Branch: "991-health-isolation"})
		if err != nil {
			t.Fatal(err)
		}
		commands[i] = newDoctorCommand()
	}
	t.Setenv("ISSUEOPS_STATE_DIR", roots[2])
	for _, i := range []int{0, 1, 0} {
		snapshot := commands[i].CollectOperationalHealth(context.Background(), repo)
		if len(snapshot.Cycles) != 1 || snapshot.Cycles[0].ID != records[i].ID {
			t.Fatalf("instance %d lost captured lifecycle inventory: %+v", i, snapshot.Cycles)
		}
	}
}
