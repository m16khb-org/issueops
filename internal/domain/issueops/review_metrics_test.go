package issueops

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

// 리뷰 지표는 record가 이미 들고 있는 원자료(devil's-advocate history, regress
// 이벤트, phase ledger)에서 파생된다. 새 상태를 만들지 않고 읽기만 한다.
func TestReviewMetricsForRecordCountsRoundsVerdictsAndDurations(t *testing.T) {
	record := issueopscontract.Record{
		ID:    "io-metrics",
		Phase: issueopscontract.PhaseImplement,
		DevilsAdvocateReview: &issueopscontract.DevilsAdvocateReview{
			Verdict: "pass", RecordedAt: "2026-09-08T00:10:00Z",
			History: []issueopscontract.DevilsAdvocateRound{
				{Verdict: "revise", RecordedAt: "2026-09-08T00:00:00Z"},
				{Verdict: "revise", RecordedAt: "2026-09-08T00:04:00Z"},
			},
		},
		RegressEvents:        []issueopscontract.RegressEvent{{Reason: "stop", FromPhase: issueopscontract.PhasePlan, At: "2026-09-08T00:05:00Z"}},
		ImplementationReview: &issueopscontract.ImplementationReview{Verdict: "pass"},
		PhaseLedger: issueopscontract.PhaseLedger{
			issueopscontract.PhasePlan: {
				Phase:       issueopscontract.PhasePlan,
				EnteredAt:   "2026-09-08T00:00:00Z",
				CompletedAt: "2026-09-08T00:01:00Z",
			},
			// 완료되지 않은 phase는 소요를 만들지 않는다.
			issueopscontract.PhaseImplement: {
				Phase:     issueopscontract.PhaseImplement,
				EnteredAt: "2026-09-08T00:01:00Z",
			},
		},
	}

	got, warnings := ReviewMetricsForRecord(record)
	if len(warnings) != 0 {
		t.Fatalf("parseable record must not warn: %v", warnings)
	}
	if got.ID != "io-metrics" || got.Phase != string(issueopscontract.PhaseImplement) {
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
	got, warnings := ReviewMetricsForRecord(issueopscontract.Record{ID: "io-bare", Phase: issueopscontract.PhasePlan})
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
	record := issueopscontract.Record{
		ID: "io-bad-time",
		DevilsAdvocateReview: &issueopscontract.DevilsAdvocateReview{
			Verdict: "pass", RecordedAt: "not-a-time",
			History: []issueopscontract.DevilsAdvocateRound{{Verdict: "revise", RecordedAt: "2026-09-08T00:00:00Z"}},
		},
		PhaseLedger: issueopscontract.PhaseLedger{
			issueopscontract.PhasePlan: {Phase: issueopscontract.PhasePlan, EnteredAt: "2026-09-08T00:00:00Z", CompletedAt: "nope"},
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
	cycles := []issueopscontract.ReviewMetricsCycle{
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

// 보존되는 outcome은 record가 지워지기 전의 타임라인을 그대로 싣는다.
func TestCycleOutcomeCarriesTimeline(t *testing.T) {
	record := issueopscontract.Record{
		ID:            "io-timeline",
		Repo:          "/repo",
		Phase:         issueopscontract.PhaseDone,
		CreatedAt:     "2026-09-08T00:00:00Z",
		RegressEvents: []issueopscontract.RegressEvent{{Reason: "stop", FromPhase: issueopscontract.PhasePlan, At: "2026-09-08T00:05:00Z"}},
	}

	outcome := CycleOutcomeForRecord(record, issueopscontract.CycleOutcomeFinished, "2026-09-08T01:00:00Z")

	if outcome.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1 (timeline is additive)", outcome.SchemaVersion)
	}
	if outcome.Timeline == nil {
		t.Fatal("outcome carries no timeline")
	}
	if want := BuildCycleTimeline(record); !reflect.DeepEqual(*outcome.Timeline, want) {
		t.Fatalf("timeline = %+v, want %+v", *outcome.Timeline, want)
	}
	if outcome.Timeline.EventsTotal == 0 {
		t.Fatalf("timeline has no events: %+v", *outcome.Timeline)
	}
}

// 타임라인이 가장 큰 모양이어도 보존 row는 정해진 크기 안에 머문다.
func TestCycleOutcomeRowStaysBounded(t *testing.T) {
	long := strings.Repeat("s", 200)
	record := issueopscontract.Record{ID: "io-bounded", Repo: "/repo", CreatedAt: "2026-09-08T00:00:00Z"}
	base := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	for i := range 500 {
		at := base.Add(time.Duration(i) * time.Second).Format(time.RFC3339)
		// A child cycle pair carries both subject and ref, the widest shape that
		// survives timelineText. The widest characters (the ones JSON escapes) are
		// covered by TestCycleOutcomeRowStaysBoundedWithEscapedText.
		record.ChildCycles = append(record.ChildCycles, issueopscontract.ChildCycleRef{
			CycleID: long, CreatedAt: at, ValidationVerdict: long, ValidatedAt: at,
		})
	}

	outcome := CycleOutcomeForRecord(record, issueopscontract.CycleOutcomeAbandoned, "2026-09-08T01:00:00Z")

	data, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 65536 {
		t.Fatalf("outcome row is %d bytes, want <= 65536", len(data))
	}
	if outcome.Timeline == nil || outcome.Timeline.EventsShown != issueopscontract.MaxTimelineEvents {
		t.Fatalf("timeline = %+v, want %d events shown", outcome.Timeline, issueopscontract.MaxTimelineEvents)
	}
	if outcome.Timeline.EventsTotal < 1000 {
		t.Fatalf("total = %d, want the full 1000+ dated events counted", outcome.Timeline.EventsTotal)
	}
}

// JSON이 여러 배로 escape하는 문자는 타임라인에서 "?"가 되므로, 그런 skill 이름
// 200건이어도 보존 row는 정해진 크기 안에 머문다.
func TestCycleOutcomeRowStaysBoundedWithEscapedText(t *testing.T) {
	hostile := strings.Repeat("<", 64)
	record := issueopscontract.Record{ID: "io-escaped", Repo: "/repo", CreatedAt: "2026-09-08T00:00:00Z"}
	base := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	for i := range issueopscontract.MaxTimelineEvents {
		record.RoutingTrace = append(record.RoutingTrace, issueopscontract.SkillRoutingEntry{
			Phase: hostile, Skill: hostile, At: base.Add(time.Hour + time.Duration(i)*time.Second).Format(time.RFC3339),
		})
	}
	for i := range 50 {
		at := base.Add(time.Duration(i) * time.Second).Format(time.RFC3339)
		record.ChildCycles = append(record.ChildCycles, issueopscontract.ChildCycleRef{
			CycleID: "io-child", CreatedAt: at, ValidationVerdict: "accepted", ValidatedAt: at,
		})
		record.Decisions = append(record.Decisions, issueopscontract.Decision{Kind: "scope", CreatedAt: at})
	}

	outcome := CycleOutcomeForRecord(record, issueopscontract.CycleOutcomeFinished, "2026-09-08T02:00:00Z")

	data, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 65536 {
		t.Fatalf("outcome row is %d bytes, want <= 65536", len(data))
	}
	if strings.Contains(string(data), `\u003c`) {
		t.Fatalf("row still carries escaped characters: %s", data)
	}
	var skills int
	for _, event := range outcome.Timeline.Events {
		if event.Kind != issueopscontract.TimelineKindSkillRouted {
			continue
		}
		skills++
		if event.Subject != "?" || event.Phase != "?" {
			t.Fatalf("skill subject = %q, phase = %q, want %q for both", event.Subject, event.Phase, "?")
		}
	}
	// The skill entries are the newest events, so all 200 fill the cap and
	// the older child and decision events are only counted.
	if skills != issueopscontract.MaxTimelineEvents || outcome.Timeline.EventsTotal != 351 {
		t.Fatalf("skill events kept = %d, total = %d, want 200 and 351", skills, outcome.Timeline.EventsTotal)
	}
}
