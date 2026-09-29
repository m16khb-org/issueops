package issueopsapp

import (
	"encoding/json"
	"issueops/cmd/issueops/issueopscli"
	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	loopmodel "issueops/internal/contract/looprun"
	"issueops/internal/testsupport"
	"os"
	"slices"
	"testing"
)

// A CLI constructed for one state root must not follow later environment
// changes into another tenant's lifecycle store.
func TestLifecycleCLIInstancesKeepCapturedState(t *testing.T) {
	repo := makeGitRepoForContract(t)
	type instance struct {
		root string
		deps issueopscli.Dependencies
	}
	var instances []instance
	for range 2 {
		t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
		root := core.IssueOpsStateRoot()
		instances = append(instances, instance{root, issueopscli.Dependencies{Runtime: newIssueOpsCLIRuntime(root), Gates: newIssueOpsCLIGates()}})
	}
	ambient := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", ambient)
	for _, item := range instances {
		output := testsupport.CaptureStdout(t, func() error {
			return issueopscli.RunIssueOpsWithDependencies([]string{"start", "--repo", repo, "--json"}, item.deps)
		})
		var record model.IssueOpsRecord
		if err := json.Unmarshal([]byte(output), &record); err != nil {
			t.Fatal(err)
		}
		saved, err := core.ReadIssueOps(item.root, record.ID)
		if err != nil || saved.ID != record.ID || saved.Repo != repo {
			t.Fatalf("CLI wrote outside captured store: record=%+v saved=%+v err=%v", record, saved, err)
		}
		output = testsupport.CaptureStdout(t, func() error {
			return issueopscli.RunIssueOpsWithDependencies([]string{"pr-readiness", "--id", record.ID, "--json"}, item.deps)
		})
		var readiness model.IssueOpsReadiness
		if err := json.Unmarshal([]byte(output), &readiness); err != nil {
			t.Fatal(err)
		}
		if readiness.Ready || len(readiness.Missing) == 0 {
			t.Fatalf("new lifecycle unexpectedly ready: %+v", readiness)
		}
	}
	entries, err := os.ReadDir(ambient)
	if err != nil || len(entries) != 0 {
		t.Fatalf("ambient store touched: entries=%v err=%v", entries, err)
	}
}

func TestLifecyclePRGateKeepsCapturedLoopState(t *testing.T) {
	repo := makeGitRepoForContract(t)
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	loop, err := newLoopService().Start(loopmodel.StartLoopRequest{Repo: repo, Name: "ownership", Goal: "verify state ownership", MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	rootA := core.IssueOpsStateRoot()
	a := newIssueOpsCLIGates()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	rootB := core.IssueOpsStateRoot()
	b := newIssueOpsCLIGates()
	record := model.IssueOpsRecord{Repo: repo}
	for _, item := range []struct {
		root    string
		gates   issueopscli.LoopGateDeps
		blocked bool
	}{{rootA, a, true}, {rootB, b, false}, {rootA, a, true}} {
		ready := item.gates.StrictPRReadinessWithState(item.root, record)
		blocked := slices.Contains(ready.Missing, "loop_incomplete:"+loop.ID)
		if blocked != item.blocked {
			t.Fatalf("loop state ownership lost: root=%s blocked=%v want=%v readiness=%+v", item.root, blocked, item.blocked, ready)
		}
	}
}
