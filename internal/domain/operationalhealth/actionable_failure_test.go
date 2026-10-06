package operationalhealth

import (
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	"testing"
	"time"
)

func TestClassifySurfacesDurableIssueOpsFailures(t *testing.T) {
	snapshot := healthyDirectSnapshot()
	snapshot.Cycles[0].ExecutionFailurePresent = true
	snapshot.Cycles[0].CleanupFailurePresent = true
	snapshot.Cycles[0].IssueCreateFailurePresent = true

	result := Classify(snapshot, Options{Now: time.Now()})

	if !hasFinding(result, operationalhealthcontract.FindingExecutionFailure, "cycle") {
		t.Fatalf("execution failure was not surfaced: %+v", result.Findings)
	}
	if !hasFinding(result, operationalhealthcontract.FindingCleanupFailure, "cycle") {
		t.Fatalf("cleanup failure was not surfaced: %+v", result.Findings)
	}
	if !hasFinding(result, operationalhealthcontract.FindingIssueCreateFailure, "cycle") {
		t.Fatalf("issue create failure was not surfaced: %+v", result.Findings)
	}
}
