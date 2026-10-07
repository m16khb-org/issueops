package devilsadvocate

import (
	reviewcontract "issueops/internal/contract/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

// storeWithRounds는 이미 기록된 라운드를 가진 record를 돌려주는 최소 store다.
func storeWithRounds(rounds ...model.IssueOpsDevilsAdvocateRound) (reviewport.DevilsAdvocateStore, *model.IssueOpsRecord) {
	written := &model.IssueOpsRecord{}
	var review *model.IssueOpsDevilsAdvocateReview
	if len(rounds) > 0 {
		current := rounds[len(rounds)-1]
		review = &model.IssueOpsDevilsAdvocateReview{
			Verdict: current.Verdict, Waived: current.Waived, WaiverRationale: current.WaiverRationale,
			Findings: current.Findings, RecordedAt: current.RecordedAt,
			History: append([]model.IssueOpsDevilsAdvocateRound{}, rounds[:len(rounds)-1]...),
		}
	}
	return reviewport.DevilsAdvocateStore{
		Read: func(_, id string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{OK: true, ID: id, PlanPath: "plans/demo.md", DevilsAdvocateReview: review}, nil
		},
		TouchWrite: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			*written = r
			return r, nil
		},
		PlanDigest: func(string, model.IssueOpsRecord) (string, error) { return "digest", nil },
	}, written
}

func reviseRound(findings string) model.IssueOpsDevilsAdvocateRound {
	return model.IssueOpsDevilsAdvocateRound{Verdict: "revise", Findings: []string{findings}, RecordedAt: "2026-09-08T00:00:00Z"}
}

// 같은 plan phase에서 revise가 다섯 번 나왔다면 계획이 수렴하지 않는 것이다.
// 여섯 번째 revise는 거부하고, 실제로 열려 있는 탈출 경로만 안내한다.
func TestRecordRejectsSixthUnwaivedReviseRound(t *testing.T) {
	store, written := storeWithRounds(reviseRound("one"), reviseRound("two"), reviseRound("three"), reviseRound("four"), reviseRound("five"))
	_, err := Record(store, "state", "io-cap", reviewcontract.DevilsAdvocateReviewRequest{
		Verdict: "revise", ReviewerContext: "subagent", Findings: []string{"six"},
	})
	if err == nil {
		t.Fatal("the sixth unwaived revise must be rejected")
	}
	for _, want := range []string{"revise round cap reached", "reflect-devils-advocate", "--waive"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("cap error must name the open exit %q: %v", want, err)
		}
	}
	// `revise` 상태에서는 regress가 거부되므로 그 명령을 직접 권해서는 안 된다.
	if strings.Contains(err.Error(), "issueops regress --id io-cap --reason") &&
		!strings.Contains(err.Error(), "stop") {
		t.Fatalf("cap error must not route straight to regress: %v", err)
	}
	if written.ID != "" {
		t.Fatalf("a rejected round must not be written: %+v", written)
	}
}

// pass와 stop은 사이클을 앞이나 뒤로 보내는 판정이므로 cap과 무관하다.
func TestRecordAllowsTerminalVerdictsAfterFiveRevises(t *testing.T) {
	for _, verdict := range []string{"pass", "stop"} {
		store, written := storeWithRounds(reviseRound("one"), reviseRound("two"), reviseRound("three"), reviseRound("four"), reviseRound("five"))
		if _, err := Record(store, "state", "io-cap", reviewcontract.DevilsAdvocateReviewRequest{
			Verdict: verdict, ReviewerContext: "subagent", Findings: []string{"attacked and settled"},
		}); err != nil {
			t.Fatalf("%s after five revises must be allowed: %v", verdict, err)
		}
		if written.DevilsAdvocateReview == nil || written.DevilsAdvocateReview.Verdict != verdict {
			t.Fatalf("%s round must be persisted: %+v", verdict, written.DevilsAdvocateReview)
		}
		if len(written.DevilsAdvocateReview.History) != 5 {
			t.Fatalf("history must keep the five earlier rounds: %+v", written.DevilsAdvocateReview.History)
		}
	}
}

// waiver는 의도적인 override이므로 통과하고, 카운트에도 들어가지 않는다.
func TestRecordAllowsWaivedReviseAndExcludesItFromTheCount(t *testing.T) {
	store, _ := storeWithRounds(reviseRound("one"), reviseRound("two"), reviseRound("three"), reviseRound("four"), reviseRound("five"))
	if _, err := Record(store, "state", "io-cap", reviewcontract.DevilsAdvocateReviewRequest{
		Verdict: "revise", ReviewerContext: "subagent", Waived: true, WaiverRationale: "scoped follow-up issue filed",
	}); err != nil {
		t.Fatalf("a waived revise must be allowed past the cap: %v", err)
	}

	waived := model.IssueOpsDevilsAdvocateRound{Verdict: "revise", Waived: true, WaiverRationale: "override", RecordedAt: "2026-09-08T00:00:00Z"}
	store, _ = storeWithRounds(reviseRound("one"), reviseRound("two"), waived, reviseRound("three"), reviseRound("four"), reviseRound("five"))
	if _, err := Record(store, "state", "io-cap", reviewcontract.DevilsAdvocateReviewRequest{
		Verdict: "revise", ReviewerContext: "subagent", Findings: []string{"six"},
	}); err == nil {
		t.Fatal("five unwaived revises must hit the cap even with a waived round between them")
	}
	store, written := storeWithRounds(reviseRound("one"), waived, reviseRound("two"), reviseRound("three"), reviseRound("four"))
	if _, err := Record(store, "state", "io-cap", reviewcontract.DevilsAdvocateReviewRequest{
		Verdict: "revise", ReviewerContext: "subagent", Findings: []string{"fifth unwaived"},
	}); err != nil {
		t.Fatalf("four unwaived revises plus one waived must stay under the cap: %v", err)
	}
	if written.DevilsAdvocateReview == nil {
		t.Fatal("the allowed round must be written")
	}
}

// 첫 라운드부터 다섯 번째까지는 그대로 통과한다.
func TestRecordAllowsTheFirstFiveReviseRounds(t *testing.T) {
	store, _ := storeWithRounds(reviseRound("one"), reviseRound("two"), reviseRound("three"), reviseRound("four"))
	if _, err := Record(store, "state", "io-cap", reviewcontract.DevilsAdvocateReviewRequest{
		Verdict: "revise", ReviewerContext: "subagent", Findings: []string{"five"},
	}); err != nil {
		t.Fatalf("the fifth revise must be allowed: %v", err)
	}
}
