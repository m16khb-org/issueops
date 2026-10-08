package issueops

import (
	issueopsport "issueops/internal/port"
)

import (
	"context"
	model "issueops/internal/contract/issueops"
	"testing"
)

func TestExecutionStatusUsesConfiguredHandler(t *testing.T) {
	t.Parallel()

	calls := 0
	out, err := testExecutionService().Execute(t.Context(), "isolated-state", model.ExecutionActionRequest{Action: model.ExecutionActionStatus, ID: "io-status"}, issueopsport.ExecutionActionDependencies{Status: func(_ context.Context, root, id string) (model.ExecutionResult, error) {
		calls++
		if root != "isolated-state" || id != "io-status" {
			t.Fatalf("wrong identity: %q %q", root, id)
		}
		return model.ExecutionResult{OK: true, ID: id, NextCommand: "configured"}, nil
	}})
	if err != nil || calls != 1 || out.(model.ExecutionResult).NextCommand != "configured" {
		t.Fatalf("status bypassed application handler: %v %+v calls=%d", err, out, calls)
	}
}
