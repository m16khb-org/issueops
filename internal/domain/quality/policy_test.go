package quality

import (
	contract "issueops/internal/contract/quality"
	"testing"
)

func TestCoverageThresholdAndGateStatus(t *testing.T) {
	packages := ParseCoveragePackages("ok  issueops/a 0.1s coverage: 59.9% of statements\nok  issueops/b 0.1s coverage: 60.0% of statements\n", 60)
	if len(packages) != 1 || packages[0].Package != "issueops/a" {
		t.Fatalf("packages=%+v", packages)
	}
	collection, health, gate := QualityStatuses(nil, []contract.Finding{{Blocking: false}})
	if collection != contract.CollectionStatusOK || health != contract.HealthStatusNeedsAttention || gate != contract.GateStatusReportOnly {
		t.Fatalf("statuses=%s/%s/%s", collection, health, gate)
	}
	collection, health, gate = QualityStatuses([]string{"collector failed"}, nil)
	if collection != contract.CollectionStatusError || health != contract.HealthStatusUnknown || gate != contract.GateStatusBlock {
		t.Fatalf("collector statuses=%s/%s/%s", collection, health, gate)
	}
}

func TestCollectorFailureDoesNotClaimHealth(t *testing.T) {
	result := contract.InspectResult{OK: true, CollectionStatus: contract.CollectionStatusOK, HealthStatus: contract.HealthStatusHealthy, GateStatus: contract.GateStatusPass}
	AddQualityCollectorFailure(&result, "coverage: unavailable")
	if result.OK || result.HealthStatus != contract.HealthStatusUnknown || result.GateStatus != contract.GateStatusBlock || len(result.Findings) != 1 {
		t.Fatalf("result=%+v", result)
	}
	AddQualityCollectorFailure(&result, "audit: unavailable")
	if len(result.Findings) != 1 || len(result.Findings[0].Evidence) != 2 {
		t.Fatalf("duplicate collector finding: %+v", result.Findings)
	}
}

func TestFailedTestPackagesListsFailLinesWithinLimit(t *testing.T) {
	output := "ok  \tissueops/a\t0.1s\tcoverage: 70.0% of statements\n" +
		"--- FAIL: TestX (0.04s)\nFAIL\ncoverage: 87.5% of statements\n" +
		"FAIL\tissueops/b\t5.812s\n" +
		"FAIL\tissueops/c [build failed]\n" +
		"FAIL\tissueops/b\t1.0s\n" +
		"FAIL\tissueops/d [setup failed]\n" +
		"FAIL\n"
	packages, omitted := FailedTestPackages(output, 2)
	if len(packages) != 2 || packages[0] != "issueops/b" || packages[1] != "issueops/c" || omitted != 1 {
		t.Fatalf("packages=%v omitted=%d", packages, omitted)
	}
	if packages, omitted := FailedTestPackages("ok  \tissueops/a\t0.1s\n", 2); len(packages) != 0 || omitted != 0 {
		t.Fatalf("passing output: packages=%v omitted=%d", packages, omitted)
	}
}
