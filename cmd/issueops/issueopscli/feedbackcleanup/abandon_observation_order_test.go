package feedbackcleanup

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"
	"testing"
)

func TestAbandonOwnershipRefusalPrecedesArtifactObservation(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := cleanupStatusRecord(t, false, true)
	_, _, command := wireAbandonCapture(t)
	observed := 0
	entered := false
	configured := command.Operations
	configured.CleanupAbandon = func(_ context.Context, _ string, req issueopscontract.CleanupAbandonRequest, _ Deps) (issueopscontract.CleanupAbandonResult, error) {
		entered = true
		return issueopscontract.CleanupAbandonResult{OK: false, ID: req.ID}, context.Canceled
	}
	command.Operations = configured
	deps := Deps{ParseFlags: parseFeedbackCleanupFlags, PrintJSON: func(any) error { return nil }, PrintError: func(error) error { return nil }, ObserveArtifactMerged: func(issueopscontract.IssueOpsRemoteArtifactVerification) (bool, error) { observed++; return false, nil }}
	err := command.RunCleanup([]string{"abandon", "--id", record.ID, "--reason", "ownership-test", "--preview", "--json"}, deps)
	if err == nil || !entered {
		t.Fatalf("executor refusal not reached: %v", err)
	}
	if observed != 0 {
		t.Fatalf("CLI observed artifact before executor ownership refusal: %d", observed)
	}
}
