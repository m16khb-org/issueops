package quality

import (
	contract "issueops/internal/contract/quality"
	"reflect"
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

func TestPioneerSkillFindingListsEveryMissingEntryForUnevenLists(t *testing.T) {
	pioneer := contract.PioneerCoverage{ReproductionMissing: []string{"r1", "r2"}, BenchmarkMissing: []string{"b1"}}
	finding, ok := findingByID(CollectQualityFindings(nil, nil, nil, nil, pioneer), "pioneer-skill-coverage")
	if !ok {
		t.Fatal("missing pioneer-skill-coverage finding")
	}
	want := []string{"benchmark missing: b1", "reproduction missing: r1", "reproduction missing: r2"}
	if !reflect.DeepEqual(finding.Evidence, want) {
		t.Fatalf("evidence=%v want %v", finding.Evidence, want)
	}
}

func TestPioneerIsolatedFindingRequiresExpectedEvidence(t *testing.T) {
	pioneer := contract.PioneerCoverage{IsolatedExpected: 0, IsolatedObserved: 1}
	if _, ok := findingByID(CollectQualityFindings(nil, nil, nil, nil, pioneer), "pioneer-isolated-evaluation-incomplete"); ok {
		t.Fatal("no isolated evaluation expected, but incomplete finding was raised")
	}
}

func TestParseCoveragePackagesClassifiesLineShapes(t *testing.T) {
	output := "?   \tissueops/q\tcoverage: 10.0% of statements\n" +
		"issueops/bare coverage: 20.0% of statements\n" +
		"ok  \tissueops/ok\t0.1s\tcoverage: 30.0% of statements\n" +
		"issueops/noword xcoverage: 5.0% of statements\n" +
		"issueops/dangling 5.0% of statements coverage:\n"
	got := ParseCoveragePackages(output, 60)
	want := []contract.CoveragePackage{
		{Package: "issueops/q", Coverage: 10},
		{Package: "issueops/bare", Coverage: 20},
		{Package: "issueops/ok", Coverage: 30},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packages=%+v want %+v", got, want)
	}
}

func TestFailedTestPackagesAcceptsTwoFieldFailLine(t *testing.T) {
	packages, omitted := FailedTestPackages("FAIL issueops/x\n", 5)
	if len(packages) != 1 || packages[0] != "issueops/x" || omitted != 0 {
		t.Fatalf("packages=%v omitted=%d", packages, omitted)
	}
}

func TestBranchEvidenceExcludesFunctionsAtThreshold(t *testing.T) {
	functions := []contract.BranchFunction{
		{File: "a.go", Line: 1, Name: "atLimit", Branches: 12},
		{File: "b.go", Line: 2, Name: "over", Branches: 13},
	}
	got := BranchEvidence(functions, 12)
	if want := []string{"b.go:2 over branches=13"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("evidence=%v want %v", got, want)
	}
}
