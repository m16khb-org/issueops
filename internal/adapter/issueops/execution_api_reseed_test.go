package issueops

import (
	issueopscontract "issueops/internal/contract/issueops"
	issueopsport "issueops/internal/port"
)

import (
	"context"
	"errors"
	ownerdomain "issueops/internal/domain/issueops"
	"strings"
	"testing"
)

func TestExecuteExecutionReseedRequiresInjectedHandler(t *testing.T) {
	_, err := testExecutionService().Execute(context.Background(), t.TempDir(), issueopscontract.ExecutionActionRequest{
		Action:        issueopscontract.ExecutionActionReplace,
		ReplaceAction: issueopscontract.ExecutionReplaceReseed,
		ID:            "io-reseed-handler",
	}, issueopsport.ExecutionActionDependencies{})
	if !errors.Is(err, issueopscontract.ErrReseedHandlerUnavailable) {
		t.Fatalf("reseed error=%v, want unavailable handler", err)
	}
}

func TestExecuteExecutionReseedUsesInjectedHandlerOnce(t *testing.T) {
	called := 0
	result, err := testExecutionService().Execute(context.Background(), t.TempDir(), issueopscontract.ExecutionActionRequest{
		Action:               issueopscontract.ExecutionActionReplace,
		ReplaceAction:        issueopscontract.ExecutionReplaceReseed,
		ID:                   "io-reseed-handler",
		ExpectedGeneration:   3,
		CompletionGeneration: 2,
		InventoryFingerprint: "inventory",
		Reason:               "holderless recovery",
		CWD:                  "/canonical/worktree",
		Confirm:              true,
	}, issueopsport.ExecutionActionDependencies{Reseed: func(_ context.Context, stateRoot string, request issueopscontract.ExecutionReseedRequest) (issueopscontract.ExecutionReplaceResult, error) {
		called++
		if stateRoot == "" || request.ID != "io-reseed-handler" || request.ExpectedGeneration != 3 || request.CompletionGeneration != 2 || request.InventoryFingerprint != "inventory" || request.Reason != "holderless recovery" || request.CWD != "/canonical/worktree" || !request.Confirm {
			t.Fatalf("unexpected injected reseed request: root=%q request=%+v", stateRoot, request)
		}
		return issueopscontract.ExecutionReplaceResult{OK: true, ID: request.ID, Action: issueopscontract.ExecutionReplaceReseed}, nil
	}})
	if err != nil {
		t.Fatalf("execute reseed: %v", err)
	}
	got, ok := result.(issueopscontract.ExecutionReplaceResult)
	if !ok || !got.OK || got.ID != "io-reseed-handler" || called != 1 {
		t.Fatalf("result=%#v called=%d", result, called)
	}
}

func TestExecutionReseedNextCommandRendersModeSpecificRecovery(t *testing.T) {
	direct := ownerdomain.OwnerReseedNextCommand("io-direct", 2, "direct", "/tmp/lease-2.token")
	for _, want := range []string{"execution claim", "--generation 2", "--claim-current-token"} {
		if !strings.Contains(direct, want) {
			t.Fatalf("direct reseed next command %q does not contain %q", direct, want)
		}
	}
	if strings.Contains(direct, "/tmp/lease-2.token") {
		t.Fatalf("direct reseed next command exposes token path: %q", direct)
	}
	orca := ownerdomain.OwnerReseedNextCommand("io-orca", 3, "orca", "/tmp/ignored.token")
	if !strings.Contains(orca, "execution resume") || strings.Contains(orca, "/tmp/ignored.token") {
		t.Fatalf("Orca reseed next command = %q", orca)
	}
	if got := ownerdomain.OwnerReseedNextCommand("io-unknown", 1, "unknown", "/tmp/token"); got != "" {
		t.Fatalf("unknown mode next command = %q, want empty", got)
	}
}
