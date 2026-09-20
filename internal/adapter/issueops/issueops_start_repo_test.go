package issueops

import (
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/contract/issueops"
)

func TestStartIssueOpsStoresAbsoluteRepoWhenRelativePathProvided(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: ".", Branch: "12-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if record.Repo != repo || record.ID != newIssueOpsID(repo, "12-demo") {
		t.Fatalf("relative repo was not normalized: %+v", record)
	}
}

func TestStartIssueOpsExplicitNewCreatesDistinctBranchlessCycles(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()

	first, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, New: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, New: true})
	if err != nil {
		t.Fatal(err)
	}

	if first.ID == second.ID {
		t.Fatalf("explicit new branchless starts reused lifecycle %q", first.ID)
	}
	for _, record := range []issueops.IssueOpsRecord{first, second} {
		if record.Repo != repo || record.Branch != "" {
			t.Fatalf("new cycle changed source identity: %+v", record)
		}
		persisted, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatalf("read new cycle %s: %v", record.ID, err)
		}
		if persisted.ID != record.ID || persisted.CreatedAt != record.CreatedAt {
			t.Fatalf("new cycle %s was overwritten: got %+v want %+v", record.ID, persisted, record)
		}
	}
}

func TestStartIssueOpsDefaultStillResumesBranchlessCycle(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()

	first, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	newCycle, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, New: true})
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo})
	if err != nil {
		t.Fatal(err)
	}

	if resumed.ID != first.ID || resumed.CreatedAt != first.CreatedAt {
		t.Fatalf("ordinary branchless start did not resume: first=%+v resumed=%+v", first, resumed)
	}
	if newCycle.ID == first.ID {
		t.Fatalf("explicit new cycle reused resumable lifecycle %q", first.ID)
	}
	persisted, err := ReadIssueOps(stateRoot, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ID != first.ID || persisted.CreatedAt != first.CreatedAt {
		t.Fatalf("explicit new start overwrote resumable cycle: got %+v want %+v", persisted, first)
	}
}

func TestStartIssueOpsExplicitNewRequiresBranchlessRequest(t *testing.T) {
	if _, err := StartIssueOps(t.TempDir(), issueops.IssueOpsStartRequest{
		Repo: t.TempDir(), Branch: "123-already-named", New: true,
	}); err == nil {
		t.Fatal("explicit new start with a branch must fail closed")
	}
}
