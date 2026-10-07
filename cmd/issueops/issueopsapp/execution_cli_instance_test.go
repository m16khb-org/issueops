package issueopsapp

import (
	"context"
	"encoding/json"
	"issueops/cmd/issueops/issueopscli"
	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	"issueops/internal/testsupport"
	"os"
	"strings"
	"testing"
)

func TestExecutionCLIInstancesReadTheirCapturedGeneration(t *testing.T) {
	repo := makeGitRepoForContract(t)
	type instance struct {
		deps       issueopscli.Dependencies
		id         string
		generation uint64
	}
	var instances []instance
	for _, generation := range []uint64{7, 11} {
		t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
		root := issueOpsStateRoot()
		record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo})
		if err != nil {
			t.Fatal(err)
		}
		record.Execution = &model.Execution{Mode: model.ExecutionModeDirect, Workspace: model.Workspace{SourceRoot: repo, Root: t.TempDir(), Branch: "7-fixture", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-08-01T00:00:00Z"}, Lease: model.WriteLease{Generation: generation, Status: model.LeaseStatusReleased}, Completion: &model.ExecutionCompletion{Generation: generation, FinalHead: strings.Repeat("a", 40), VerificationReportPath: "verification.json", Verification: []string{"fixture"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/7", CompletedAt: "2026-08-01T00:00:00Z"}, Selection: selectionFixture(model.ExecutionModeDirect)}
		if _, err = (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		instances = append(instances, instance{issueOpsCLIDependencies(), record.ID, generation})
	}
	ambient := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", ambient)
	for _, i := range []int{0, 1, 0} {
		item := instances[i]
		output := testsupport.CaptureStdout(t, func() error {
			return issueopscli.RunIssueOpsWithDependencies([]string{"execution", "status", "--id", item.id, "--json"}, item.deps)
		})
		var result model.ExecutionResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatal(err)
		}
		if !result.OK || result.ID != item.id || result.Execution.Lease.Generation != item.generation {
			t.Fatalf("execution read crossed state roots: %+v want generation %d", result, item.generation)
		}
	}
	entries, err := os.ReadDir(ambient)
	if err != nil || len(entries) != 0 {
		t.Fatalf("ambient execution state touched: %v %v", entries, err)
	}
}
