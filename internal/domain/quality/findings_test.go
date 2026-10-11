package quality

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	contract "issueops/internal/contract/quality"
)

func findingByID(findings []contract.Finding, id string) (contract.Finding, bool) {
	for _, f := range findings {
		if f.ID == id {
			return f, true
		}
	}
	return contract.Finding{}, false
}

func TestCollectQualityFindingsEmptyInputYieldsNoFindings(t *testing.T) {
	got := CollectQualityFindings(nil, nil, nil, nil, contract.PioneerCoverage{})
	if len(got) != 0 {
		t.Fatalf("findings=%+v", got)
	}
}

func TestCollectQualityFindingsOrderingAndBlocking(t *testing.T) {
	pioneer := contract.PioneerCoverage{
		BenchmarkMissing:    []string{"a"},
		ReproductionMissing: []string{"b"},
		IsolatedExpected:    3,
		IsolatedObserved:    2,
	}
	got := CollectQualityFindings(
		[]string{"cov failed"},
		[]contract.CoveragePackage{{Package: "p", Coverage: 12.34}},
		[]contract.BranchFunction{{File: "f.go", Line: 3, Name: "F", Branches: 13}},
		[]contract.AuditItem{{ID: "A1", Priority: "P1", Title: "t"}},
		pioneer,
	)
	var ids []string
	for _, f := range got {
		ids = append(ids, f.ID)
	}
	want := []string{"quality-collector-error", "pioneer-skill-coverage", "pioneer-isolated-evaluation-incomplete", "low-coverage-packages", "high-branch-functions", "project-audit-items"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids=%v", ids)
	}
	pc, _ := findingByID(got, "pioneer-skill-coverage")
	if !reflect.DeepEqual(pc.Evidence, []string{"benchmark missing: a", "reproduction missing: b"}) || pc.Blocking {
		t.Fatalf("pioneer=%+v", pc)
	}
	if f, _ := findingByID(got, "project-audit-items"); f.Blocking || f.Severity != "p1" {
		t.Fatalf("audit=%+v", f)
	}
	if f, _ := findingByID(got, "low-coverage-packages"); f.Evidence[0] != "p 12.3%" {
		t.Fatalf("coverage=%+v", f)
	}
}

func TestCollectQualityFindingsP0AuditBlocksAndBlockedPioneerIsReportOnly(t *testing.T) {
	got := CollectQualityFindings(nil, nil, nil,
		[]contract.AuditItem{{ID: "A1", Priority: "P1"}, {ID: "A2", Priority: "P0"}},
		contract.PioneerCoverage{IsolatedExpected: 2, IsolatedObserved: 2, IsolatedBlocked: 1})
	audit, _ := findingByID(got, "project-audit-items")
	if !audit.Blocking || audit.Severity != "p0" {
		t.Fatalf("audit=%+v", audit)
	}
	blocked, ok := findingByID(got, "pioneer-isolated-evaluation-blocked")
	if !ok || blocked.Blocking {
		t.Fatalf("blocked=%+v ok=%v", blocked, ok)
	}
	if _, ok := findingByID(got, "pioneer-isolated-evaluation-incomplete"); ok {
		t.Fatal("complete-but-blocked must not be incomplete")
	}
}

func TestCollectQualityFindingsBranchBoundary(t *testing.T) {
	got := CollectQualityFindings(nil, nil, []contract.BranchFunction{{Name: "F", Branches: 12}}, nil, contract.PioneerCoverage{})
	if len(got) != 0 {
		t.Fatalf("12 branches must not be flagged: %+v", got)
	}
}

func TestAddQualityCollectorFailurePrependsFindingWhenAbsent(t *testing.T) {
	result := contract.InspectResult{Findings: []contract.Finding{{ID: "other"}}}
	AddQualityCollectorFailure(&result, "w1")
	if len(result.Findings) != 2 || result.Findings[0].ID != "quality-collector-error" || result.Findings[1].ID != "other" {
		t.Fatalf("findings=%+v", result.Findings)
	}
}

