package issueops

import (
	"fmt"
	"strings"
	"time"

	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

// ReviewMetricsDeps는 `--repo` 집계가 필요로 하는 유일한 외부 관측이다. 저장소
// 필터는 inventory list가 이미 소유하므로(worktree common-dir 해석 포함) 여기서
// 다시 만들지 않고 ID 목록만 받는다.
type ReviewMetricsDeps struct {
	// ListCycleIDs는 그 저장소의 사이클 ID와 함께 **읽히지 않은** ID도 돌려준다.
	// 후자를 버리면 집계 모집단이 조용히 줄어든다.
	ListCycleIDs func(stateRoot, repo string) (ids []string, unreadable []string, err error)
}

// ReviewMetrics는 적대 리뷰 지표를 읽어 돌려준다. record를 쓰지 않으며 새 durable
// 상태도 만들지 않는다. `--id`는 사이클 하나, `--repo`는 그 저장소의 전량이다.
func ReviewMetrics(stateRoot, id, repo string, deps ReviewMetricsDeps) (issueops.IssueOpsReviewMetricsResult, error) {
	id, repo = strings.TrimSpace(id), strings.TrimSpace(repo)
	if (id == "") == (repo == "") {
		return issueops.IssueOpsReviewMetricsResult{OK: false}, fmt.Errorf("review metrics requires exactly one of --id or --repo")
	}
	result := issueops.IssueOpsReviewMetricsResult{
		OK:          true,
		Cycles:      []issueops.IssueOpsReviewMetricsCycle{},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	ids := []string{id}
	if repo != "" {
		if deps.ListCycleIDs == nil {
			return issueops.IssueOpsReviewMetricsResult{OK: false}, fmt.Errorf("review metrics repo listing is unavailable")
		}
		listed, unreadable, err := deps.ListCycleIDs(stateRoot, repo)
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
		record, err := ReadIssueOps(stateRoot, cycleID)
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
