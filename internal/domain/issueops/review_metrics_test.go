package issueops

import (
	"reflect"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

// 리뷰 지표는 record가 이미 들고 있는 원자료(devil's-advocate history, regress
// 이벤트, phase ledger)에서 파생된다. 새 상태를 만들지 않고 읽기만 한다.
func TestReviewMetricsForRecordCountsRoundsVerdictsAndDurations(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		ID:    "io-metrics",
		Phase: issueopscontract.IssueOpsPhaseImplement,
		DevilsAdvocateReview: &issueopscontract.IssueOpsDevilsAdvocateReview{
			Verdict: "pass", RecordedAt: "2026-09-08T00:10:00Z",
			History: []issueopscontract.IssueOpsDevilsAdvocateRound{
				{Verdict: "revise", RecordedAt: "2026-09-08T00:00:00Z"},
				{Verdict: "revise", RecordedAt: "2026-09-08T00:04:00Z"},
			},
		},
		RegressEvents:        []issueopscontract.IssueOpsRegressEvent{{Reason: "stop", FromPhase: issueopscontract.IssueOpsPhasePlan, At: "2026-09-08T00:05:00Z"}},
		ImplementationReview: &issueopscontract.IssueOpsImplementationReview{Verdict: "pass"},
		PhaseLedger: issueopscontract.IssueOpsPhaseLedger{
			issueopscontract.IssueOpsPhasePlan: {
				Phase:       issueopscontract.IssueOpsPhasePlan,
				EnteredAt:   "2026-09-08T00:00:00Z",
				CompletedAt: "2026-09-08T00:01:00Z",
			},
			// 완료되지 않은 phase는 소요를 만들지 않는다.
			issueopscontract.IssueOpsPhaseImplement: {
				Phase:     issueopscontract.IssueOpsPhaseImplement,
				EnteredAt: "2026-09-08T00:01:00Z",
			},
		},
	}

	got, warnings := ReviewMetricsForRecord(record)
	if len(warnings) != 0 {
		t.Fatalf("parseable record must not warn: %v", warnings)
	}
	if got.ID != "io-metrics" || got.Phase != string(issueopscontract.IssueOpsPhaseImplement) {
		t.Fatalf("identity must round-trip: %+v", got)
	}
	if got.DevilsAdvocateRounds != 3 {
		t.Fatalf("rounds = %d, want 3 (history 2 + current)", got.DevilsAdvocateRounds)
	}
	if want := []string{"revise", "revise", "pass"}; !reflect.DeepEqual(got.DevilsAdvocateVerdicts, want) {
		t.Fatalf("verdicts = %v, want %v (oldest first)", got.DevilsAdvocateVerdicts, want)
	}
	if got.RegressCount != 1 || got.ImplementationReviewVerdict != "pass" {
		t.Fatalf("regress/implementation verdict wrong: %+v", got)
	}
	if got.StageDurationsSeconds["plan"] != 60 {
		t.Fatalf("plan duration = %v, want 60", got.StageDurationsSeconds["plan"])
	}
	if _, ok := got.StageDurationsSeconds["implement"]; ok {
		t.Fatalf("phase without completed_at must not produce a duration: %+v", got.StageDurationsSeconds)
	}
	if want := []float64{240, 360}; !reflect.DeepEqual(got.ReviewRoundGapsSeconds, want) {
		t.Fatalf("round gaps = %v, want %v", got.ReviewRoundGapsSeconds, want)
	}
}

func TestReviewMetricsForRecordWithoutReviewReportsZeroRounds(t *testing.T) {
	got, warnings := ReviewMetricsForRecord(issueopscontract.IssueOpsRecord{ID: "io-bare", Phase: issueopscontract.IssueOpsPhasePlan})
	if len(warnings) != 0 {
		t.Fatalf("bare record must not warn: %v", warnings)
	}
	if got.DevilsAdvocateRounds != 0 || len(got.DevilsAdvocateVerdicts) != 0 {
		t.Fatalf("no review means no rounds, got %+v", got)
	}
	if len(got.StageDurationsSeconds) != 0 || len(got.ReviewRoundGapsSeconds) != 0 {
		t.Fatalf("no ledger means no durations, got %+v", got)
	}
}

// 파싱할 수 없는 시각은 해당 항목만 건너뛰고 warning으로 남긴다. 지표 하나가
// 깨졌다고 전체 사이클을 버리면 측정 자체가 불가능해진다.
func TestReviewMetricsForRecordWarnsOnUnparsableTimestamps(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		ID: "io-bad-time",
		DevilsAdvocateReview: &issueopscontract.IssueOpsDevilsAdvocateReview{
			Verdict: "pass", RecordedAt: "not-a-time",
			History: []issueopscontract.IssueOpsDevilsAdvocateRound{{Verdict: "revise", RecordedAt: "2026-09-08T00:00:00Z"}},
		},
		PhaseLedger: issueopscontract.IssueOpsPhaseLedger{
			issueopscontract.IssueOpsPhasePlan: {Phase: issueopscontract.IssueOpsPhasePlan, EnteredAt: "2026-09-08T00:00:00Z", CompletedAt: "nope"},
		},
	}
	got, warnings := ReviewMetricsForRecord(record)
	if len(warnings) != 2 {
		t.Fatalf("both bad timestamps must warn once each: %v", warnings)
	}
	if got.DevilsAdvocateRounds != 2 {
		t.Fatalf("round counting must survive a bad timestamp: %+v", got)
	}
	if len(got.ReviewRoundGapsSeconds) != 0 || len(got.StageDurationsSeconds) != 0 {
		t.Fatalf("unparseable values must be skipped, not guessed: %+v", got)
	}
}

func TestAggregateReviewMetricsComputesVerdictRatios(t *testing.T) {
	cycles := []issueopscontract.IssueOpsReviewMetricsCycle{
		{DevilsAdvocateRounds: 3, DevilsAdvocateVerdicts: []string{"revise", "revise", "pass"}},
		{DevilsAdvocateRounds: 1, DevilsAdvocateVerdicts: []string{"stop"}},
		{DevilsAdvocateRounds: 0},
	}
	got := AggregateReviewMetrics(cycles)
	if got.Cycles != 3 || got.ReviewedCycles != 2 {
		t.Fatalf("cycle counts wrong: %+v", got)
	}
	if got.MeanRounds != 2 {
		t.Fatalf("mean rounds = %v, want 2 (over reviewed cycles only)", got.MeanRounds)
	}
	if got.ReviseRatio != 0.5 {
		t.Fatalf("revise ratio = %v, want 0.5 (2 of 4 verdicts)", got.ReviseRatio)
	}
	if got.StopRatio != 0.25 {
		t.Fatalf("stop ratio = %v, want 0.25", got.StopRatio)
	}
}

func TestAggregateReviewMetricsOnEmptyInputIsZero(t *testing.T) {
	got := AggregateReviewMetrics(nil)
	if got.Cycles != 0 || got.MeanRounds != 0 || got.ReviseRatio != 0 || got.StopRatio != 0 {
		t.Fatalf("empty aggregate must be zero, got %+v", got)
	}
}
