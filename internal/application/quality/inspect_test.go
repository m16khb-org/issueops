package quality

import (
	"errors"
	contract "issueops/internal/contract/quality"
	"testing"

	catalog "issueops/internal/contract/qualitycatalog"
)

func TestInspectKeepsPartialCollectorFailures(t *testing.T) {
	result := Inspect("/repo", InspectDeps{
		Now:                  func() string { return "now" },
		Coverage:             func(string) (string, error) { return "", errors.New("unavailable") },
		BranchFunctions:      func(string) ([]contract.BranchFunction, []string) { return nil, nil },
		AuditItems:           func(string) ([]contract.AuditItem, []string) { return nil, nil },
		SelfAugmentOpenCount: func(string) (int, error) { return 0, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 0, nil },
		Candidates:           func(string) []catalog.Candidate { return nil },
		CodeSNR:              func(string) (contract.SNRResult, error) { return contract.SNRResult{}, nil },
		PioneerCoverage:      func(string) (contract.PioneerCoverage, error) { return contract.PioneerCoverage{}, nil },
	})
	if result.OK || result.CollectionStatus != "error" || result.HealthStatus != "unknown" || result.GateStatus != "block" || len(result.Warnings) != 1 || len(result.Signals) == 0 {
		t.Fatalf("result=%+v", result)
	}
}
