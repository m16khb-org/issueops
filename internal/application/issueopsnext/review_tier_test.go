package issueopsnext

import (
	"context"
	"reflect"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

// reviewTierPorts는 티어 관측을 계측한 ports다. 호출 횟수를 세어 implement 이전
// phase에서 git을 읽지 않는다는 계약을 검증한다.
func reviewTierPorts(record issueopscontract.IssueOpsRecord, paths []string, observed bool, calls *int) Ports {
	entries := []issueopsinventorycontract.ListEntry{{ID: record.ID, Phase: record.Phase, Branch: record.Branch}}
	ports := testPorts(entries, map[string]issueopscontract.IssueOpsRecord{record.ID: record})
	ports.Actor = func() (string, string, error) { return "claude", "session-1", nil }
	ports.PlannerDefaults = func(string) (string, string, bool) { return "planner-model", "high", true }
	ports.ReviewEffortForTier = func(host, tier string) string {
		if tier == "docs-only" {
			return "medium"
		}
		return "high"
	}
	ports.ChangedPaths = func(issueopscontract.IssueOpsRecord) ([]string, bool) {
		*calls++
		return paths, observed
	}
	return ports
}

func implementRecordForTier() issueopscontract.IssueOpsRecord {
	return issueopscontract.IssueOpsRecord{ID: "io-tier", Repo: "/repo", Branch: "9-tier", Phase: issueopscontract.IssueOpsPhaseImplement}
}

// 문서만 바뀐 사이클은 side effect 렌즈 하나와 낮은 effort로 검토한다.
func TestNextNarrowsReviewForDocsOnlyChangeSets(t *testing.T) {
	calls := 0
	ports := reviewTierPorts(implementRecordForTier(), []string{".issueops/CAUTIONS.md", "README.md"}, true, &calls)
	result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Review.Tier != "docs-only" {
		t.Fatalf("tier = %q, want docs-only", result.Review.Tier)
	}
	if result.Review.Effort != "medium" {
		t.Fatalf("effort = %q, want medium for docs-only", result.Review.Effort)
	}
	if !reflect.DeepEqual(result.Review.Lenses, []string{"side-effect"}) {
		t.Fatalf("lenses = %v, want [side-effect]", result.Review.Lenses)
	}
	if result.Review.Model != "planner-model" {
		t.Fatalf("model must stay the host planner default: %q", result.Review.Model)
	}
	if calls != 1 {
		t.Fatalf("changed paths must be observed exactly once, got %d", calls)
	}
}

// 계약 표면이 바뀌면 하위 호환 렌즈가 먼저 오고 effort는 기본값을 유지한다.
func TestNextKeepsFullLensesForContractChangeSets(t *testing.T) {
	calls := 0
	ports := reviewTierPorts(implementRecordForTier(), []string{"internal/contract/issueops/types.go"}, true, &calls)
	result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Review.Tier != "contract" || result.Review.Effort != "high" {
		t.Fatalf("contract tier must keep the default effort: %+v", result.Review)
	}
	if len(result.Review.Lenses) != 4 || result.Review.Lenses[0] != "compat" {
		t.Fatalf("contract lenses must lead with compat: %v", result.Review.Lenses)
	}
}

// implement 이전 phase에는 봉인할 변경 집합이 없다. git을 읽지 않는다.
func TestNextDoesNotObserveChangedPathsBeforeImplement(t *testing.T) {
	calls := 0
	record := implementRecordForTier()
	record.Phase = issueopscontract.IssueOpsPhasePlan
	ports := reviewTierPorts(record, []string{".issueops/CAUTIONS.md"}, true, &calls)
	result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("plan phase must not read git, got %d calls", calls)
	}
	if result.Review.Tier != "default" {
		t.Fatalf("tier = %q, want default before implement", result.Review.Tier)
	}
	if result.Review.Effort != "high" {
		t.Fatalf("effort = %q, want the host planner default", result.Review.Effort)
	}
}

// 변경 집합을 관측하지 못하면 추정하지 않고 기본 티어와 경고를 남긴다.
func TestNextWarnsWhenTheChangeSetIsUnobservable(t *testing.T) {
	calls := 0
	ports := reviewTierPorts(implementRecordForTier(), nil, false, &calls)
	result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Review.Tier != "default" {
		t.Fatalf("unobservable change set must fall back to default, got %q", result.Review.Tier)
	}
	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "change set is unobservable") {
			found = true
		}
	}
	if !found {
		t.Fatalf("an unobservable change set must warn: %v", result.Warnings)
	}
}
