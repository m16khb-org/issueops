package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
)

func TestReadinessFetchBoundariesRefreshBeforePostFetchFacts(t *testing.T) {
	for _, mode := range []string{"internal", "callback", "prefetch"} {
		t.Run(mode, func(t *testing.T) {
			locked, events := false, []string{}
			s, git := readinessForEffects(t, &locked, &events)
			scopes := 0
			s.NewObservationScope = func(copy Readiness) Readiness {
				scopes++
				scope := scopes
				copy.Git = &scopedReadinessGit{readinessGitFixture: *git, counts: func(string) string {
					events = append(events, "counts")
					if scope != scopes {
						t.Errorf("post-fetch counts used earlier scope %d, current %d", scope, scopes)
					}
					return "0 0"
				}}
				copy.ClearGitObservations = func() { events = append(events, "clear") }
				return copy
			}
			record := model.IssueOpsRecord{Repo: "/repo", Branch: "feature", Phase: model.IssueOpsPhaseFeedback, Execution: &model.Execution{Mode: model.ExecutionModeDirect}}
			want := []string{"observe", "clear", "fetch", "clear", "counts", "paths"}
			switch mode {
			case "internal":
				strictPRForTest(s, record)
			case "callback":
				s.StrictPRWithFetch("state", record, func(root string) review.UpstreamFetch {
					events = append(events, "fetch")
					return review.UpstreamFetch{Root: root}
				})
			case "prefetch":
				fetched := s.PrefetchUpstream(record)
				locked = true
				s.StrictPRWithFetch("state", record, fetched)
				want = []string{"fetch", "observe", "clear", "clear", "counts", "paths"}
			}
			wantScopes := 1
			if mode == "prefetch" {
				wantScopes = 2
			}
			if scopes != wantScopes || !reflect.DeepEqual(events, want) {
				t.Fatalf("scopes=%d events=%v want=%v", scopes, events, want)
			}
		})
	}
}
