package quality

import "testing"

func TestCoverageThresholdAndGateStatus(t *testing.T) {
	packages := ParseCoveragePackages("ok  issueops/a 0.1s coverage: 59.9% of statements\nok  issueops/b 0.1s coverage: 60.0% of statements\n", 60)
	if len(packages) != 1 || packages[0].Package != "issueops/a" {
		t.Fatalf("packages=%+v", packages)
	}
	collection, health, gate := QualityStatuses(nil, []Finding{{Blocking: false}})
	if collection != CollectionStatusOK || health != HealthStatusNeedsAttention || gate != GateStatusReportOnly {
		t.Fatalf("statuses=%s/%s/%s", collection, health, gate)
	}
	collection, health, gate = QualityStatuses([]string{"collector failed"}, nil)
	if collection != CollectionStatusError || health != HealthStatusUnknown || gate != GateStatusBlock {
		t.Fatalf("collector statuses=%s/%s/%s", collection, health, gate)
	}
}

func TestCollectorFailureDoesNotClaimHealth(t *testing.T) {
	result := InspectResult{OK: true, CollectionStatus: CollectionStatusOK, HealthStatus: HealthStatusHealthy, GateStatus: GateStatusPass}
	AddQualityCollectorFailure(&result, "coverage: unavailable")
	if result.OK || result.HealthStatus != HealthStatusUnknown || result.GateStatus != GateStatusBlock || len(result.Findings) != 1 {
		t.Fatalf("result=%+v", result)
	}
	AddQualityCollectorFailure(&result, "audit: unavailable")
	if len(result.Findings) != 1 || len(result.Findings[0].Evidence) != 2 {
		t.Fatalf("duplicate collector finding: %+v", result.Findings)
	}
}
