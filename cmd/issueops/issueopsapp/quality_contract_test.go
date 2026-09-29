package issueopsapp

import (
	qualityapp "issueops/internal/application/quality"
	qualitycontract "issueops/internal/contract/quality"
	"testing"
)

func qualitycliInspectDepsForContract() qualityapp.InspectDeps {
	return qualityapp.InspectDeps{
		Now: func() string { return "2000-01-01T00:00:00Z" },
		Coverage: func(string) (string, error) {
			return "ok  \tissueops/internal/core/commandguard\t0.011s\tcoverage: 54.3% of statements\n", nil
		},
		SelfAugmentOpenCount: func(string) (int, error) { return 10, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 0, nil },
		PioneerCoverage: func(string) (qualitycontract.PioneerCoverage, error) {
			return qualitycontract.PioneerCoverage{
				Expected:             12,
				BenchmarkObserved:    12,
				ReproductionObserved: 12,
			}, nil
		},
		CodeSNR: func(string) (qualitycontract.SNRResult, error) {
			return qualitycontract.SNRResult{SignalLines: 70, NoiseLines: 30, TotalLines: 100, Ratio: 0.7}, nil
		},
	}
}

func TestQualityContractFixtureSeparatesHealthFromCollection(t *testing.T) {
	result := inspectQualityForTest(t.TempDir(), qualitycliInspectDepsForContract())

	if !result.OK || result.CollectionStatus != qualitycontract.CollectionStatusOK {
		t.Fatalf("collection status = ok=%v status=%q warnings=%v", result.OK, result.CollectionStatus, result.Warnings)
	}
	if result.HealthStatus != qualitycontract.HealthStatusNeedsAttention || result.GateStatus != qualitycontract.GateStatusReportOnly {
		t.Fatalf("quality statuses = health=%q gate=%q", result.HealthStatus, result.GateStatus)
	}
	if len(result.Findings) != 1 || result.Findings[0].ID != "low-coverage-packages" {
		t.Fatalf("findings = %+v", result.Findings)
	}
}
