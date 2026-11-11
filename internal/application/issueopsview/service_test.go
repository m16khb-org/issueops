package issueopsview

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsviewcontract "issueops/internal/contract/issueopsview"
	issueopsdomain "issueops/internal/domain/issueops"
)

// ctxKey keys the value a test puts in the context View receives, so a fake
// port can prove it got that same context.
type ctxKey struct{}

type fakeReads struct {
	record        issueopsviewcontract.Record
	recordErr     error
	outcome       issueopsviewcontract.CycleOutcome
	outcomeFound  bool
	outcomeErr    error
	recordCalls   int
	retainedCalls int
	stateRoots    []string
	ids           []string
	// contexts are the contexts the ports received, in call order.
	contexts []context.Context
}

func (fake *fakeReads) ports(now time.Time) Ports {
	return Ports{
		ReadRecord: func(ctx context.Context, stateRoot, id string) (issueopsviewcontract.Record, error) {
			fake.recordCalls++
			fake.contexts = append(fake.contexts, ctx)
			if err := ctx.Err(); err != nil {
				return issueopsviewcontract.Record{}, err
			}
			fake.stateRoots = append(fake.stateRoots, stateRoot)
			fake.ids = append(fake.ids, id)
			return fake.record, fake.recordErr
		},
		RetainedOutcome: func(ctx context.Context, stateRoot, id string) (issueopsviewcontract.CycleOutcome, bool, error) {
			fake.retainedCalls++
			fake.contexts = append(fake.contexts, ctx)
			fake.stateRoots = append(fake.stateRoots, stateRoot)
			fake.ids = append(fake.ids, id)
			return fake.outcome, fake.outcomeFound, fake.outcomeErr
		},
		Now: func() time.Time { return now },
	}
}

func notExist(id string) error {
	return fmt.Errorf("issueops record %s not found: %w", id, fs.ErrNotExist)
}

