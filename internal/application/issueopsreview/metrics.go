package issueopsreview

import (
	"fmt"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"strings"
	"time"
)

// MetricsReader keeps repository observations explicit and never mutates records.
type MetricsReader struct {
	ReadRecord   func(stateRoot, id string) (issueops.IssueOpsRecord, error)
	ListCycleIDs func(stateRoot, repo string) (ids, unreadable []string, err error)
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
	for _, cycleID := range ids {
		record, err := reader.ReadRecord(stateRoot, cycleID)
		if err != nil {
			// 단일 조회는 실패를 그대로 돌려준다. 집계는 읽히지 않는 사이클
			// 하나 때문에 나머지 관측을 버리지 않고 warning으로 남긴다.
			if repo == "" {
				return issueops.IssueOpsReviewMetricsResult{OK: false}, err
			}
			result.ReadErrors++
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: unreadable record, excluded from the aggregate (%v)", cycleID, err))
			continue
		}
		cycle, warnings := issueopsdomain.ReviewMetricsForRecord(record)
		result.Cycles = append(result.Cycles, cycle)
		result.Warnings = append(result.Warnings, warnings...)
	}
	result.Aggregate = issueopsdomain.AggregateReviewMetrics(result.Cycles)
	return result, nil
}