func TestAddQualityCollectorFailureAppendsEvidenceToExistingFinding(t *testing.T) {
	result := contract.InspectResult{
		OK:       true,
		Findings: []contract.Finding{{ID: "other"}, {ID: "quality-collector-error", Evidence: []string{"w0"}}},
	}
	AddQualityCollectorFailure(&result, "w1")
	if len(result.Findings) != 2 || result.Findings[0].ID != "other" {
		t.Fatalf("findings=%+v", result.Findings)
	}
	if !reflect.DeepEqual(result.Findings[1].Evidence, []string{"w0", "w1"}) {
		t.Fatalf("evidence=%v", result.Findings[1].Evidence)
	}
	if !reflect.DeepEqual(result.Warnings, []string{"w1"}) || result.OK ||
		result.CollectionStatus != contract.CollectionStatusError || result.GateStatus != contract.GateStatusBlock {
		t.Fatalf("result=%+v", result)
	}
}

func TestAddSNRRegressionFinding(t *testing.T) {
	result := contract.InspectResult{}
	AddSNRRegressionFinding(&result, 0.5, 0.4)
	if len(result.Findings) != 1 || !result.Findings[0].Blocking ||
		result.Findings[0].Evidence[0] != "baseline=0.5000 current=0.4000 delta=-0.1000" ||
		result.GateStatus != contract.GateStatusBlock || result.HealthStatus != contract.HealthStatusNeedsAttention {
		t.Fatalf("result=%+v", result)
	}
}

func TestSuccessfulSignalValue(t *testing.T) {
	signals := []contract.Signal{{ID: "a", Value: 1, Status: "error"}, {ID: "b", Value: 2, Status: "ok"}}
	if v, ok := SuccessfulSignalValue(signals, "b"); v != 2 || !ok {
		t.Fatalf("b=%v %v", v, ok)
	}
	if v, ok := SuccessfulSignalValue(signals, "a"); v != 1 || ok {
		t.Fatalf("a=%v %v", v, ok)
	}
	if v, ok := SuccessfulSignalValue(signals, "zz"); v != 0 || ok {
		t.Fatalf("zz=%v %v", v, ok)
	}
}

func TestBoundedEvidenceTruncatesWithoutAliasing(t *testing.T) {
	in := []string{"a", "b", "c"}
	if got := BoundedEvidence(in, 3); !reflect.DeepEqual(got, in) {
		t.Fatalf("got=%v", got)
	}
	got := BoundedEvidence(in, 2)
	got[0] = "x"
	if !reflect.DeepEqual(got[1:], []string{"b"}) || in[0] != "a" {
		t.Fatalf("got=%v in=%v", got, in)
	}
}

func TestQualityStatusesBlockingFinding(t *testing.T) {
	c, h, g := QualityStatuses(nil, nil)
	if c != contract.CollectionStatusOK || h != contract.HealthStatusHealthy || g != contract.GateStatusPass {
		t.Fatalf("%s/%s/%s", c, h, g)
	}
	_, h, g = QualityStatuses(nil, []contract.Finding{{}, {Blocking: true}})
	if h != contract.HealthStatusNeedsAttention || g != contract.GateStatusBlock {
		t.Fatalf("%s/%s", h, g)
	}
}

func TestStatusHelpers(t *testing.T) {
	if StatusForCount(0) != "ok" || StatusForCount(1) != "needs_attention" {
		t.Fatal("count status")
	}
	if StatusForCollector(errors.New("x"), "ok") != "error" || StatusForCollector(nil, "fb") != "fb" {
		t.Fatal("collector status")
	}
	cases := []struct {
		c    contract.PioneerCoverage
		want string
	}{
		{contract.PioneerCoverage{}, "unknown"},
		{contract.PioneerCoverage{IsolatedExpected: 2, IsolatedObserved: 1}, "needs_attention"},
		{contract.PioneerCoverage{IsolatedExpected: 2, IsolatedObserved: 2, IsolatedFailed: 1}, "needs_attention"},
		{contract.PioneerCoverage{IsolatedExpected: 2, IsolatedObserved: 2, IsolatedBlocked: 1}, "needs_attention"},
		{contract.PioneerCoverage{IsolatedExpected: 2, IsolatedObserved: 2, IsolatedPassed: 2}, "ok"},
	}
	for i, tc := range cases {
		if got := PioneerIsolatedStatus(tc.c); got != tc.want {
			t.Errorf("case %d: %s want %s", i, got, tc.want)
		}
	}
}

func TestFirstQualityWarning(t *testing.T) {
	if FirstQualityWarning(nil) != nil {
		t.Fatal("nil expected")
	}
	if err := FirstQualityWarning([]string{"first", "second"}); err == nil || err.Error() != "first" {
		t.Fatalf("err=%v", err)
	}
}