func TestViewServiceTimeline(t *testing.T) {
	now := time.Date(2026, 11, 11, 3, 4, 5, 0, time.FixedZone("KST", 9*3600))
	const wantNow = "2026-11-10T18:04:05Z"
	record := issueopsviewcontract.Record{
		ID: "io-live", CreatedAt: "2026-09-07T00:00:00Z",
		RegressEvents: []issueopscontract.RegressEvent{{At: "2026-09-08T00:00:00Z", FromPhase: issueopscontract.PhasePlan}},
	}
	retainedTimeline := issueopsdomain.BuildCycleTimeline(record)
	run := func(fake *fakeReads, id, view string) (issueopsviewcontract.Result, error) {
		return NewService(fake.ports(now)).View(context.Background(), "/state", "/cwd", id, view)
	}

	t.Run("live record builds the timeline from the record", func(t *testing.T) {
		fake := &fakeReads{record: record}
		got, err := run(fake, "io-live", "timeline")
		if err != nil {
			t.Fatal(err)
		}
		want := issueopsdomain.BuildCycleTimeline(record)
		if !got.OK || got.View != "timeline" || got.ID != "io-live" || got.Source != issueopsviewcontract.SourceRecord || got.GeneratedAt != wantNow {
			t.Fatalf("result = %+v", got)
		}
		if got.Outcome != "" || got.RetainedAt != "" {
			t.Fatalf("a live result carries no retained fields: %+v", got)
		}
		if got.Timeline == nil || !reflect.DeepEqual(*got.Timeline, want) {
			t.Fatalf("timeline = %+v, want %+v", got.Timeline, want)
		}
		if fake.retainedCalls != 0 || fake.stateRoots[0] != "/state" {
			t.Fatalf("live path read the retained row or lost the state root: %+v", fake)
		}
	})

	t.Run("missing record falls back to the retained outcome with its timeline", func(t *testing.T) {
		fake := &fakeReads{
			recordErr:    notExist("io-gone"),
			outcome:      issueopsviewcontract.CycleOutcome{SchemaVersion: 1, ID: "io-gone", Outcome: "abandoned", RetainedAt: "2026-09-09T00:00:00Z", Timeline: &retainedTimeline},
			outcomeFound: true,
		}
		got, err := run(fake, "io-gone", "timeline")
		if err != nil {
			t.Fatal(err)
		}
		if !got.OK || got.ID != "io-gone" || got.Source != issueopsviewcontract.SourceRetained || got.Outcome != "abandoned" || got.RetainedAt != "2026-09-09T00:00:00Z" || got.GeneratedAt != wantNow {
			t.Fatalf("result = %+v", got)
		}
		if got.Timeline == nil || !reflect.DeepEqual(*got.Timeline, retainedTimeline) {
			t.Fatalf("timeline = %+v, want %+v", got.Timeline, retainedTimeline)
		}
	})

	t.Run("retained outcome without a timeline names the gap", func(t *testing.T) {
		fake := &fakeReads{
			recordErr:    notExist("io-old"),
			outcome:      issueopsviewcontract.CycleOutcome{SchemaVersion: 1, ID: "io-old", Outcome: "finished", RetainedAt: "2026-08-01T00:00:00Z"},
			outcomeFound: true,
		}
		got, err := run(fake, "io-old", "timeline")
		if err != nil {
			t.Fatal(err)
		}
		want := issueopsviewcontract.CycleTimeline{
			Limits: []string{"retained_without_timeline"},
			Events: []issueopsviewcontract.TimelineEvent{},
		}
		if got.Source != issueopsviewcontract.SourceRetained || got.Timeline == nil || !reflect.DeepEqual(*got.Timeline, want) {
			t.Fatalf("result = %+v timeline=%+v", got, got.Timeline)
		}
	})

	t.Run("no record and no retained row returns the record error", func(t *testing.T) {
		fake := &fakeReads{recordErr: notExist("io-none")}
		got, err := run(fake, "io-none", "timeline")
		if !errors.Is(err, fs.ErrNotExist) || got.OK {
			t.Fatalf("got %+v err=%v, want not-exist", got, err)
		}
		if fake.retainedCalls != 1 {
			t.Fatalf("retained reads = %d, want 1", fake.retainedCalls)
		}
	})

	t.Run("a record error that is not not-exist is returned without a retained read", func(t *testing.T) {
		boom := errors.New("record row is corrupt")
		fake := &fakeReads{recordErr: boom, outcomeFound: true}
		got, err := run(fake, "io-bad", "timeline")
		if !errors.Is(err, boom) || got.OK {
			t.Fatalf("got %+v err=%v, want %v", got, err, boom)
		}
		if fake.retainedCalls != 0 {
			t.Fatalf("retained reads = %d, want 0", fake.retainedCalls)
		}
	})

	t.Run("blank id is rejected before any read", func(t *testing.T) {
		for _, id := range []string{"", "   "} {
			fake := &fakeReads{record: record}
			if _, err := run(fake, id, "timeline"); err == nil || err.Error() != "view requires --id" {
				t.Fatalf("id %q: err = %v", id, err)
			}
			if fake.recordCalls != 0 || fake.retainedCalls != 0 {
				t.Fatalf("id %q: read before validating: %+v", id, fake)
			}
		}
	})

	t.Run("only the timeline and graph views are accepted", func(t *testing.T) {
		// graph is accepted since the graph view exists; its own behavior is
		// TestViewServiceGraph. A blank or unknown view is still rejected before
		// any read.
		for _, view := range []string{"", "bogus"} {
			fake := &fakeReads{record: record}
			if _, err := run(fake, "io-live", view); err == nil || err.Error() != "--view must be timeline or graph" {
				t.Fatalf("view %q: err = %v", view, err)
			}
			if fake.recordCalls != 0 {
				t.Fatalf("view %q: read before validating", view)
			}
		}
	})

	t.Run("a retained read error is returned", func(t *testing.T) {
		boom := errors.New("retained row is corrupt")
		fake := &fakeReads{recordErr: notExist("io-gone"), outcomeErr: boom}
		got, err := run(fake, "io-gone", "timeline")
		if !errors.Is(err, boom) || got.OK {
			t.Fatalf("got %+v err=%v, want %v", got, err, boom)
		}
	})

	t.Run("missing ports fail closed", func(t *testing.T) {
		if _, err := NewService(Ports{}).View(context.Background(), "/state", "", "io-live", "timeline"); err == nil {
			t.Fatal("a service without ports must fail")
		}
	})

	t.Run("the caller's context reaches every port", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ctxKey{}, "caller")
		live := &fakeReads{record: record}
		if _, err := NewService(live.ports(now)).View(ctx, "/state", "", "io-live", "timeline"); err != nil {
			t.Fatal(err)
		}
		gone := &fakeReads{recordErr: notExist("io-gone"), outcomeFound: true, outcome: issueopsviewcontract.CycleOutcome{ID: "io-gone"}}
		if _, err := NewService(gone.ports(now)).View(ctx, "/state", "", "io-gone", "timeline"); err != nil {
			t.Fatal(err)
		}
		contexts := append(live.contexts, gone.contexts...)
		if len(contexts) != 3 {
			t.Fatalf("ports saw %d contexts, want 3 (record, record, retained)", len(contexts))
		}
		for _, got := range contexts {
			if got.Value(ctxKey{}) != "caller" {
				t.Fatalf("a port got a context without the caller's value: %v", got)
			}
		}
	})

	t.Run("a cancelled context reaches the port, which stops the read", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		fake := &fakeReads{record: record}
		if _, err := NewService(fake.ports(now)).View(ctx, "/state", "", "io-live", "timeline"); !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if fake.recordCalls != 1 || fake.retainedCalls != 0 || len(fake.stateRoots) != 0 {
			t.Fatalf("a cancelled read must stop at the record port: %+v", fake)
		}
	})
}

