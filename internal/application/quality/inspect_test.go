package quality

import (
	"errors"
	"testing"

	catalog "issueops/internal/contract/qualitycatalog"
)

func TestInspectKeepsPartialCollectorFailures(t *testing.T) {
	result := Inspect("/repo", InspectDeps{
		Now:                  func() string { return "now" },
		Coverage:             func(string) (string, error) { return "", errors.New("unavailable") },
		BranchFunctions:      func(string) ([]BranchFunction, []string) { return nil, nil },
		AuditItems:           func(string) ([]AuditItem, []string) { return nil, nil },
		SelfAugmentOpenCount: func(string) (int, error) { return 0, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 0, nil },
		Candidates:           func(string) []catalog.Candidate { return nil },
		CodeSNR:              func(string) (SNRResult, error) { return SNRResult{}, nil },
		PioneerCoverage:      func(string) (PioneerCoverage, error) { return PioneerCoverage{}, nil },
	})
	if result.OK || result.CollectionStatus != "error" || result.HealthStatus != "unknown" || result.GateStatus != "block" || len(result.Warnings) != 1 || len(result.Signals) == 0 {
		t.Fatalf("result=%+v", result)
	}
}