func TestPioneerIsolatedEvidenceFormat(t *testing.T) {
	got := PioneerIsolatedEvidence(contract.PioneerCoverage{IsolatedObserved: 1, IsolatedExpected: 2, IsolatedPassed: 3, IsolatedBlocked: 4, IsolatedFailed: 5, HiddenHoldoutObserved: 6})
	if len(got) != 2 || got[0] != "observed=1 expected=2 pass=3 blocked=4 fail=5 hidden=6" {
		t.Fatalf("got=%v", got)
	}
}

func TestEvidenceFallbacksAndFormatting(t *testing.T) {
	if got := CoverageEvidence(nil); !reflect.DeepEqual(got, []string{"go test -cover ./..."}) {
		t.Fatalf("coverage fallback=%v", got)
	}
	if got := CoverageEvidence([]contract.CoveragePackage{{Package: "p", Coverage: 50}}); got[0] != "p 50.0%" {
		t.Fatalf("coverage=%v", got)
	}
	if got := BranchEvidence(nil, 12); !reflect.DeepEqual(got, []string{"Go AST branch scan"}) {
		t.Fatalf("branch fallback=%v", got)
	}
	if got := AuditEvidence(nil); !reflect.DeepEqual(got, []string{".issueops/PROJECT_AUDIT.md"}) {
		t.Fatalf("audit fallback=%v", got)
	}
	if got := AuditEvidence([]contract.AuditItem{{ID: "A1", Priority: "P0", Title: "t"}}); got[0] != "A1 P0 t" {
		t.Fatalf("audit=%v", got)
	}
	snr := SNREvidence(contract.SNRResult{SignalLines: 1, NoiseLines: 2, TotalLines: 3})
	if len(snr) != 2 || snr[0] != "signal=1 noise=2 total=3 lines" {
		t.Fatalf("snr=%v", snr)
	}
}

func TestBranchEvidenceFiltersAndCapsAtTen(t *testing.T) {
	var fns []contract.BranchFunction
	fns = append(fns, contract.BranchFunction{File: "low.go", Name: "Low", Branches: 5})
	for i := range 12 {
		fns = append(fns, contract.BranchFunction{File: "f.go", Line: i, Name: "F", Branches: 20})
	}
	got := BranchEvidence(fns, 12)
	if len(got) != 10 || strings.Contains(got[0], "low.go") || got[0] != "f.go:0 F branches=20" {
		t.Fatalf("got=%v", got)
	}
}

func TestSNRHelpers(t *testing.T) {
	for line, want := range map[string]bool{"": false, "{})(,": true, "}x": false, "foo()": false} {
		if SNRStructuralOnly(line) != want {
			t.Errorf("structural(%q) want %v", line, want)
		}
	}
	for _, tc := range []struct {
		r    float64
		want bool
	}{{0, true}, {1, true}, {0.5, true}, {-0.1, false}, {1.1, false}, {math.NaN(), false}, {math.Inf(1), false}} {
		if ValidSNRRatio(tc.r) != tc.want {
			t.Errorf("ValidSNRRatio(%v) want %v", tc.r, tc.want)
		}
	}
	if SNRRegressed(0.5, 0.49) || !SNRRegressed(0.5, 0.48) {
		t.Fatal("regression tolerance is 0.01")
	}
}

func TestSNRBaselineKeyAndValidation(t *testing.T) {
	a, b := SNRBaselineKey("repo-a"), SNRBaselineKey("repo-b")
	if a == b || a != SNRBaselineKey("repo-a") || !strings.HasPrefix(a, "quality-snr-baseline-") || len(a) != len("quality-snr-baseline-")+16 {
		t.Fatalf("keys=%s %s", a, b)
	}
	rec := contract.SNRBaselineRecord{SchemaVersion: contract.SNRBaselineSchemaVersion, Repository: "r", Ratio: 0.5}
	if !ValidSNRBaseline(rec, "r") || ValidSNRBaseline(rec, "other") {
		t.Fatal("repository match")
	}
	bad := rec
	bad.SchemaVersion++
	if ValidSNRBaseline(bad, "r") {
		t.Fatal("schema mismatch")
	}
	bad = rec
	bad.Ratio = 2
	if ValidSNRBaseline(bad, "r") {
		t.Fatal("invalid ratio")
	}
}
