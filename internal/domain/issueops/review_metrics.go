package issueops

import (
	"fmt"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

// ReviewMetricsForRecord는 record 하나에서 적대 리뷰 지표를 파생한다. 읽기
// 전용이며 record를 바꾸지 않는다. 파싱할 수 없는 시각은 그 항목만 건너뛰고
// warning으로 남긴다 — 지표 하나가 깨졌다고 사이클 전체를 버리면 측정 자체가
// 불가능해진다.
func ReviewMetricsForRecord(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsReviewMetricsCycle, []string) {
	cycle := issueopscontract.IssueOpsReviewMetricsCycle{
		ID:           record.ID,
		Phase:        string(record.Phase),
		RegressCount: len(record.RegressEvents),
	}
	if review := record.ImplementationReview; review != nil {
		cycle.ImplementationReviewVerdict = review.Verdict
	}
	var warnings []string
	if review := record.DevilsAdvocateReview; review != nil {
		stamps := make([]string, 0, len(review.History)+1)
		for _, round := range review.History {
			cycle.DevilsAdvocateVerdicts = append(cycle.DevilsAdvocateVerdicts, round.Verdict)
			stamps = append(stamps, round.RecordedAt)
		}
		cycle.DevilsAdvocateVerdicts = append(cycle.DevilsAdvocateVerdicts, review.Verdict)
		stamps = append(stamps, review.RecordedAt)
		cycle.DevilsAdvocateRounds = len(cycle.DevilsAdvocateVerdicts)
		gaps, gapWarnings := reviewRoundGaps(record.ID, stamps)
		cycle.ReviewRoundGapsSeconds = gaps
		warnings = append(warnings, gapWarnings...)
	}
	durations, ledgerWarnings := stageDurations(record)
	cycle.StageDurationsSeconds = durations
	warnings = append(warnings, ledgerWarnings...)
	return cycle, warnings
}

// reviewRoundGaps는 연속한 두 라운드의 간격을 초로 돌려준다. 한쪽이라도 파싱되지
// 않으면 그 간격은 만들지 않는다 — 추정한 값을 지표로 내보내지 않는다.
func reviewRoundGaps(id string, stamps []string) ([]float64, []string) {
	parsed := make([]*time.Time, len(stamps))
	var warnings []string
	for index, raw := range stamps {
		at, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: devil's-advocate round %d has an unparseable recorded_at %q", id, index+1, raw))
			continue
		}
		value := at
		parsed[index] = &value
	}
	var gaps []float64
	for index := 1; index < len(parsed); index++ {
		if parsed[index-1] == nil || parsed[index] == nil {
			continue
		}
		gaps = append(gaps, parsed[index].Sub(*parsed[index-1]).Seconds())
	}
	return gaps, warnings
}

// stageDurations는 완료된 phase의 체류 시간을 초로 돌려준다. 순회 순서는
// IssueOpsPhases 고정 순서다(map 순서는 비결정적이라 warning 순서가 흔들린다).
func stageDurations(record issueopscontract.IssueOpsRecord) (map[string]float64, []string) {
	if len(record.PhaseLedger) == 0 {
		return nil, nil
	}
	durations := map[string]float64{}
	var warnings []string
	for _, phase := range issueopscontract.IssueOpsPhases {
		entry, ok := record.PhaseLedger[phase]
		if !ok || entry.EnteredAt == "" || entry.CompletedAt == "" {
			continue
		}
		entered, enteredErr := time.Parse(time.RFC3339, entry.EnteredAt)
		completed, completedErr := time.Parse(time.RFC3339, entry.CompletedAt)
		if enteredErr != nil || completedErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: phase %s has an unparseable ledger timestamp", record.ID, phase))
			continue
		}
		durations[string(phase)] = completed.Sub(entered).Seconds()
	}
	if len(durations) == 0 {
		return nil, warnings
	}
	return durations, warnings
}

// AggregateReviewMetrics는 사이클 목록을 가로지르는 요약을 만든다.
func AggregateReviewMetrics(cycles []issueopscontract.IssueOpsReviewMetricsCycle) issueopscontract.IssueOpsReviewMetricsAggregate {
	aggregate := issueopscontract.IssueOpsReviewMetricsAggregate{Cycles: len(cycles)}
	totalRounds, totalVerdicts, revise, stop := 0, 0, 0, 0
	for _, cycle := range cycles {
		if cycle.DevilsAdvocateRounds > 0 {
			aggregate.ReviewedCycles++
			totalRounds += cycle.DevilsAdvocateRounds
		}
		for _, verdict := range cycle.DevilsAdvocateVerdicts {
			totalVerdicts++
			switch verdict {
			case "revise":
				revise++
			case "stop":
				stop++
			}
		}
	}
	if aggregate.ReviewedCycles > 0 {
		aggregate.MeanRounds = float64(totalRounds) / float64(aggregate.ReviewedCycles)
	}
	if totalVerdicts > 0 {
		aggregate.ReviseRatio = float64(revise) / float64(totalVerdicts)
		aggregate.StopRatio = float64(stop) / float64(totalVerdicts)
	}
	return aggregate
}
