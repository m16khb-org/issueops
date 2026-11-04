package operationalhealth

import (
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestCycleFromRecordProjectsDurableFailures(t *testing.T) {
	record := issueopscontract.Record{
		ID:    "io-failed",
		Phase: issueopscontract.PhaseImplement,
		Execution: &issueopscontract.Execution{
			Failure: &issueopscontract.ExecutionFailure{Code: "external_operation_ambiguous"},
		},
		CleanupFinishFailure: &issueopscontract.CleanupFinishFailure{
			Step: issueopscontract.CleanupFailureStepWorktreeRemove,
		},
		IssueCreateIntent: &issueopscontract.IssueCreateIntent{
			Status: issueopscontract.IssueCreateIntentInvokedUnknown,
		},
	}

	cycle, _ := cycleFromRecord(record, nil)

	if !cycle.ExecutionFailurePresent || !cycle.CleanupFailurePresent || !cycle.IssueCreateFailurePresent {
		t.Fatalf("failure projection = %+v", cycle)
	}
}

func TestCycleFromRecordDoesNotFlagNormalPendingIssueCreate(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	record := issueopscontract.Record{
		ID:    "io-pending",
		Phase: issueopscontract.PhaseProblem,
		IssueCreateIntent: &issueopscontract.IssueCreateIntent{
			Status:    issueopscontract.IssueCreateIntentPending,
			StartedAt: now.Add(-time.Minute).Format(time.RFC3339Nano),
			UpdatedAt: now.Add(-time.Minute).Format(time.RFC3339Nano),
		},
	}

	cycle, _ := cycleFromRecordAt(record, nil, now)

	if cycle.IssueCreateFailurePresent {
		t.Fatalf("normal in-flight intent was marked unhealthy: %+v", cycle)
	}
}

func TestCycleFromRecordFlagsStalePendingIssueCreate(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	record := issueopscontract.Record{
		ID:    "io-stale-pending",
		Phase: issueopscontract.PhaseProblem,
		IssueCreateIntent: &issueopscontract.IssueCreateIntent{
			Status:    issueopscontract.IssueCreateIntentPending,
			StartedAt: now.Add(-10 * time.Minute).Format(time.RFC3339Nano),
			UpdatedAt: now.Add(-10 * time.Minute).Format(time.RFC3339Nano),
		},
	}

	cycle, _ := cycleFromRecordAt(record, nil, now)

	if !cycle.IssueCreateFailurePresent {
		t.Fatalf("stale pending intent was not marked unhealthy: %+v", cycle)
	}
}
