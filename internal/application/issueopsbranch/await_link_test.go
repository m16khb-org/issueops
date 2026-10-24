package issueopsbranch

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	linkedbranch "issueops/internal/domain/issueopslinkedbranch"
)

const awaitSealedBase = "aaaa111"

func awaitRecord() issueopscontract.IssueOpsRecord {
	return issueopscontract.IssueOpsRecord{
		ID:   "io-await",
		Repo: "example/repo",
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{
			Provider: "github",
			IssueURL: "https://github.com/example/repo/issues/7",
			Branch:   "feature/await",
			BaseSHA:  awaitSealedBase,
		},
	}
}

// awaitHarness gives a virtual clock that only Sleep advances.
type awaitHarness struct {
	now      time.Time
	sleeps   int
	observed []linkedbranch.Observation
	script   []func() (linkedbranch.Observation, error)
	remote   func() (string, error)
	sleepErr error
	awaiter  LinkAwaiter
}

func newAwaitHarness(record issueopscontract.IssueOpsRecord) *awaitHarness {
	h := &awaitHarness{now: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)}
	h.remote = func() (string, error) { return awaitSealedBase, nil }
	h.awaiter = LinkAwaiter{
		Load:      func(string) (issueopscontract.IssueOpsRecord, error) { return record, nil },
		RemoteRef: func(context.Context, string, string) (string, error) { return h.remote() },
		ObserveLinkedBranches: func(_ context.Context, _ string) (linkedbranch.Observation, error) {
			i := len(h.observed)
			h.observed = append(h.observed, linkedbranch.Observation{})
			if i >= len(h.script) {
				i = len(h.script) - 1
			}
			return h.script[i]()
		},
		Sleep: func(_ context.Context, d time.Duration) error {
			h.sleeps++
			if h.sleepErr != nil {
				return h.sleepErr
			}
			h.now = h.now.Add(d)
			return nil
		},
		Now: func() time.Time { return h.now },
	}
	return h
}

func absent() (linkedbranch.Observation, error) { return linkedbranch.Observation{}, nil }

func healthy() (linkedbranch.Observation, error) {
	return linkedbranch.Observation{
		TotalCount: 1,
		Nodes:      []linkedbranch.Node{{ID: "LB_1", RefName: "feature/await", RefOID: awaitSealedBase}},
	}, nil
}

