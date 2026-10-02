package issueopsapp

import (
	"os"
	"strings"
	"testing"

	"issueops/cmd/issueops/mcpcli"
	issueopscore "issueops/internal/adapter/issueops"
	statestore "issueops/internal/adapter/outbound/state"
	authoritycontract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
)

// A valid capability must reach the same resume/reseed domain decision as a
// native stdio caller with the same fixture. The lifecycle fence database is
// not the grant database, so it must not be where the grant is rechecked.
func TestMCPHTTPCapabilityReachesResumeAndReseedDomainDecisions(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := gitRepoForHTTPTest(t)
	self, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	grant := authorizeSessionForTest(t, repo, "client-a", self)
	holder := model.NativeActor{Host: "codex", SessionID: "client-a", SessionProcess: &self}
	worktree := gitWorktreeForHTTPTest(t, repo, "300-http-lease")
	record := seedHTTPLeaseRecord(t, repo, worktree, holder)

	native := holder
	native.ProcessAncestry = []model.NativeProcessReceipt{self}
	_, resumeErr := issueOpsResumeHandler(t.Context(), issueOpsStateRoot(), model.ExecutionResumeRequest{
		ID: record.ID, ExpectedGeneration: 1, Actor: native, CWD: worktree, Confirm: true,
	})
	if resumeErr == nil || !strings.Contains(resumeErr.Error(), "execution resume requires an existing Orca binding") {
		t.Fatalf("native resume err = %v, want the Orca binding decision", resumeErr)
	}
	_, reseedErr := issueOpsReseedHandler(t.Context(), issueOpsStateRoot(), model.ExecutionReseedRequest{
		ID: record.ID, ExpectedGeneration: 1, Actor: native, CWD: worktree, Reason: "parity", Confirm: true,
	})
	if reseedErr == nil {
		t.Fatal("native reseed succeeded on a fixture without its prerequisites")
	}

	url, _ := startProductionHTTPServer(t, issueOpsMCPHTTPDependencies())
	bearer, err := mcpcli.EnsureHTTPBearer(statestore.StateDir())
	if err != nil {
		t.Fatal(err)
	}
	client := connectHTTPClient(t, url, bearer, "")
	for _, tc := range []struct {
		name string
		args map[string]any
		want error
	}{
		{"resume", map[string]any{"action": "resume", "id": record.ID, "cwd": worktree, "expected_generation": 1, "confirm": true, "authority_file": grant}, resumeErr},
		{"reseed", map[string]any{"action": "replace", "replace_action": "reseed", "id": record.ID, "cwd": worktree, "expected_generation": 1, "reason": "parity", "confirm": true, "authority_file": grant}, reseedErr},
	} {
		payload, isError := callHTTPTool(t, client, "issueops_execution", tc.args)
		message, _ := payload["error"].(string)
		if !isError || payload["error_code"] == authoritycontract.CodeInvalid || !strings.Contains(message, tc.want.Error()) {
			t.Errorf("HTTP %s payload=%v isError=%v, want native decision %q", tc.name, payload, isError, tc.want)
		}
	}
}
