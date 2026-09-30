package issueops

import (
	issueopscontract "issueops/internal/contract/issueops"
	issueopsport "issueops/internal/port"
)

import (
	"context"
	"errors"
	"testing"
)

func TestExecuteExecutionReleaseRequiresInjectedHandler(t *testing.T) {
	_, err := testExecutionService().Execute(context.Background(), t.TempDir(), issueopscontract.ExecutionActionRequest{
		Action: issueopscontract.ExecutionActionRelease,
		ID:     "io-release-handler",
	}, issueopsport.ExecutionActionDependencies{})
	if !errors.Is(err, issueopscontract.ErrReleaseHandlerUnavailable) {
		t.Fatalf("release error=%v, want unavailable handler", err)
	}
}

func TestExecuteExecutionReleaseUsesInjectedHandlerOnce(t *testing.T) {
	called := 0
	result, err := testExecutionService().Execute(context.Background(), t.TempDir(), issueopscontract.ExecutionActionRequest{
		Action:     issueopscontract.ExecutionActionRelease,
		ID:         "io-release-handler",
		Generation: 3,
		CWD:        "/canonical/worktree",
	}, issueopsport.ExecutionActionDependencies{Release: func(_ context.Context, stateRoot string, request issueopscontract.ExecutionReleaseRequest) (issueopscontract.ExecutionResult, error) {
		called++
		if stateRoot == "" || request.ID != "io-release-handler" || request.Generation != 3 || request.CWD != "/canonical/worktree" {
			t.Fatalf("unexpected injected release request: root=%q request=%+v", stateRoot, request)
		}
		return issueopscontract.ExecutionResult{OK: true, ID: request.ID}, nil
	}})
	if err != nil {
		t.Fatalf("execute release: %v", err)
	}
	got, ok := result.(issueopscontract.ExecutionResult)
	if !ok || !got.OK || got.ID != "io-release-handler" || called != 1 {
		t.Fatalf("result=%#v called=%d", result, called)
	}
}
