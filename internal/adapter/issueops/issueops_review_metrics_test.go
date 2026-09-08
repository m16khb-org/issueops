package issueops

import (
	"path/filepath"
	"strings"
	"testing"
)

// 집계 모집단이 조용히 줄면 revise 비율이 어떤 사이클들에서 나온 값인지 알 수 없다.
// 읽히지 않는 사이클은 버리지 말고 경고로 드러낸다.
func TestReviewMetricsWarnsAboutUnreadableCyclesDuringRepoAggregation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	result, err := ReviewMetrics(stateRoot, "", "/repo", ReviewMetricsDeps{
		ListCycleIDs: func(string, string) ([]string, []string, error) {
			return nil, []string{"io-corrupt"}, nil
		},
	})
	if err != nil {
		t.Fatalf("an unreadable cycle must not fail the aggregation: %v", err)
	}
	if !result.OK || len(result.Cycles) != 0 {
		t.Fatalf("no readable cycle means no metrics rows: %+v", result)
	}
	if result.ReadErrors != 1 {
		t.Fatalf("read_errors = %d, want 1", result.ReadErrors)
	}
	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "io-corrupt") && strings.Contains(warning, "unreadable") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the unreadable cycle must be named in a warning: %v", result.Warnings)
	}
}
