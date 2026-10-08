package issueopsnext

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	agentmodelcontract "issueops/internal/contract/agentmodel"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

// reviewTierPorts는 티어 관측을 계측한 ports다. 호출 횟수를 세어 implement 이전
// phase에서 git을 읽지 않는다는 계약을 검증한다.
func reviewTierPorts(record issueopscontract.IssueOpsRecord, paths []string, observed bool, calls *int) Ports {
	entries := []issueopsinventorycontract.ListEntry{{ID: record.ID, Phase: record.Phase, Branch: record.Branch}}
	ports := testPorts(entries, map[string]issueopscontract.IssueOpsRecord{record.ID: record})
	ports.Actor = func() (string, string, error) { return "claude", "session-1", nil }
	ports.ReviewModel = func(host string, role agentmodelcontract.Role, tier, repo string) (string, string, error) {
		if tier == "docs-only" {
			return "planner-model", "medium", nil
		}
		return "planner-model", "high", nil
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

// frontend 신호는 티어와 독립이며 관측에 성공했을 때만 켜진다.
func TestNextFlagsFrontendChangeSetsWithoutChangingTier(t *testing.T) {
	calls := 0
	ports := reviewTierPorts(implementRecordForTier(), []string{"src/pages/Home.tsx", "internal/adapter/x.go"}, true, &calls)
	result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Review.Frontend {
		t.Fatalf("a .tsx change must raise the frontend signal: %+v", result.Review)
	}
	if result.Review.Tier != "default" {
		t.Fatalf("the frontend signal must not change the tier, got %q", result.Review.Tier)
	}

	calls = 0
	docs := reviewTierPorts(implementRecordForTier(), []string{".issueops/CAUTIONS.md"}, true, &calls)
	docsResult, err := NewService(docs).Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatal(err)
	}
	if docsResult.Review.Frontend || docsResult.Review.Tier != "docs-only" {
		t.Fatalf("a docs-only change set carries no frontend signal: %+v", docsResult.Review)
	}

	calls = 0
	unobserved := reviewTierPorts(implementRecordForTier(), []string{"src/pages/Home.tsx"}, false, &calls)
	unobservedResult, err := NewService(unobserved).Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatal(err)
	}
	if unobservedResult.Review.Frontend {
		t.Fatal("an unobservable change set must not raise a signal it did not see")
	}
}

// plan 이전에는 plan-review, implement 이후에는 diff-review 역할로 해석한다.
func TestNextResolvesTheReviewRoleByPhase(t *testing.T) {
	for phase, want := range map[issueopscontract.IssueOpsPhase]agentmodelcontract.Role{
		issueopscontract.IssueOpsPhasePlan:        agentmodelcontract.RolePlanReview,
		issueopscontract.IssueOpsPhaseImplement:   agentmodelcontract.RoleDiffReview,
		issueopscontract.IssueOpsPhaseAISlopClean: agentmodelcontract.RoleDiffReview,
	} {
		calls := 0
		record := implementRecordForTier()
		record.Phase = phase
		ports := reviewTierPorts(record, []string{"internal/x.go"}, true, &calls)
		var roles []agentmodelcontract.Role
		var repos []string
		ports.ReviewModel = func(host string, role agentmodelcontract.Role, tier, repo string) (string, string, error) {
			roles, repos = append(roles, role), append(repos, repo)
			return "m-" + string(role), "xhigh", nil
		}
		result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "")
		if err != nil {
			t.Fatal(err)
		}
		if result.Review.Model != "m-"+string(want) || result.Review.Effort != "xhigh" || roles[len(roles)-1] != want || repos[len(repos)-1] != "/repo" {
			t.Fatalf("%s: review = %+v roles=%v repos=%v", phase, result.Review, roles, repos)
		}
	}
}

// 깨진 설정 파일은 기본값으로 조용히 대체하지 않는다. 경고를 한 번 남기고
// review 값을 비우며, tier와 lenses는 그대로 계산한다.
func TestNextWarnsOnceAndBlanksReviewWhenSettingsAreBroken(t *testing.T) {
	calls := 0
	ports := reviewTierPorts(implementRecordForTier(), []string{"README.md"}, true, &calls)
	ports.ReviewModel = func(string, agentmodelcontract.Role, string, string) (string, string, error) {
		return "", "", errors.New("/cfg/issueops/agent-models.json: json: unknown field \"modle\"")
	}
	result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Review.Model != "" || result.Review.Effort != "" || result.Review.Tier != "docs-only" || len(result.Review.Lenses) == 0 {
		t.Fatalf("review = %+v", result.Review)
	}
	count := 0
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "agent model settings are invalid: /cfg/issueops/agent-models.json") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}
