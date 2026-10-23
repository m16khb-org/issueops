package issueopscycle

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
)

func TestReadinessPublicOperationsCreateOneScope(t *testing.T) {
	for _, operation := range []struct {
		name string
		run  func(Readiness, model.IssueOpsRecord)
	}{
		{"plan", func(s Readiness, r model.IssueOpsRecord) { s.Plan(r) }},
		{"compatibility", func(s Readiness, r model.IssueOpsRecord) { s.Compatibility(r) }},
		{"implementation", func(s Readiness, r model.IssueOpsRecord) { s.Implementation(r) }},
		{"clean", func(s Readiness, r model.IssueOpsRecord) { s.AISlopClean(r) }},
		{"pr", func(s Readiness, r model.IssueOpsRecord) { s.PR(r) }},
		{"completion", func(s Readiness, r model.IssueOpsRecord) { s.Completion(r, model.IssueOpsPhasePR) }},
		{"observe", func(s Readiness, r model.IssueOpsRecord) { s.ObserveLocalPR(r) }},
		{"strict", func(s Readiness, r model.IssueOpsRecord) { strictPRForTest(s, r) }},
		{"state", func(s Readiness, r model.IssueOpsRecord) { s.StrictPRWithState("state", r) }},
		{"callback", func(s Readiness, r model.IssueOpsRecord) {
			s.StrictPRWithFetch("state", r, func(root string) review.UpstreamFetch { return review.UpstreamFetch{Root: root} })
		}},
		{"prefetch", func(s Readiness, r model.IssueOpsRecord) { s.PrefetchUpstream(r) }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			locked, events := false, []string{}
			s, git := readinessForEffects(t, &locked, &events)
			scopes := 0
			s.NewObservationScope = func(copy Readiness) Readiness {
				scopes++
				copy.Git = git
				return copy
			}
			record := model.IssueOpsRecord{Repo: "/repo", Branch: "feature", Phase: model.IssueOpsPhaseFeedback, Execution: &model.Execution{}}
			operation.run(s, record)
			if scopes != 1 || s.NewObservationScope == nil {
				t.Fatalf("scopes=%d original factory cleared=%v", scopes, s.NewObservationScope == nil)
			}
		})
	}
}

type scopedReadinessGit struct {
	readinessGitFixture
	head   string
	counts func(string) string
	// block, when set, holds Head so concurrent operations overlap.
	block func()
}

func (g *scopedReadinessGit) Head(model.IssueOpsRecord) string {
	if g.block != nil {
		g.block()
	}
	return g.head
}
func (g *scopedReadinessGit) Counts(root string) string {
	if g.counts != nil {
		return g.counts(root)
	}
	return "0 0"
}

func TestReadinessSameInstanceReobservesChangedState(t *testing.T) {
	locked, events := false, []string{}
	s, git := readinessForEffects(t, &locked, &events)
	head := "first"
	scopes := 0
	s.NewObservationScope = func(copy Readiness) Readiness {
		scopes++
		copy.Git = &scopedReadinessGit{readinessGitFixture: *git, head: head}
		return copy
	}
	record := model.IssueOpsRecord{Repo: "/repo", Branch: "feature"}
	first, _ := s.ObserveLocalPR(record)
	head = "second"
	second, _ := s.ObserveLocalPR(record)
	if scopes != 2 || first.CurrentHead != "first" || second.CurrentHead != "second" {
		t.Fatalf("stale operation state: scopes=%d first=%+v second=%+v", scopes, first, second)
	}
}

func TestReadinessConcurrentOperationsHaveIndependentScopes(t *testing.T) {
	locked, events := false, []string{}
	s, git := readinessForEffects(t, &locked, &events)
	s.ObserveChanges = func(model.IssueOpsRecord, string) review.LocalChangeObservation {
		return review.LocalChangeObservation{Verified: true, Fingerprint: "fingerprint"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	started, release := make(chan string, 2), make(chan struct{})
	done := make(chan model.IssueOpsReadiness, 2)
	var scopes atomic.Int32
	s.NewObservationScope = func(copy Readiness) Readiness {
		head := fmt.Sprint(scopes.Add(1))
		copy.Git = &scopedReadinessGit{readinessGitFixture: *git, head: head, block: func() {
			started <- head
			select {
			case <-release:
			case <-ctx.Done():
			}
		}}
		return copy
	}
	for range 2 {
		go func() {
			ready, _ := s.ObserveLocalPR(model.IssueOpsRecord{Repo: "/repo", Branch: "feature"})
			done <- ready
		}()
	}
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("concurrent scopes did not overlap")
		}
	}
	close(release)
	heads := map[string]bool{}
	for range 2 {
		select {
		case result := <-done:
			heads[result.CurrentHead] = true
		case <-ctx.Done():
			t.Fatal("concurrent observations did not finish")
		}
	}
	if scopes.Load() != 2 || len(heads) != 2 {
		t.Fatalf("shared concurrent state: scopes=%d heads=%v", scopes.Load(), heads)
	}
}
