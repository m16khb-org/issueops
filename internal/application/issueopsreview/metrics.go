package issueopsreview

import (
	"errors"
	"fmt"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"strings"
	"time"
)

// MetricsReader keeps repository observations explicit and never mutates records.
// ListOutcomes returns the summaries cleanup or prune retained for deleted records whose
// repository matches repo (every summary when repo is empty).
type MetricsReader struct {
	ReadRecord   func(stateRoot, id string) (issueops.Record, error)
	ListCycleIDs func(stateRoot, repo string) (ids, unreadable []string, err error)
	ListOutcomes func(stateRoot, repo string) (outcomes []issueops.CycleOutcome, unreadable []string, err error)
	Now          func() time.Time
}

func (reader MetricsReader) Read(stateRoot, id, repo string) (issueops.ReviewMetricsResult, error) {
	id, repo = strings.TrimSpace(id), strings.TrimSpace(repo)
	if (id == "") == (repo == "") {
		return issueops.ReviewMetricsResult{OK: false}, fmt.Errorf("review metrics requires exactly one of --id or --repo")
	}
	result := issueops.ReviewMetricsResult{
		OK:          true,
		Cycles:      []issueops.ReviewMetricsCycle{},
		GeneratedAt: reader.Now().UTC().Format(time.RFC3339),
	}
	ids := []string{id}
	if repo != "" {
		listed, err := reader.listRepoCycleIDs(stateRoot, repo, &result)
		if err != nil {
			return issueops.ReviewMetricsResult{OK: false}, err
		}
		ids = listed
	}
	live := map[string]bool{}
	for _, cycleID := range ids {
		record, err := reader.ReadRecord(stateRoot, cycleID)
		if err != nil {
			// 단일 조회는 record가 없으면 보존된 결과를 찾고, 그것도 없으면
			// 실패를 그대로 돌려준다. 집계는 읽히지 않는 사이클 하나 때문에
			// 나머지 관측을 버리지 않고 warning으로 남긴다.
			if repo == "" {
				return reader.readRetained(stateRoot, cycleID, result, err)
			}
			result.ReadErrors++
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: unreadable record, excluded from the aggregate (%v)", cycleID, err))
			continue
		}
		live[cycleID] = true
		cycle, warnings := issueopsdomain.ReviewMetricsForRecord(record)
		result.Cycles = append(result.Cycles, cycle)
		result.Warnings = append(result.Warnings, warnings...)
	}
	if repo != "" && reader.ListOutcomes != nil {
		if err := reader.appendRetainedOutcomes(stateRoot, repo, live, &result); err != nil {
			return issueops.ReviewMetricsResult{OK: false}, err
		}
	}
	result.Aggregate = issueopsdomain.AggregateReviewMetrics(result.Cycles)
	return result, nil
}

// listRepoCycleIDs lists the cycles of repo and records every unreadable one as
// a read error and warning on result.
func (reader MetricsReader) listRepoCycleIDs(stateRoot, repo string, result *issueops.ReviewMetricsResult) ([]string, error) {
	if reader.ListCycleIDs == nil {
		return nil, fmt.Errorf("review metrics repo listing is unavailable")
	}
	listed, unreadable, err := reader.ListCycleIDs(stateRoot, repo)
	if err != nil {
		return nil, err
	}
	for _, cycleID := range unreadable {
		result.ReadErrors++
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: unreadable record, excluded from the aggregate", cycleID))
	}
	return listed, nil
}

// appendRetainedOutcomes adds the outcomes cleanup or prune retained for repo, skipping
// any cycle that still has a live record, and reports every retained row
// (live or not) as result.Retention.
func (reader MetricsReader) appendRetainedOutcomes(stateRoot, repo string, live map[string]bool, result *issueops.ReviewMetricsResult) error {
	outcomes, unreadable, err := reader.ListOutcomes(stateRoot, repo)
	if err != nil {
		return err
	}
	for _, cycleID := range unreadable {
		result.ReadErrors++
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: unreadable retained outcome, excluded from the aggregate", cycleID))
	}
	retention := &issueops.ReviewMetricsRetention{}
	var last time.Time
	for _, outcome := range outcomes {
		// 같은 ID의 live record가 있으면 그것이 최신 관측이다.
		if !live[outcome.ID] {
			result.Cycles = append(result.Cycles, outcome.Metrics)
		}
		// retention은 보존 경로가 실행됐는지 보이는 신호라 live 여부와 무관하게 센다.
		// 알 수 없는 outcome 값의 행은 수에도 last 후보에도 넣지 않는다.
		switch outcome.Outcome {
		case issueops.CycleOutcomeFinished:
			retention.Finished++
		case issueops.CycleOutcomeAbandoned:
			retention.Abandoned++
		case issueops.CycleOutcomePruned:
			retention.Pruned++
		default:
			continue
		}
		// 시각은 문자열이 아니라 시점으로 비교한다(오프셋이 달라도 맞다). 해석 실패
		// 행은 수에는 들어가고 last 후보에서만 빠진다. 출력은 UTC RFC3339Nano다.
		if at, err := time.Parse(time.RFC3339Nano, outcome.RetainedAt); err == nil && (retention.LastRetainedAt == "" || at.After(last)) {
			last, retention.LastRetainedAt = at, at.UTC().Format(time.RFC3339Nano)
		}
	}
	result.Retention = retention
	return nil
}

// readRetained answers a single-cycle query whose record is gone from the
// outcome cleanup or prune retained, or returns the original record error.
func (reader MetricsReader) readRetained(stateRoot, id string, result issueops.ReviewMetricsResult, recordErr error) (issueops.ReviewMetricsResult, error) {
	if reader.ListOutcomes == nil {
		return issueops.ReviewMetricsResult{OK: false}, recordErr
	}
	outcomes, _, err := reader.ListOutcomes(stateRoot, "")
	if err != nil {
		return issueops.ReviewMetricsResult{OK: false}, errors.Join(recordErr, err)
	}
	for _, outcome := range outcomes {
		if outcome.ID == id {
			result.Cycles = append(result.Cycles, outcome.Metrics)
			result.Aggregate = issueopsdomain.AggregateReviewMetrics(result.Cycles)
			return result, nil
		}
	}
	return issueops.ReviewMetricsResult{OK: false}, recordErr
}
