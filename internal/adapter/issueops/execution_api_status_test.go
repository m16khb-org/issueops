package issueops

import (
	"context"
	model "issueops/internal/contract/issueops"
	"testing"
)

func TestExecutionStatusUsesConfiguredHandler(t *testing.T) {
	calls := 0
	out, err := ExecuteExecution(t.Context(), "isolated-state", ExecutionActionRequest{Action: ExecutionActionStatus, ID: "io-status"}, ExecutionActionDependencies{Status: func(_ context.Context, root, id string) (model.ExecutionResult, error) {
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
