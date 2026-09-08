package issueopscli

import (
	"encoding/json"
	"testing"

	issueopscore "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
)

// review-metrics는 읽기 전용 조망이다. 사이클 하나와 저장소 전량 두 경로가
// 같은 지표를 돌려주고, 범위가 모호하면 관측하지 않고 거부한다.
func TestCLIIssueOpsReviewMetricsReadsRoundsAndRejectsAmbiguousScope(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIGitRepoForRemoteVerifyTest(t)
	record, err := issueopscore.StartIssueOps(issueopscore.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{
		Repo:   repo,
		Branch: "77-review-metrics",
	})
	if err != nil {
		t.Fatal(err)
	}
	record.DevilsAdvocateReview = &issueopscontract.IssueOpsDevilsAdvocateReview{
		Verdict: "pass", RecordedAt: "2026-09-08T00:10:00Z",
		History: []issueopscontract.IssueOpsDevilsAdvocateRound{
			{Verdict: "revise", RecordedAt: "2026-09-08T00:00:00Z"},
		},
	}
	if _, err := issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	single := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"review-metrics", "--id", record.ID, "--json"})
	})
	var byID issueopscontract.IssueOpsReviewMetricsResult
	if err := json.Unmarshal([]byte(single), &byID); err != nil {
		t.Fatalf("review-metrics --id should return JSON: %v\n%s", err, single)
	}
	if !byID.OK || len(byID.Cycles) != 1 {
		t.Fatalf("single-cycle metrics should return exactly one cycle: %+v", byID)
	}
	if byID.Cycles[0].DevilsAdvocateRounds != 2 {
		t.Fatalf("rounds = %d, want 2 (history 1 + current)", byID.Cycles[0].DevilsAdvocateRounds)
	}
	if byID.Aggregate.ReviewedCycles != 1 || byID.Aggregate.MeanRounds != 2 {
		t.Fatalf("aggregate should follow the single cycle: %+v", byID.Aggregate)
	}

	byRepo := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"review-metrics", "--repo", repo, "--json"})
	})
	var repoResult issueopscontract.IssueOpsReviewMetricsResult
	if err := json.Unmarshal([]byte(byRepo), &repoResult); err != nil {
		t.Fatalf("review-metrics --repo should return JSON: %v\n%s", err, byRepo)
	}
	if len(repoResult.Cycles) != 1 || repoResult.Cycles[0].ID != record.ID {
		t.Fatalf("repo aggregation should find the started cycle: %+v", repoResult.Cycles)
	}

	// 범위가 둘 다이거나 둘 다 아니면 무엇을 관측할지 정해지지 않는다.
	for _, args := range [][]string{
		{"review-metrics", "--json"},
		{"review-metrics", "--id", record.ID, "--repo", repo, "--json"},
	} {
		if err := runIssueOps(args); err == nil {
			t.Fatalf("ambiguous scope %v must be rejected", args)
		}
	}

	// 오류도 다른 issueops 명령과 같은 JSON 형태여야 스크립트가 파싱할 수 있다.
	missingOut, missingErr := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"review-metrics", "--id", "io-missing", "--json"})
	})
	assertIssueOpsJSONErrorContains(t, missingOut, missingErr, "io-missing")

	scopeOut, scopeErr := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"review-metrics", "--json"})
	})
	assertIssueOpsJSONErrorContains(t, scopeOut, scopeErr, "exactly one of --id or --repo")
}
