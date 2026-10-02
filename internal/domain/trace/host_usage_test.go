package trace

import (
	"regexp"
	"slices"
	"testing"

	tracecontract "issueops/internal/contract/trace"
)

func ptr(v int64) *int64 { return &v }

func cumulative(session, model string, input, output int64) UsageObservation {
	return UsageObservation{
		Host: "claude", Model: model, SessionID: session,
		Finality: tracecontract.UsageFinalityFinal, Temporality: tracecontract.UsageTemporalityCumulative,
		Input: ptr(input), Output: ptr(output), CostBasis: tracecontract.UsageCostBasisUnknown,
	}
}

func delta(session, turn string, input int64) UsageObservation {
	return UsageObservation{
		Host: "codex", SessionID: session, TurnID: turn,
		Finality: tracecontract.UsageFinalityFinal, Temporality: tracecontract.UsageTemporalityDelta,
		Input: ptr(input), CostBasis: tracecontract.UsageCostBasisUnknown,
	}
}

func warned(report *tracecontract.UsageReport, code string) bool {
	return slices.Contains(report.Warnings, code)
}

func TestReduceUsageCumulativeKeepsLastSnapshotPerEpoch(t *testing.T) {
	report := ReduceUsage([]UsageObservation{
		cumulative("s", "m", 100, 10),
		cumulative("s", "m", 250, 40),
		cumulative("s", "other", 7, 1),
		cumulative("s", "m", 20, 2),
		cumulative("s", "m", 30, 3),
	}, nil)
	if len(report.Samples) != 3 {
		t.Fatalf("samples=%d %+v", len(report.Samples), report)
	}
	first, other, reset := report.Samples[0], report.Samples[1], report.Samples[2]
	if *first.InputTokens != 250 || first.Epoch != "0" || *other.InputTokens != 7 || other.Epoch != "0" || *reset.InputTokens != 30 || reset.Epoch != "1" {
		t.Errorf("samples = %+v", report.Samples)
	}
	if report.Coverage != tracecontract.UsageCoveragePartial || !warned(report, "usage_cumulative_reset") {
		t.Errorf("coverage=%q warnings=%q", report.Coverage, report.Warnings)
	}
}

func TestReduceUsageCostDecreaseIsAlsoAReset(t *testing.T) {
	cost := func(value float64) *float64 { return &value }
	a, b := cumulative("s", "m", 10, 1), cumulative("s", "m", 20, 2)
	a.CostUSD, b.CostUSD = cost(0.9), cost(0.1)
	report := ReduceUsage([]UsageObservation{a, b}, nil)
	if len(report.Samples) != 2 || !warned(report, "usage_cumulative_reset") {
		t.Errorf("report = %+v", report)
	}
}

func TestReduceUsageDeltaDeduplicatesOnlyWithIdentity(t *testing.T) {
	report := ReduceUsage([]UsageObservation{delta("s", "t1", 5), delta("s", "t1", 5), delta("s", "t2", 5)}, nil)
	if len(report.Samples) != 2 || report.Coverage != tracecontract.UsageCoverageComplete || !warned(report, "usage_duplicate_event") {
		t.Errorf("identified duplicates = %+v", report)
	}
	conflict := ReduceUsage([]UsageObservation{delta("s", "t1", 5), delta("s", "t1", 6)}, nil)
	if len(conflict.Samples) != 1 || *conflict.Samples[0].InputTokens != 5 || conflict.Coverage != tracecontract.UsageCoveragePartial || !warned(conflict, "usage_conflicting_duplicate") {
		t.Errorf("conflict = %+v", conflict)
	}
	anonymous := ReduceUsage([]UsageObservation{delta("s", "", 5), delta("s", "", 5)}, nil)
	if len(anonymous.Samples) != 2 || anonymous.Coverage != tracecontract.UsageCoverageUnknown {
		t.Errorf("anonymous = %+v", anonymous)
	}
	sessionless := ReduceUsage([]UsageObservation{cumulative("", "m", 5, 1), cumulative("", "m", 5, 1)}, nil)
	if len(sessionless.Samples) != 2 || sessionless.Coverage != tracecontract.UsageCoverageUnknown {
		t.Errorf("sessionless cumulative = %+v", sessionless)
	}
}

func TestReduceUsageCoverageRules(t *testing.T) {
	empty := ReduceUsage(nil, nil)
	if empty.Coverage != tracecontract.UsageCoverageUnknown || empty.Samples == nil || empty.Warnings == nil {
		t.Errorf("empty = %+v", empty)
	}
	nonFinal := delta("s", "t1", 5)
	nonFinal.Finality = tracecontract.UsageFinalityPartial
	if report := ReduceUsage([]UsageObservation{nonFinal}, nil); report.Coverage != tracecontract.UsageCoveragePartial {
		t.Errorf("non-final coverage = %q", report.Coverage)
	}
	if report := ReduceUsage([]UsageObservation{delta("s", "t1", 5)}, []string{"usage_missing", "usage_missing", "invalid_jsonl_line"}); report.Coverage != tracecontract.UsageCoveragePartial || len(report.Warnings) != 2 || report.Warnings[0] != "invalid_jsonl_line" {
		t.Errorf("loss warnings must be unique, sorted and downgrade coverage: %+v", report)
	}
	complete := ReduceUsage([]UsageObservation{delta("s", "t1", 5)}, nil)
	NoteUsageLoss(complete, "input_truncated")
	if complete.Coverage != tracecontract.UsageCoveragePartial || !warned(complete, "input_truncated") {
		t.Errorf("after loss = %+v", complete)
	}
	unknown := ReduceUsage(nil, nil)
	NoteUsageLoss(unknown, "input_truncated")
	if unknown.Coverage != tracecontract.UsageCoverageUnknown {
		t.Errorf("loss must never upgrade coverage: %q", unknown.Coverage)
	}
}

func TestUsageDigestIsDeterministicAndDomainSeparated(t *testing.T) {
	hexDigest := regexp.MustCompile(`^[0-9a-f]{64}$`)
	first, again := UsageDigest("session", "abc"), UsageDigest("session", "abc")
	if !hexDigest.MatchString(first) || first != again {
		t.Errorf("digest must be stable lowercase hex")
	}
	if UsageDigest("session", "abc") == UsageDigest("turn", "abc") || UsageDigest("session", "abc") == UsageDigest("session", "abd") {
		t.Errorf("digests must be separated by kind and id")
	}
	if UsageDigest("session", "") != "" {
		t.Errorf("an empty identifier must stay empty")
	}
}
