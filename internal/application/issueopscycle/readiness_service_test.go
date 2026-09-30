package issueopscycle

import (
	"errors"
	model "issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
	port "issueops/internal/port/issueopscycle"
	"reflect"
	"strings"
	"testing"
)

type readinessGitFixture struct {
	root   string
	locked *bool
	events *[]string
	t      *testing.T
}

func (g *readinessGitFixture) Root(model.IssueOpsRecord) string { return g.root }
func (g *readinessGitFixture) Head(model.IssueOpsRecord) string { return "head" }
func (g *readinessGitFixture) IsWorktree(string) bool           { return true }
func (g *readinessGitFixture) Branch(string) string             { return "feature" }
func (g *readinessGitFixture) Clean(string) bool                { return true }
func (g *readinessGitFixture) BaseAdvanced(string, string) bool { return false }
func (g *readinessGitFixture) Upstream(string) string           { return "origin/feature" }
func (g *readinessGitFixture) Counts(string) string             { return "0 0" }
func (g *readinessGitFixture) Fetch(root string) review.UpstreamFetch {
	if *g.locked {
		g.t.Fatal("network fetch executed under lifecycle lock")
	}
	*g.events = append(*g.events, "fetch")
	return review.UpstreamFetch{Root: root}
}
func readinessForEffects(t *testing.T, locked *bool, events *[]string) (Readiness, *readinessGitFixture) {
	git := &readinessGitFixture{root: "/repo", locked: locked, events: events, t: t}
	ready := Readiness{
		Git: git, Paths: port.ReadinessObservations{
			WorktreePathValid: func(string) bool { return true }, PlanPathExists: func(string, string) bool { return true }, PlanInLinkedWorktree: func(model.IssueOpsRecord) bool { return true }, WorkspaceMatches: func(string, string) bool { return true }, LinkedPlanDigest: func(model.IssueOpsRecord) (string, error) { return "", nil }},
		HasEvidence: func(model.IssueOpsRecord) bool { return true },
		ObserveChanges: func(model.IssueOpsRecord, string) review.LocalChangeObservation {
			*events = append(*events, "observe")
			return review.LocalChangeObservation{Paths: []string{"src.go"}, Fingerprint: "fingerprint", Verified: true}
		},
		ChangedPaths: func(model.IssueOpsRecord) []string { *events = append(*events, "paths"); return []string{"src.go"} },
		Cleanup:      func(model.IssueOpsRecord) model.IssueOpsCleanupStatus { return model.IssueOpsCleanupStatus{} },
		ChildMissing: func(string, model.IssueOpsRecord) ([]string, []string) { return nil, nil },
	}
	return ready, git
}

func TestReadinessLocalSharesSnapshotAndStrictObservesPathsAfterFetch(t *testing.T) {
	locked := false
	events := []string{}
	ready, _ := readinessForEffects(t, &locked, &events)
	record := model.IssueOpsRecord{Repo: "/repo", Branch: "feature", Phase: model.IssueOpsPhaseFeedback, Execution: &model.Execution{Mode: model.ExecutionModeDirect}}
	local, changes := ready.ObserveLocalPR(record)
	if local.Strict || !changes.Verified || !reflect.DeepEqual(events, []string{"observe"}) {
		t.Fatalf("local observation performed extra effects: %v %+v", events, changes)
	}
	events = nil
	strict := ready.StrictPR(record)
	if !strict.Strict || !reflect.DeepEqual(events, []string{"observe", "fetch", "paths"}) {
		t.Fatalf("strict ordering=%v", events)
	}
}

func TestReadinessPrefetchRejectsChangedRootWithoutAnotherFetch(t *testing.T) {
	locked := false
	events := []string{}
	ready, git := readinessForEffects(t, &locked, &events)
	record := model.IssueOpsRecord{Repo: "/repo", Branch: "feature", Phase: model.IssueOpsPhaseFeedback, Execution: &model.Execution{Mode: model.ExecutionModeDirect}}
	fetched := ready.PrefetchUpstream(record)
	git.root = "/other"
	locked = true
	result := ready.StrictPRWithFetch("state", record, fetched)
	if !strings.Contains(strings.Join(result.Missing, ","), "upstream_fetch") || !reflect.DeepEqual(events, []string{"fetch", "observe", "paths"}) {
		t.Fatalf("changed root was not refused with bounded effects: missing=%v events=%v", result.Missing, events)
	}
}

func TestPhaseAdvanceAuthorizesBeforePrefetchAndAgainUnderLock(t *testing.T) {
	for _, denyAt := range []int{0, 1, 2} {
		t.Run(string(rune('0'+denyAt)), func(t *testing.T) {
			locked := false
			events := []string{}
			calls, writes := 0, 0
			ready, _ := readinessForEffects(t, &locked, &events)
			record := model.IssueOpsRecord{ID: "io-phase", Repo: "/repo", Branch: "feature", Phase: model.IssueOpsPhaseFeedback, Execution: &model.Execution{Mode: model.ExecutionModeDirect}}
			denied := errors.New("holder denied")
			service := PhaseService{Readiness: ready, Store: port.PhaseStore{
				Read: func(string, string) (model.IssueOpsRecord, error) {
					events = append(events, "read")
					return record, nil
				},
				WithLock: func(_, _ string, fn func() error) error {
					events = append(events, "lock")
					locked = true
					defer func() { locked = false }()
					return fn()
				},
				ValidateMutation: func(model.IssueOpsRecord) error {
					calls++
					events = append(events, "authorize")
					if calls == denyAt {
						return denied
					}
					return nil
				},
				Write: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) { writes++; return r, nil },
			}}
			_, err := service.Advance("state", record.ID, "pr")
			if err == nil || writes != 0 {
				t.Fatalf("incomplete/unauthorized phase must not persist: err=%v writes=%d", err, writes)
			}
			want := []string{"read", "authorize"}
			if denyAt != 1 {
				want = append(want, "fetch", "lock", "read", "authorize")
			}
			if denyAt == 0 {
				want = append(want, "read", "observe", "paths")
			}
			if !reflect.DeepEqual(events, want) {
				t.Fatalf("phase effect order=%v want=%v", events, want)
			}
			if denyAt != 0 && !errors.Is(err, denied) {
				t.Fatalf("authority error lost: %v", err)
			}
		})
	}
}