// fakeGraphReads adds the two graph-only ports to fakeReads and counts their calls.
type fakeGraphReads struct {
	fakeReads
	next          issueopsnextcontract.Result
	nextErr       error
	worktree      bool
	nextCalls     int
	worktreeCalls int
	worktreeRoots []string
	nextCWDs      []string
}

func (fake *fakeGraphReads) graphPorts(now time.Time) Ports {
	ports := fake.fakeReads.ports(now)
	ports.Next = func(ctx context.Context, stateRoot, cwd, id string) (issueopsviewcontract.NextResult, error) {
		fake.nextCalls++
		fake.contexts = append(fake.contexts, ctx)
		fake.nextCWDs = append(fake.nextCWDs, cwd)
		return fake.next, fake.nextErr
	}
	ports.WorktreePresent = func(ctx context.Context, root string) bool {
		fake.worktreeCalls++
		fake.contexts = append(fake.contexts, ctx)
		fake.worktreeRoots = append(fake.worktreeRoots, root)
		return fake.worktree
	}
	return ports
}

func TestViewServiceGraph(t *testing.T) {
	now := time.Date(2026, 11, 11, 3, 4, 5, 0, time.UTC)
	liveRecord := func(execution *issueopscontract.Execution) issueopsviewcontract.Record {
		return issueopsviewcontract.Record{
			ID: "io-live", Phase: issueopscontract.PhasePlan, Execution: execution,
			ArchitectureReview: &reviewcontract.ArchitectureReview{Approved: true, ReviewedArchitectureDigest: "sha256:architecture"},
			PlanPath:           "plan.md",
		}
	}
	execution := &issueopscontract.Execution{Workspace: issueopscontract.Workspace{Root: "/wt/io-live"}}
	run := func(fake *fakeGraphReads) (issueopsviewcontract.Result, error) {
		return NewService(fake.graphPorts(now)).View(context.Background(), "/state", "/cwd", "io-live", "graph")
	}
	bindingStates := func(t *testing.T, graph *issueopsviewcontract.Graph) []string {
		t.Helper()
		if graph == nil {
			t.Fatal("result has no graph")
		}
		var states []string
		for _, edge := range graph.Edges {
			if edge.Binding != "" {
				states = append(states, edge.State)
			}
		}
		return states
	}
	allEqual := func(states []string, want string) bool {
		for _, state := range states {
			if state != want {
				return false
			}
		}
		return len(states) > 0
	}

	t.Run("a failed next marks every binding edge unknown and warns without failing the view", func(t *testing.T) {
		fake := &fakeGraphReads{fakeReads: fakeReads{record: liveRecord(nil)}, nextErr: errors.New("git probe exploded")}
		got, err := run(fake)
		if err != nil || !got.OK {
			t.Fatalf("a failed next must not fail the view: %+v err=%v", got, err)
		}
		if got.View != "graph" || got.Source != issueopsviewcontract.SourceRecord || got.ID != "io-live" || got.GeneratedAt != "2026-11-11T03:04:05Z" || got.Timeline != nil {
			t.Fatalf("result header = %+v", got)
		}
		if got.Graph == nil || got.Graph.UnknownReason != issueopsviewcontract.UnknownNextFailed {
			t.Fatalf("graph = %+v, want unknown_reason next_failed", got.Graph)
		}
		if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "git probe exploded") {
			t.Fatalf("warnings = %v, want the next error", got.Warnings)
		}
		if states := bindingStates(t, got.Graph); !allEqual(states, issueopsviewcontract.EdgeStateUnknown) {
			t.Fatalf("binding states = %v, want all unknown", states)
		}
		if fake.nextCalls != 1 || fake.nextCWDs[0] != "/cwd" {
			t.Fatalf("next calls = %d cwd=%v, want one call with the caller's cwd", fake.nextCalls, fake.nextCWDs)
		}
	})

	t.Run("a next stage keyed empty is stage_unjudgeable", func(t *testing.T) {
		fake := &fakeGraphReads{fakeReads: fakeReads{record: liveRecord(nil)}}
		got, err := run(fake)
		if err != nil {
			t.Fatal(err)
		}
		if got.Graph == nil || got.Graph.UnknownReason != issueopsviewcontract.UnknownStageUnjudgeable || got.Graph.JudgedBy.Stage != "" {
			t.Fatalf("graph = %+v, want stage_unjudgeable for the empty stage key", got.Graph)
		}
		if states := bindingStates(t, got.Graph); !allEqual(states, issueopsviewcontract.EdgeStateUnknown) {
			t.Fatalf("binding states = %v, want all unknown", states)
		}
		if len(got.Warnings) != 0 {
			t.Fatalf("warnings = %v, want none", got.Warnings)
		}
	})

	t.Run("a record without execution never asks about a worktree", func(t *testing.T) {
		fake := &fakeGraphReads{fakeReads: fakeReads{record: liveRecord(nil)}, next: issueopsnextcontract.Result{
			Stage: issueopsnextcontract.Stage{Key: "plan.review"},
		}}
		got, err := run(fake)
		if err != nil {
			t.Fatal(err)
		}
		if fake.worktreeCalls != 0 {
			t.Fatalf("WorktreePresent calls = %d, want 0 without an execution", fake.worktreeCalls)
		}
		if got.Graph == nil || got.Graph.UnknownReason != "" || got.Graph.JudgedBy.Stage != "plan.review" {
			t.Fatalf("graph = %+v, want a judged plan.review graph", got.Graph)
		}
	})

	t.Run("a missing worktree marks the worktree edges unknown and keeps the digest edge judged", func(t *testing.T) {
		record := liveRecord(execution)
		record.ImplementationReview = &issueopscontract.ImplementationReview{Verdict: "pass"}
		fake := &fakeGraphReads{fakeReads: fakeReads{record: record}, worktree: false, next: issueopsnextcontract.Result{
			Stage: issueopsnextcontract.Stage{Key: "verify"},
		}}
		got, err := run(fake)
		if err != nil {
			t.Fatal(err)
		}
		if got.Graph == nil || got.Graph.UnknownReason != issueopsviewcontract.UnknownWorktreeMissing {
			t.Fatalf("graph = %+v, want worktree_missing", got.Graph)
		}
		if fake.worktreeCalls != 1 || fake.worktreeRoots[0] != "/wt/io-live" {
			t.Fatalf("WorktreePresent calls = %d roots=%v, want one call on the execution root", fake.worktreeCalls, fake.worktreeRoots)
		}
		if states := bindingStates(t, got.Graph); !reflect.DeepEqual(states, []string{issueopsviewcontract.EdgeStateNotReported, issueopsviewcontract.EdgeStateUnknown}) {
			t.Fatalf("binding states = %v, want [not_reported unknown]: the digest edge, then the implementation review edge", states)
		}
	})

	t.Run("the graph view passes the caller's context to next and the worktree check", func(t *testing.T) {
		fake := &fakeGraphReads{fakeReads: fakeReads{record: liveRecord(execution)}, worktree: true, next: issueopsnextcontract.Result{
			Stage: issueopsnextcontract.Stage{Key: "verify"},
		}}
		ctx := context.WithValue(context.Background(), ctxKey{}, "caller")
		if _, err := NewService(fake.graphPorts(now)).View(ctx, "/state", "/cwd", "io-live", "graph"); err != nil {
			t.Fatal(err)
		}
		if len(fake.contexts) != 3 {
			t.Fatalf("ports saw %d contexts, want 3 (record, next, worktree)", len(fake.contexts))
		}
		for _, got := range fake.contexts {
			if got.Value(ctxKey{}) != "caller" {
				t.Fatalf("a port got a context without the caller's value: %v", got)
			}
		}
	})

	t.Run("a judged stage with the worktree present reports stale from next's missing", func(t *testing.T) {
		record := liveRecord(execution)
		record.ImplementationReview = &issueopscontract.ImplementationReview{Verdict: "pass"}
		fake := &fakeGraphReads{fakeReads: fakeReads{record: record}, worktree: true, next: issueopsnextcontract.Result{
			Stage: issueopsnextcontract.Stage{Key: "verify"}, Missing: []string{"implementation_review_stale"},
		}}
		got, err := run(fake)
		if err != nil {
			t.Fatal(err)
		}
		var stale []string
		for _, edge := range got.Graph.Edges {
			if edge.State == issueopsviewcontract.EdgeStateStale {
				stale = append(stale, edge.Gate)
			}
		}
		if !reflect.DeepEqual(stale, []string{"implementation_review_stale"}) {
			t.Fatalf("stale gates = %v, want only implementation_review_stale; graph=%+v", stale, got.Graph)
		}
		if got.Graph.UnknownReason != "" || !reflect.DeepEqual(got.Graph.JudgedBy.StaleKeys, []string{"implementation_review_stale"}) {
			t.Fatalf("judged_by = %+v reason=%q", got.Graph.JudgedBy, got.Graph.UnknownReason)
		}
	})

	t.Run("no record and a retained row refuses with the retired note and never calls next", func(t *testing.T) {
		fake := &fakeGraphReads{fakeReads: fakeReads{
			recordErr:    notExist("io-gone"),
			outcome:      issueopsviewcontract.CycleOutcome{SchemaVersion: 1, ID: "io-gone", Outcome: "abandoned", RetainedAt: "2026-09-09T00:00:00Z"},
			outcomeFound: true,
		}}
		got, err := NewService(fake.graphPorts(now)).View(context.Background(), "/state", "/cwd", "io-gone", "graph")
		if err == nil || got.OK || !strings.Contains(err.Error(), "cleanup removed it") || !strings.HasPrefix(err.Error(), "graph needs a live record; ") {
			t.Fatalf("got %+v err=%v, want the graph-needs-a-live-record error with the retired note", got, err)
		}
		if fake.nextCalls != 0 || fake.worktreeCalls != 0 {
			t.Fatalf("next calls = %d worktree calls = %d, want 0 and 0", fake.nextCalls, fake.worktreeCalls)
		}
	})

	t.Run("no record and no retained row returns the not-exist error", func(t *testing.T) {
		fake := &fakeGraphReads{fakeReads: fakeReads{recordErr: notExist("io-none")}}
		got, err := NewService(fake.graphPorts(now)).View(context.Background(), "/state", "/cwd", "io-none", "graph")
		if !errors.Is(err, fs.ErrNotExist) || got.OK {
			t.Fatalf("got %+v err=%v, want not-exist", got, err)
		}
		if fake.retainedCalls != 1 || fake.nextCalls != 0 {
			t.Fatalf("retained reads = %d next calls = %d, want 1 and 0", fake.retainedCalls, fake.nextCalls)
		}
	})

	t.Run("the graph view fails closed without its ports", func(t *testing.T) {
		fake := &fakeReads{record: liveRecord(nil)}
		if _, err := NewService(fake.ports(now)).View(context.Background(), "/state", "/cwd", "io-live", "graph"); err == nil {
			t.Fatal("a graph view without Next and WorktreePresent must fail")
		}
		if fake.recordCalls != 0 {
			t.Fatal("the missing-port check must come before any read")
		}
	})
}