func TestAwaitLinkLoadErrorReturnsNotOKWithRequestedID(t *testing.T) {
	boom := errors.New("load failed")
	awaiter := LinkAwaiter{Load: func(string) (issueopscontract.IssueOpsRecord, error) {
		return issueopscontract.IssueOpsRecord{}, boom
	}}
	res, err := awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-x"})
	if !errors.Is(err, boom) || res.OK || res.ID != "io-x" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestAwaitLinkRejectsMalformedTimeoutBeforeObserving(t *testing.T) {
	h := newAwaitHarness(awaitRecord())
	h.script = []func() (linkedbranch.Observation, error){healthy}
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await", Timeout: "soon"})
	if err == nil || !strings.Contains(err.Error(), "not a duration") || res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(h.observed) != 0 {
		t.Fatalf("observed %d times despite invalid timeout", len(h.observed))
	}
}

func TestAwaitLinkReportsMissingGatesWithoutObserving(t *testing.T) {
	record := awaitRecord()
	record.BranchPrepare.Provider = "gitlab"
	record.BranchPrepare.BaseSHA = ""
	h := newAwaitHarness(record)
	h.script = []func() (linkedbranch.Observation, error){healthy}
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await"})
	if err == nil || res.OK {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	want := []string{"branch_await_link_is_github_only", "sealed_base_missing"}
	if strings.Join(res.Missing, ",") != strings.Join(want, ",") {
		t.Fatalf("Missing=%v want %v", res.Missing, want)
	}
	for _, m := range want {
		if !strings.Contains(err.Error(), m) {
			t.Fatalf("error %q lacks %q", err, m)
		}
	}
	if len(h.observed) != 0 {
		t.Fatal("observer ran although gates were missing")
	}
}

func TestAwaitLinkWithoutObserverFailsClosed(t *testing.T) {
	awaiter := LinkAwaiter{Load: func(string) (issueopscontract.IssueOpsRecord, error) { return awaitRecord(), nil }}
	res, err := awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await"})
	if err == nil || res.OK || len(res.Missing) != 1 || res.Missing[0] != "linked_branch_observation_unavailable" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestAwaitLinkAlreadyVerifiedSucceedsWithoutObserving(t *testing.T) {
	record := awaitRecord()
	record.BranchPrepare.LinkVerified = true
	h := newAwaitHarness(record)
	h.script = []func() (linkedbranch.Observation, error){absent}
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await"})
	if err != nil || !res.OK || !res.Linked || !res.AlreadyVerified || res.Attempts != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(h.observed) != 0 || h.sleeps != 0 {
		t.Fatalf("observed=%d sleeps=%d", len(h.observed), h.sleeps)
	}
	if res.Branch != "feature/await" || res.SealedBase != awaitSealedBase {
		t.Fatalf("sealed identity not echoed: %+v", res)
	}
}

func TestAwaitLinkReturnsHealthyLinkAfterWaitingForCoordinator(t *testing.T) {
	h := newAwaitHarness(awaitRecord())
	h.script = []func() (linkedbranch.Observation, error){absent, absent, healthy}
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await", Timeout: "5m"})
	if err != nil || !res.OK || !res.Linked {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if res.Attempts != 3 || h.sleeps != 2 {
		t.Fatalf("attempts=%d sleeps=%d", res.Attempts, h.sleeps)
	}
	if res.LinkedBranchID != "LB_1" || res.ObservedOID != awaitSealedBase || res.State != string(linkedbranch.StateHealthy) {
		t.Fatalf("observation not reported: %+v", res)
	}
	if res.TimeoutSeconds != 300 {
		t.Fatalf("TimeoutSeconds=%d", res.TimeoutSeconds)
	}
	// The next step is the sealed prepare re-recorded with --link-verified, as
	// one runnable command: agents retyped the values and got them wrong.
	for _, want := range []string{"issueops branch prepare --id io-await", "--base-sha '" + awaitSealedBase + "'", "--link-verified $RECORD_ACTOR_FLAGS"} {
		if !strings.Contains(res.NextCommand, want) {
			t.Fatalf("NextCommand=%q lacks %q", res.NextCommand, want)
		}
	}
}

func TestAwaitLinkStopsImmediatelyOnMismatchedLink(t *testing.T) {
	h := newAwaitHarness(awaitRecord())
	h.script = []func() (linkedbranch.Observation, error){func() (linkedbranch.Observation, error) {
		return linkedbranch.Observation{TotalCount: 1, Nodes: []linkedbranch.Node{{ID: "LB_2", RefName: "feature/await", RefOID: "bbbb222"}}}, nil
	}}
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await"})
	if err == nil || !strings.Contains(err.Error(), "does not match the sealed identity") || res.OK || res.Linked {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if res.State != string(linkedbranch.StateMismatched) || res.Attempts != 1 || h.sleeps != 0 || res.TimedOut {
		t.Fatalf("mismatch must not be retried: %+v sleeps=%d", res, h.sleeps)
	}
}

func TestAwaitLinkTimesOutWhenLinkNeverAppears(t *testing.T) {
	h := newAwaitHarness(awaitRecord())
	h.script = []func() (linkedbranch.Observation, error){absent}
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await", Timeout: "1m"})
	if err == nil || !res.TimedOut || res.OK || res.Linked {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	for _, want := range []string{"feature/await", "did not appear within 1m0s", awaitSealedBase, string(linkedbranch.StateAbsent)} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
	// 1m / 15s interval: observations at t=0,15,30,45,60 → the 5th hits the deadline.
	wantAttempts := int(time.Minute/domain.AwaitBranchLinkInterval) + 1
	if res.Attempts != wantAttempts || h.sleeps != wantAttempts-1 {
		t.Fatalf("attempts=%d sleeps=%d want attempts=%d", res.Attempts, h.sleeps, wantAttempts)
	}
}

func TestAwaitLinkSurfacesSleepErrorAndStops(t *testing.T) {
	h := newAwaitHarness(awaitRecord())
	h.script = []func() (linkedbranch.Observation, error){absent}
	h.sleepErr = context.Canceled
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await"})
	if !errors.Is(err, context.Canceled) || res.OK || res.TimedOut || res.Attempts != 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestAwaitLinkTreatsReadbackFailureAsAmbiguousAndRetries(t *testing.T) {
	h := newAwaitHarness(awaitRecord())
	h.script = []func() (linkedbranch.Observation, error){
		func() (linkedbranch.Observation, error) { return linkedbranch.Observation{}, errors.New("graphql 502") },
		healthy,
	}
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await"})
	if err != nil || !res.Linked || res.Attempts != 2 || h.sleeps != 1 {
		t.Fatalf("res=%+v err=%v sleeps=%d", res, err, h.sleeps)
	}
}

// A provider that says the repository or issue does not exist will not change
// its answer; await-link stops after one readback instead of polling to the
// timeout. A transient failure (above) still retries.
func TestAwaitLinkStopsWhenTheProviderCannotFindTheIssue(t *testing.T) {
	h := newAwaitHarness(awaitRecord())
	h.script = []func() (linkedbranch.Observation, error){
		func() (linkedbranch.Observation, error) {
			return linkedbranch.Observation{}, errors.New("gh: Could not resolve to a Repository with the name 'example/hello'.")
		},
	}
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await"})
	if err == nil || !strings.Contains(err.Error(), "cannot find the issue") || res.OK || res.Linked || res.TimedOut || res.Attempts != 1 || h.sleeps != 0 {
		t.Fatalf("res=%+v err=%v sleeps=%d", res, err, h.sleeps)
	}
}

func TestAwaitLinkReportsReadbackFailureReasonOnTimeout(t *testing.T) {
	h := newAwaitHarness(awaitRecord())
	h.script = []func() (linkedbranch.Observation, error){
		func() (linkedbranch.Observation, error) { return linkedbranch.Observation{}, errors.New("graphql 502") },
	}
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await", Timeout: "1s"})
	if err == nil || !res.TimedOut {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if res.State != string(linkedbranch.StateAmbiguous) || !strings.Contains(res.StateReason, "linked branch readback failed: graphql 502") {
		t.Fatalf("State=%q reason=%q", res.State, res.StateReason)
	}
}

func TestAwaitLinkTreatsRemoteRefErrorAsAmbiguousAndNeverLinks(t *testing.T) {
	h := newAwaitHarness(awaitRecord())
	h.script = []func() (linkedbranch.Observation, error){healthy}
	h.remote = func() (string, error) { return "", errors.New("ls-remote denied") }
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await", Timeout: "1s"})
	if err == nil || res.Linked || !res.TimedOut {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if res.State != string(linkedbranch.StateAmbiguous) || res.StateReason != "ls-remote denied" {
		t.Fatalf("State=%q reason=%q", res.State, res.StateReason)
	}
}

func TestAwaitLinkRejectsLinkWhoseRemoteBranchDiffersFromSeal(t *testing.T) {
	h := newAwaitHarness(awaitRecord())
	h.script = []func() (linkedbranch.Observation, error){healthy}
	h.remote = func() (string, error) { return "cccc333", nil }
	res, err := h.awaiter.Await(t.Context(), issueopscontract.AwaitBranchLinkRequest{ID: "io-await"})
	if err == nil || res.Linked || res.State != string(linkedbranch.StateMismatched) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if !strings.Contains(res.StateReason, "cccc333") {
		t.Fatalf("reason %q should name the remote OID", res.StateReason)
	}
}
