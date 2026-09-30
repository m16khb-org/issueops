package issueops

import (
	"context"
	"strings"
	"testing"
)

func TestAbandonCancelledBranchObservationIsNotAbsence(t *testing.T) {
	root, record := abandonTestRecord(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	before, err := (CleanupRecordStore{StateRoot: root}).Load(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	executor := abandonExecutorForTests(root, CleanupAbandonDeps{
		Git: func(_ string, args ...string) (int, string) {
			if len(args) > 0 && args[0] == "rev-parse" {
				cancel()
				return 1, "context canceled"
			}
			t.Fatalf("unexpected command: %v", args)
			return 1, ""
		},
	})
	result, err := executor.Run(ctx, abandonRequest(record.ID, false, ""))
	if err == nil || result.Fingerprint != "" || !strings.Contains(strings.Join(result.Missing, ","), "local_branch_observable") {
		t.Fatalf("cancelled observation approved: %+v %v", result, err)
	}
	after, err := (CleanupRecordStore{StateRoot: root}).Load(context.Background(), record.ID)
	if err != nil || before.Revision != after.Revision {
		t.Fatalf("cancelled preview changed record: %v", err)
	}
}
