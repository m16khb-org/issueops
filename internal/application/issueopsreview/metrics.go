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
// ListOutcomes returns the summaries cleanup retained for deleted records whose
// repository matches repo (every summary when repo is empty).
type MetricsReader struct {
	ReadRecord   func(stateRoot, id string) (issueops.IssueOpsRecord, error)
	ListCycleIDs func(stateRoot, repo string) (ids, unreadable []string, err error)
	ListOutcomes func(stateRoot, repo string) (outcomes []issueops.IssueOpsCycleOutcome, unreadable []string, err error)
	Now          func() time.Time
}

func (reader MetricsReader) Read(stateRoot, id, repo string) (issueops.IssueOpsReviewMetricsResult, error) {
	id, repo = strings.TrimSpace(id), strings.TrimSpace(repo)
	if (id == "") == (repo == "") {
		return issueops.IssueOpsReviewMetricsResult{OK: false}, fmt.Errorf("review metrics requires exactly one of --id or --repo")
	}
	result := issueops.IssueOpsReviewMetricsResult{
		OK:          true,
		Cycles:      []issueops.IssueOpsReviewMetricsCycle{},
		GeneratedAt: reader.Now().UTC().Format(time.RFC3339),
	}
	ids := []string{id}
	if repo != "" {
		if reader.ListCycleIDs == nil {
			return issueops.IssueOpsReviewMetricsResult{OK: false}, fmt.Errorf("review metrics repo listing is unavailable")
		}
		listed, unreadable, err := reader.ListCycleIDs(stateRoot, repo)
		if err != nil {
			return issueops.IssueOpsReviewMetricsResult{OK: false}, err
		}
		ids = listed
		for _, cycleID := range unreadable {
			result.ReadErrors++
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: unreadable record, excluded from the aggregate", cycleID))
		}
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
		outcomes, unreadable, err := reader.ListOutcomes(stateRoot, repo)
		if err != nil {
			return issueops.IssueOpsReviewMetricsResult{OK: false}, err
		}
		for _, cycleID := range unreadable {
			result.ReadErrors++
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: unreadable retained outcome, excluded from the aggregate", cycleID))
		}
		for _, outcome := range outcomes {
			// 같은 ID의 live record가 있으면 그것이 최신 관측이다.
			if !live[outcome.ID] {
				result.Cycles = append(result.Cycles, outcome.Metrics)
			}
		}
	}
	result.Aggregate = issueopsdomain.AggregateReviewMetrics(result.Cycles)
	return result, nil
}

// readRetained answers a single-cycle query whose record is gone from the
// outcome cleanup retained, or returns the original record error.
func (reader MetricsReader) readRetained(stateRoot, id string, result issueops.IssueOpsReviewMetricsResult, recordErr error) (issueops.IssueOpsReviewMetricsResult, error) {
	if reader.ListOutcomes == nil {
		return issueops.IssueOpsReviewMetricsResult{OK: false}, recordErr
	}
	outcomes, _, err := reader.ListOutcomes(stateRoot, "")
	if err != nil {
		return issueops.IssueOpsReviewMetricsResult{OK: false}, errors.Join(recordErr, err)
	}
	for _, outcome := range outcomes {
		if outcome.ID == id {
			result.Cycles = append(result.Cycles, outcome.Metrics)
			result.Aggregate = issueopsdomain.AggregateReviewMetrics(result.Cycles)
			return result, nil
		}
	}
	return issueops.IssueOpsReviewMetricsResult{OK: false}, recordErr
}
