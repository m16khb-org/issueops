package issueopscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	commandparsecontract "issueops/internal/contract/commandparse"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
	provenanceport "issueops/internal/port/issueopsprovenance"
)

// stubRuntimeCommand wires only the supplied runtime hooks; any other hook is nil
// and would panic, which keeps these tests honest about what each command touches.
func stubRuntimeCommand(runtime IssueOpsCLIDeps) command {
	runtime.IssueOpsStateRoot = func() string { return "/stub-root" }
	runtime.ObserveNativeProcessAncestry = func(int) ([]issueopscontract.NativeProcessReceipt, error) { return nil, nil }
	return command{Runtime: runtime}
}

func runStub(t *testing.T, cli command, args func(command) error) (string, error) {
	t.Helper()
	return captureStdoutAndErrorForIssueOps(t, func() error { return args(cli) })
}

func TestParseExpectedRoutingAcceptsPairingsAndRejectsMalformedSpecs(t *testing.T) {
	got, err := parseExpectedRouting(" plan:database-design , ,implement:algorithm-optimization ")
	if err != nil {
		t.Fatal(err)
	}
	want := []issueopsroutingcontract.Expected{{Phase: "plan", Skill: "database-design"}, {Phase: "implement", Skill: "algorithm-optimization"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("pairings = %#v, want %#v", got, want)
	}
	tests := []struct{ spec, wantErr string }{
		{"", "--expect is required"},
		{"   ", "--expect is required"},
		{"plan", `invalid --expect pairing "plan"`},
		{"plan:", `invalid --expect pairing "plan:"`},
		{":skill", `invalid --expect pairing ":skill"`},
		{"plan:ok,bad", `invalid --expect pairing "bad"`},
		{" , ", "--expect produced no pairings"},
	}
	for _, test := range tests {
		if _, err := parseExpectedRouting(test.spec); err == nil || !strings.Contains(err.Error(), test.wantErr) {
			t.Errorf("parseExpectedRouting(%q) error = %v, want substring %q", test.spec, err, test.wantErr)
		}
	}
}

func TestRoutingScoreReportsMissingPairingsAndRoutesErrors(t *testing.T) {
	var gotExpected []issueopsroutingcontract.Expected
	var result issueopsroutingcontract.Result
	var fail error
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		ScoreLiveRoutingFidelity: func(_, _ string, expected []issueopsroutingcontract.Expected) (issueopsroutingcontract.Result, int, error) {
			gotExpected = expected
			return result, 3, fail
		},
	})
	result = issueopsroutingcontract.Result{OK: false, Missing: []issueopsroutingcontract.Expected{{Phase: "plan", Skill: "database-design"}}}
	out, err := runStub(t, cli, func(c command) error {
		return c.runIssueOpsRoutingScore([]string{"--id", "c1", "--expect", "plan:database-design"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(gotExpected) != 1 || gotExpected[0].Skill != "database-design" {
		t.Fatalf("expected pairings were not parsed from --expect: %#v", gotExpected)
	}
	if !strings.Contains(out, "routing fidelity: ok=false (observed 3 pairings)") || !strings.Contains(out, "- missing: database-design@plan") {
		t.Fatalf("text output wrong:\n%s", out)
	}

	// A malformed --expect fails before the runtime is consulted.
	gotExpected = nil
	if _, err := runStub(t, cli, func(c command) error {
		return c.runIssueOpsRoutingScore([]string{"--id", "c1", "--expect", "nonsense"})
	}); err == nil || gotExpected != nil {
		t.Fatalf("malformed --expect must fail before scoring: err=%v expected=%#v", err, gotExpected)
	}

	fail = errors.New("no routing recorded")
	out, err = runStub(t, cli, func(c command) error {
		return c.runIssueOpsRoutingScore([]string{"--id", "c1", "--expect", "plan:x", "--json"})
	})
	if !errors.Is(err, fail) {
		t.Fatalf("--json failure must still return the scoring error, got %v", err)
	}
	assertIssueOpsStructuredFailure(t, out, "no routing recorded")
	if _, err := runStub(t, cli, func(c command) error {
		return c.runIssueOpsRoutingScore([]string{"--id", "c1", "--expect", "plan:x"})
	}); !errors.Is(err, fail) {
		t.Fatalf("text mode must return the scoring error, got %v", err)
	}
}

func TestBranchAwaitLinkFailsClosedAndReportsOutcome(t *testing.T) {
	cli := stubRuntimeCommand(IssueOpsCLIDeps{})
	if _, err := runStub(t, cli, func(c command) error { return c.runIssueOpsBranchAwaitLink([]string{"--id", "c1"}) }); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing await hook must fail closed, got %v", err)
	}

	var gotReq issueopscontract.AwaitBranchLinkRequest
	var result issueopscontract.AwaitBranchLinkResult
	var fail error
	cli.Runtime.AwaitIssueOpsBranchLink = func(_ context.Context, _ string, req issueopscontract.AwaitBranchLinkRequest) (issueopscontract.AwaitBranchLinkResult, error) {
		gotReq = req
		return result, fail
	}
	result = issueopscontract.AwaitBranchLinkResult{Linked: true, Branch: "12-fix", ObservedOID: "deadbeef", Attempts: 4, NextCommand: "issueops execution prepare"}
	out, err := runStub(t, cli, func(c command) error {
		return c.runIssueOpsBranchAwaitLink([]string{"--id", "c1", "--timeout", "5m"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotReq.ID != "c1" || gotReq.Timeout != "5m" {
		t.Fatalf("request not forwarded: %#v", gotReq)
	}
	if !strings.Contains(out, "branch link observed: branch=12-fix oid=deadbeef attempts=4") || !strings.Contains(out, "next: issueops execution prepare") {
		t.Fatalf("observed output wrong:\n%s", out)
	}

	result = issueopscontract.AwaitBranchLinkResult{AlreadyVerified: true, Branch: "12-fix"}
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsBranchAwaitLink([]string{"--id", "c1"}) })
	if err != nil || strings.TrimSpace(out) != "branch link already recorded: branch=12-fix" {
		t.Fatalf("already-verified output: err=%v out=%q", err, out)
	}

	// A timeout still prints the structured observation in JSON mode and returns the error.
	fail = errors.New("await timed out")
	result = issueopscontract.AwaitBranchLinkResult{ID: "c1", TimedOut: true}
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsBranchAwaitLink([]string{"--id", "c1", "--json"}) })
	if !errors.Is(err, fail) {
		t.Fatalf("timeout must return the error, got %v", err)
	}
	if !strings.Contains(out, `"timed_out": true`) {
		t.Fatalf("JSON mode must print the observation on failure:\n%s", out)
	}
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsBranchAwaitLink([]string{"--id", "c1"}) })
	if !errors.Is(err, fail) || out != "" {
		t.Fatalf("text mode failure prints nothing and returns the error: err=%v out=%q", err, out)
	}
}

func TestPruneValidatesMaxAgeAndDistinguishesPreviewPrunedAndIncomplete(t *testing.T) {
	var gotAge time.Duration
	var gotConfirm bool
	var result issueopsretentioncontract.Result
	var fail error
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		PruneIssueOps: func(_ string, age time.Duration, confirm bool) (issueopsretentioncontract.Result, error) {
			gotAge, gotConfirm = age, confirm
			return result, fail
		},
	})
	if _, err := runStub(t, cli, func(c command) error { return c.runIssueOpsPrune([]string{"--max-age", "30days"}) }); err == nil || !strings.Contains(err.Error(), "invalid --max-age") {
		t.Fatalf("expected invalid max-age error, got %v", err)
	}
	result = issueopsretentioncontract.Result{Cutoff: "T0", Pruned: []string{"a", "b"}, Kept: []string{"c"}, Unreadable: []string{"x"}}
	out, err := runStub(t, cli, func(c command) error { return c.runIssueOpsPrune([]string{"--max-age", "48h"}) })
	if err != nil {
		t.Fatal(err)
	}
	if gotAge != 48*time.Hour || gotConfirm {
		t.Fatalf("flags not forwarded: age=%v confirm=%v", gotAge, gotConfirm)
	}
	if strings.TrimSpace(out) != "preview: 2 done cycles selected, 1 kept, 1 unreadable (cutoff T0)" {
		t.Fatalf("preview output: %q", out)
	}
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsPrune([]string{"--confirm"}) })
	if err != nil {
		t.Fatal(err)
	}
	if gotAge != 720*time.Hour || !gotConfirm || !strings.HasPrefix(out, "pruned: 2 done cycles selected") {
		t.Fatalf("confirm run: age=%v confirm=%v out=%q", gotAge, gotConfirm, out)
	}

	fail = errors.New("delete failed")
	result = issueopsretentioncontract.Result{Cutoff: "T1", Pruned: []string{"a"}, Kept: []string{"b", "c"}, ReadErrors: 2, DeleteErrors: 1}
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsPrune([]string{"--confirm"}) })
	if !errors.Is(err, fail) {
		t.Fatalf("expected delete failure, got %v", err)
	}
	if strings.TrimSpace(out) != "incomplete: 1 done cycles pruned, 2 kept, 2 unreadable, 1 delete failures (cutoff T1)" {
		t.Fatalf("incomplete output: %q", out)
	}
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsPrune([]string{"--confirm", "--json"}) })
	if !errors.Is(err, fail) || !strings.Contains(out, `"delete_errors": 1`) {
		t.Fatalf("JSON failure must print the partial result and return the error: err=%v out=%s", err, out)
	}
}

func TestNextTextRendersStageLeaseReviewAndExits(t *testing.T) {
	live := true
	result := issueopsnextcontract.Result{
		Stage:       issueopsnextcontract.Stage{Key: "implement", Index: 5},
		Selected:    &issueopsnextcontract.Entry{ID: "c7", Phase: "implement"},
		Candidates:  []issueopsnextcontract.Entry{{ID: "c8", Phase: "plan", Branch: "8-b"}},
		Lease:       issueopsnextcontract.Lease{Status: string(issueopsnextcontract.LeaseStatusActive), Generation: 3, HolderLive: &live},
		Missing:     []string{"plan", "spec"},
		Review:      issueopsnextcontract.Review{Model: "gpt", Tier: "contract", Lenses: []string{"a", "b"}, Frontend: true},
		NextCommand: "issueops execution claim",
		Exits:       issueopsnextcontract.Exits{AbandonCommand: "issueops abandon"},
		Warnings:    []string{"stale base"},
	}
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		IssueOpsNext: func(_, cwd, id string) (issueopsnextcontract.Result, error) {
			if cwd != "/somewhere" || id != "c7" {
				t.Errorf("selection flags not forwarded: cwd=%q id=%q", cwd, id)
			}
			return result, nil
		},
	})
	out, err := runStub(t, cli, func(c command) error { return c.runIssueOpsNext([]string{"--cwd", " /somewhere ", "--id", "c7"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"stage 5/10 implement  cycle c7  phase implement  lease active(gen 3, other)",
		"missing: plan, spec",
		"candidate: c8 phase=plan branch=8-b",
		"review: model=gpt effort=- tier=contract lenses=a,b",
		"review signal: frontend",
		"next: issueops execution claim",
		"exits: pause=- abandon=issueops abandon takeover=-",
		"warning: stale base",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	// Without a selection or lease the placeholders appear, and an empty review stays silent.
	result = issueopsnextcontract.Result{Stage: issueopsnextcontract.Stage{Key: "none", Index: 0}}
	out, _ = runStub(t, cli, func(c command) error { return c.runIssueOpsNext([]string{"--cwd", "/somewhere", "--id", "c7"}) })
	if strings.TrimSpace(out) != "stage 0/10 none  cycle -  phase -  lease none" {
		t.Fatalf("empty result output: %q", out)
	}
}

func TestNextLeaseHolderLabels(t *testing.T) {
	tests := []struct {
		name  string
		lease issueopsnextcontract.Lease
		want  string
	}{
		{"self", issueopsnextcontract.Lease{Status: string(issueopsnextcontract.LeaseStatusActive), Generation: 2, HolderIsSelf: true}, "active(gen 2, self)"},
		{"other while revoking", issueopsnextcontract.Lease{Status: string(issueopsnextcontract.LeaseStatusRevoking), Generation: 4}, "revoking(gen 4, other)"},
		{"released has no holder", issueopsnextcontract.Lease{Status: "released", Generation: 1}, "released(gen 1, none)"},
		{"no lease", issueopsnextcontract.Lease{}, "none"},
	}
	for _, test := range tests {
		if got := issueOpsNextLease(issueopsnextcontract.Result{Lease: test.lease}); got != test.want {
			t.Errorf("%s: lease label = %q, want %q", test.name, got, test.want)
		}
	}
	if got := issueOpsNextPhase(issueopsnextcontract.Result{Selected: &issueopsnextcontract.Entry{ID: "c"}}); got != "-" {
		t.Errorf("selected entry without phase must render '-', got %q", got)
	}
}

func TestNextRunReportsRuntimeFailure(t *testing.T) {
	fail := errors.New("classify failed")
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		IssueOpsNext: func(_, _, _ string) (issueopsnextcontract.Result, error) { return issueopsnextcontract.Result{}, fail },
	})
	out, err := runStub(t, cli, func(c command) error { return c.runIssueOpsNext([]string{"--cwd", "/x"}) })
	if !errors.Is(err, fail) || out != "" {
		t.Fatalf("runtime failure must surface without output: err=%v out=%q", err, out)
	}
}

func TestListRendersFlagsForEveryAttention(t *testing.T) {
	var gotRepo string
	result := issueopsinventorycontract.ListResult{
		GeneratedAt: "T0", ScannedRecords: 5, ReadErrors: 1,
		Entries: []issueopsinventorycontract.ListEntry{
			{ID: "a", Branch: "1-a", Phase: "plan", LeaseStatus: "active"},
			{ID: "b", Branch: "2-b", Phase: "pr", LeaseStatus: "released", Claimable: true, CleanupCandidate: true, CompletionUnreflected: true,
				PendingKind: "pr", FailureCode: "boom", CleanupFailureStep: "worktree", IssueCreateStatus: "pending"},
		},
	}
	var fail error
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		ListIssueOpsCycles: func(_, repo string) (issueopsinventorycontract.ListResult, error) {
			gotRepo = repo
			return result, fail
		},
	})
	out, err := runStub(t, cli, func(c command) error { return c.runIssueOpsList([]string{"--repo", "/r"}) })
	if err != nil {
		t.Fatal(err)
	}
	if gotRepo != "/r" {
		t.Fatalf("repo filter not forwarded: %q", gotRepo)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 || lines[0] != "cycles: 2 (scanned 5 records, unreadable 1 at T0)" {
		t.Fatalf("header wrong:\n%s", out)
	}
	if lines[1] != "- a 1-a phase=plan lease=active" {
		t.Errorf("plain entry must carry no flags: %q", lines[1])
	}
	wantB := "- b 2-b phase=pr lease=released [claimable] [cleanup] [unreflected] [pending:pr] [failed:boom] [cleanup-failed:worktree] [issue-create:pending]"
	if lines[2] != wantB {
		t.Errorf("flagged entry = %q, want %q", lines[2], wantB)
	}
	fail = errors.New("scan failed")
	if _, err := runStub(t, cli, func(c command) error { return c.runIssueOpsList(nil) }); !errors.Is(err, fail) {
		t.Fatalf("list failure must surface, got %v", err)
	}
}

func TestPhaseRejectsDoneAndReportsTrackedMaterials(t *testing.T) {
	var materials issueopscontract.IssueOpsTrackedMaterials
	var fail error
	called := false
	cli := stubRuntimeCommand(IssueOpsCLIDeps{})
	cli.Gates = LoopGateDeps{AdvancePhaseReport: func(_, id, to string, _ issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, issueopscontract.IssueOpsTrackedMaterials, error) {
		called = true
		return issueopscontract.IssueOpsRecord{ID: id, Phase: issueopscontract.IssueOpsPhase(to), Repo: "/r"}, materials, fail
	}}
	out, err := runStub(t, cli, func(c command) error { return c.runIssueOpsPhase([]string{"--id", "c1", "--to", " done ", "--json"}) })
	if err == nil {
		t.Fatal("done rejection must return an error even in JSON mode")
	}
	assertIssueOpsStructuredFailure(t, out, "done is entered atomically by issueops execution complete")
	if called {
		t.Fatal("done must be rejected before any phase advance")
	}
	if _, err := runStub(t, cli, func(c command) error { return c.runIssueOpsPhase([]string{"--id", "c1", "--to", "done"}) }); err == nil {
		t.Fatal("text mode must return the done rejection")
	}

	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsPhase([]string{"--id", "c1", "--to", "plan"}) })
	if err != nil || strings.TrimSpace(out) != "c1 plan /r" {
		t.Fatalf("plain advance output: err=%v out=%q", err, out)
	}
	materials = issueopscontract.IssueOpsTrackedMaterials{Written: []string{"docs/plan.md"}, Warnings: []string{"plan is thin"}}
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsPhase([]string{"--id", "c1", "--to", "plan"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"c1 plan /r", "tracked copy: docs/plan.md (commit it with the change)", "warning: plan is thin"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsPhase([]string{"--id", "c1", "--to", "plan", "--json"}) })
	if err != nil || !strings.Contains(out, `"tracked_materials"`) || !strings.Contains(out, "docs/plan.md") {
		t.Fatalf("JSON must include tracked materials: err=%v out=%s", err, out)
	}
	fail = errors.New("gate closed")
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsPhase([]string{"--id", "c1", "--to", "plan", "--json"}) })
	if !errors.Is(err, fail) {
		t.Fatalf("gate failure must be returned, got %v", err)
	}
	assertIssueOpsStructuredFailure(t, out, "gate closed")
}

func TestPRReadinessStrictUsesGateAndReportsMissing(t *testing.T) {
	lax := issueopscontract.IssueOpsReadiness{Ready: true}
	strict := issueopscontract.IssueOpsReadiness{Ready: false, Missing: []string{"upstream sync", "plan path"}}
	var readErr error
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		ReadIssueOps: func(_, id string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{ID: id}, readErr
		},
		IssueOpsPRReadiness: func(issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness { return lax },
	})
	cli.Gates = LoopGateDeps{StrictPRReadinessWithState: func(string, issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness { return strict }}
	out, err := runStub(t, cli, func(c command) error { return c.runIssueOpsPRReadiness([]string{"--id", "c1"}) })
	if err != nil || strings.TrimSpace(out) != "ready: true" {
		t.Fatalf("lax readiness: err=%v out=%q", err, out)
	}
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsPRReadiness([]string{"--id", "c1", "--strict"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ready: false") || !strings.Contains(out, "- missing: upstream sync") || !strings.Contains(out, "- missing: plan path") {
		t.Fatalf("strict readiness must use the strict gate result:\n%s", out)
	}
	readErr = errors.New("record unreadable")
	out, err = runStub(t, cli, func(c command) error { return c.runIssueOpsPRReadiness([]string{"--id", "c1", "--json"}) })
	if !errors.Is(err, readErr) {
		t.Fatalf("read failure must be returned, got %v", err)
	}
	assertIssueOpsStructuredFailure(t, out, "record unreadable")
	if _, err := runStub(t, cli, func(c command) error { return c.runIssueOpsPRReadiness([]string{"--id", "c1"}) }); !errors.Is(err, readErr) {
		t.Fatalf("text mode must return the read error, got %v", err)
	}
}

func TestArtifactStageValidatesInputAndReportsStagedNames(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(file, []byte("plan body"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stagedContent []byte
	var stageErr, listErr error
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		StageIssueOpsArtifact: func(_, id, name string, content []byte) (issueopsartifactcontract.Record, error) {
			stagedContent = content
			return issueopsartifactcontract.Record{ID: id}, stageErr
		},
		StagedIssueOpsArtifactNames: func(string, string) ([]string, error) { return []string{"plan", "spec"}, listErr },
		UnstageIssueOpsArtifact: func(_, id, name string) (issueopsartifactcontract.Record, error) {
			return issueopsartifactcontract.Record{ID: id, Phase: "plan", Repo: "/r"}, nil
		},
	})
	artifact := func(args ...string) func(command) error {
		return func(c command) error { return c.runIssueOpsArtifact(args) }
	}
	if _, err := runStub(t, cli, artifact("stage", "--id", "c1", "--name", "plan")); err == nil || !strings.Contains(err.Error(), "requires --file") {
		t.Fatalf("missing --file must be rejected, got %v", err)
	}
	if _, err := runStub(t, cli, artifact("stage", "--id", "c1", "--name", "plan", "--file", filepath.Join(dir, "absent.md"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unreadable file must surface, got %v", err)
	}
	out, err := runStub(t, cli, artifact("stage", "--id", "c1", "--name", "plan", "--file", file))
	if err != nil || strings.TrimSpace(out) != "staged artifacts for c1: plan, spec" || string(stagedContent) != "plan body" {
		t.Fatalf("stage text: err=%v out=%q content=%q", err, out, stagedContent)
	}
	out, err = runStub(t, cli, artifact("stage", "--id", "c1", "--name", "plan", "--file", file, "--json"))
	if err != nil || !strings.Contains(out, `"staged"`) || !strings.Contains(out, `"spec"`) {
		t.Fatalf("stage json: err=%v out=%s", err, out)
	}
	listErr = errors.New("list failed")
	if _, err := runStub(t, cli, artifact("stage", "--id", "c1", "--name", "plan", "--file", file)); !errors.Is(err, listErr) {
		t.Fatalf("listing failure must surface, got %v", err)
	}
	stageErr = errors.New("name not allowed")
	out, _ = runStub(t, cli, artifact("stage", "--id", "c1", "--name", "bogus", "--file", file, "--json"))
	assertIssueOpsStructuredFailure(t, out, "name not allowed")
	if _, err := runStub(t, cli, artifact("stage", "--id", "c1", "--name", "bogus", "--file", file)); !errors.Is(err, stageErr) {
		t.Fatalf("stage failure must surface, got %v", err)
	}

	out, err = runStub(t, cli, artifact("unstage", "--id", "c1", "--name", "plan"))
	if err != nil || strings.TrimSpace(out) != "c1 plan /r" {
		t.Fatalf("unstage output: err=%v out=%q", err, out)
	}
	out, err = runStub(t, cli, artifact("--help"))
	if err != nil || !strings.Contains(out, "issueops artifact stage --id ID") {
		t.Fatalf("help: err=%v out=%q", err, out)
	}
	if _, err := runStub(t, cli, artifact("shred")); err == nil || !strings.Contains(err.Error(), `unknown issueops artifact subcommand "shred"`) {
		t.Fatalf("unknown artifact subcommand: %v", err)
	}
}

func TestReviewRecordCommandsRejectUnknownSubcommandsAndMapFlags(t *testing.T) {
	var gotImpl issueopscontract.IssueOpsImplementationReviewRequest
	var gotDocs issueopscontract.IssueOpsProjectDocsReviewRequest
	var gotSchema issueopscontract.IssueOpsSchemaEvidenceRequest
	record := issueopscontract.IssueOpsRecord{ID: "c1", Phase: "feedback", Repo: "/r"}
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		RecordIssueOpsImplementationReviewWithActor: func(_, _ string, req issueopscontract.IssueOpsImplementationReviewRequest, _ issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			gotImpl = req
			return record, nil
		},
		RecordIssueOpsProjectDocsReviewWithActor: func(_, _ string, req issueopscontract.IssueOpsProjectDocsReviewRequest, _ issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			gotDocs = req
			return record, nil
		},
		RecordIssueOpsSchemaEvidenceWithActor: func(_, _ string, req issueopscontract.IssueOpsSchemaEvidenceRequest, _ issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			gotSchema = req
			return record, nil
		},
	})
	for name, run := range map[string]func([]string) error{
		"implementation-review": cli.runIssueOpsImplementationReview,
		"project-docs-review":   cli.runIssueOpsProjectDocsReview,
		"schema-evidence":       cli.runIssueOpsSchemaEvidence,
	} {
		out, err := captureStdoutAndErrorForIssueOps(t, func() error { return run(nil) })
		if err != nil || !strings.Contains(out, "Usage: issueops "+name+" record") {
			t.Errorf("%s: no-arg must print usage: err=%v out=%q", name, err, out)
		}
		if _, err := captureStdoutAndErrorForIssueOps(t, func() error { return run([]string{"erase"}) }); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("unknown issueops %s subcommand", name)) {
			t.Errorf("%s: unknown subcommand error = %v", name, err)
		}
	}
	if _, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return cli.runIssueOpsImplementationReview([]string{"record", "--id", "c1", "--verdict", "pass", "--finding", "f1", "--finding", "f2", "--evidence", "e", "--reviewer-host", "codex", "--reviewer-model", "m", "--reviewer-effort", "high"})
	}); err != nil {
		t.Fatal(err)
	}
	if gotImpl.Verdict != "pass" || len(gotImpl.Findings) != 2 || gotImpl.Evidence[0] != "e" || gotImpl.ReviewerHost != "codex" || gotImpl.ReviewerModel != "m" || gotImpl.ReviewerEffort != "high" {
		t.Fatalf("implementation-review flags not mapped: %#v", gotImpl)
	}
	if _, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return cli.runIssueOpsProjectDocsReview([]string{"record", "--id", "c1", "--verdict", "no-change", "--reviewed-doc", "A.md", "--doc", "B.md", "--evidence", "read"})
	}); err != nil {
		t.Fatal(err)
	}
	if gotDocs.Verdict != "no-change" || len(gotDocs.ReviewedDocs) != 1 || gotDocs.Docs[0] != "B.md" || gotDocs.Evidence[0] != "read" {
		t.Fatalf("project-docs-review flags not mapped: %#v", gotDocs)
	}
	if _, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return cli.runIssueOpsSchemaEvidence([]string{"record", "--id", "c1", "--measurement", "rows=3", "--source", "psql", "--waive", "--waiver-rationale", "no db"})
	}); err != nil {
		t.Fatal(err)
	}
	if !gotSchema.Waive || gotSchema.WaiverRationale != "no db" || gotSchema.Measurements[0] != "rows=3" || gotSchema.Sources[0] != "psql" {
		t.Fatalf("schema-evidence flags not mapped: %#v", gotSchema)
	}
}

func TestReviewMetricsTextAndErrors(t *testing.T) {
	var fail error
	result := issueopscontract.IssueOpsReviewMetricsResult{
		Aggregate: issueopscontract.IssueOpsReviewMetricsAggregate{Cycles: 2, ReviewedCycles: 1, MeanRounds: 1.5, ReviseRatio: 0.25, StopRatio: 0},
		Cycles:    []issueopscontract.IssueOpsReviewMetricsCycle{{ID: "c1", Phase: "pr", DevilsAdvocateRounds: 2, RegressCount: 1}},
		Warnings:  []string{"one cycle unreadable"},
	}
	var gotID, gotRepo string
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		IssueOpsReviewMetrics: func(_, id, repo string) (issueopscontract.IssueOpsReviewMetricsResult, error) {
			gotID, gotRepo = id, repo
			return result, fail
		},
	})
	out, err := runStub(t, cli, func(c command) error { return c.runIssueOpsReviewMetrics([]string{"--repo", "/r"}) })
	if err != nil {
		t.Fatal(err)
	}
	if gotID != "" || gotRepo != "/r" {
		t.Fatalf("selectors not forwarded: id=%q repo=%q", gotID, gotRepo)
	}
	for _, want := range []string{"cycles: 2 (reviewed 1, mean rounds 1.50, revise 0.25, stop 0.00)", "  c1  phase=pr  rounds=2  regress=1", "  warning: one cycle unreadable"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	fail = errors.New("pick exactly one of --id and --repo")
	out, _ = runStub(t, cli, func(c command) error { return c.runIssueOpsReviewMetrics([]string{"--json"}) })
	assertIssueOpsStructuredFailure(t, out, "pick exactly one")
	if _, err := runStub(t, cli, func(c command) error { return c.runIssueOpsReviewMetrics(nil) }); !errors.Is(err, fail) {
		t.Fatalf("text mode must return the error, got %v", err)
	}
}

func TestChildStatusListAndValidationOutput(t *testing.T) {
	var gotRepair bool
	var fail error
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		IssueOpsChildStatusWithActor: func(_, _ string, repair bool, _ issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildStatusResult, error) {
			gotRepair = repair
			return issueopscontract.IssueOpsChildStatusResult{Children: []issueopscontract.IssueOpsChildStatusEntry{
				{CycleID: "k1", Phase: "plan", Branch: "5-k", ValidationVerdict: "accepted"},
			}}, fail
		},
		DropIssueOpsChildWithActor: func(_, parent, child, reason string, _ issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error) {
			return issueopscontract.IssueOpsChildValidationResult{ParentID: parent, ChildID: child}, fail
		},
	})
	out, err := runStub(t, cli, func(c command) error { return c.runIssueOpsChildStatus([]string{"--parent", "p", "--repair"}, true) })
	if err != nil || !gotRepair || strings.TrimSpace(out) != "k1 plan 5-k verdict=accepted" {
		t.Fatalf("status: err=%v repair=%v out=%q", err, gotRepair, out)
	}
	// child list is read-only: it must not even accept --repair.
	if _, err := runStub(t, cli, func(c command) error { return c.runIssueOpsChildStatus([]string{"--parent", "p", "--repair"}, false) }); err == nil {
		t.Fatal("child list must reject --repair")
	}
	_, err = runStub(t, cli, func(c command) error { return c.runIssueOpsChildStatus([]string{"--parent", "p"}, false) })
	if err != nil || gotRepair {
		t.Fatalf("child list must never repair: err=%v repair=%v", err, gotRepair)
	}

	out, err = runStub(t, cli, func(c command) error {
		return c.runIssueOpsChildDrop([]string{"--parent", "p", "--child", "k1", "--reason", "obsolete"})
	})
	if err != nil || !strings.Contains(out, "ParentID:p") || !strings.Contains(out, "ChildID:k1") {
		t.Fatalf("drop text must print the validation result: err=%v out=%q", err, out)
	}

	fail = errors.New("not your child")
	out, _ = runStub(t, cli, func(c command) error { return c.runIssueOpsChildStatus([]string{"--parent", "p", "--json"}, true) })
	assertIssueOpsStructuredFailure(t, out, "not your child")
	if _, err := runStub(t, cli, func(c command) error { return c.runIssueOpsChildStatus([]string{"--parent", "p"}, true) }); !errors.Is(err, fail) {
		t.Fatalf("text mode must return the error and print no rows, got %v", err)
	}
	out, _ = runStub(t, cli, func(c command) error {
		return c.runIssueOpsChildDrop([]string{"--parent", "p", "--child", "k1", "--json"})
	})
	assertIssueOpsStructuredFailure(t, out, "not your child")
}

func TestStatusAndRecordRoutingRouteErrors(t *testing.T) {
	fail := errors.New("no such cycle")
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		IssueOpsStatus: func(string, string) (issueopsstatuscontract.Record, error) {
			return issueopsstatuscontract.Record{}, fail
		},
		RecordIssueOpsRoutingWithActor: func(_, _, phase, skill string, _ issueopscontract.IssueOpsActor) (issueopsroutingcontract.Record, error) {
			return issueopsroutingcontract.Record{}, fail
		},
	})
	out, _ := runStub(t, cli, func(c command) error { return c.runIssueOpsStatus([]string{"--id", "x", "--json"}) })
	assertIssueOpsStructuredFailure(t, out, "no such cycle")
	out, _ = runStub(t, cli, func(c command) error {
		return c.runIssueOpsRecordRouting([]string{"--id", "x", "--phase", "plan", "--skill", "s", "--json"})
	})
	assertIssueOpsStructuredFailure(t, out, "no such cycle")
}

func TestIssueOpsCommandIDResolvesFlagFormsPerCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"separate value", []string{"phase", "--id", " c1 ", "--to", "plan"}, "c1"},
		{"equals value", []string{"phase", "--id=c2", "--to", "plan"}, "c2"},
		{"child commands key on parent", []string{"child", "status", "--parent", "p1", "--id", "ignored"}, "p1"},
		{"child with equals parent", []string{"child", "status", "--parent=p2"}, "p2"},
		{"flag without value", []string{"phase", "--id"}, ""},
		{"absent", []string{"phase", "--to", "plan"}, ""},
		{"child without parent ignores id", []string{"child", "status", "--id", "c3"}, ""},
	}
	for _, test := range tests {
		if got := issueOpsCommandID(test.args); got != test.want {
			t.Errorf("%s: id = %q, want %q", test.name, got, test.want)
		}
	}
}

func generatedEnvelope(path, sha string, generation uint64) []string {
	return []string{
		commandparsecontract.GeneratedByExecutableFlag, path,
		commandparsecontract.GeneratedBySHA256Flag, sha,
		commandparsecontract.GeneratedForGenerationFlag, fmt.Sprint(generation),
	}
}

func TestGeneratedInvocationFailsClosedBeforeObservation(t *testing.T) {
	sha := strings.Repeat("a", 64)
	execution := &issueopscontract.Execution{Lease: issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusActive, Generation: 2}}
	var readErr error
	var record issueopscontract.IssueOpsRecord
	cli := stubRuntimeCommand(IssueOpsCLIDeps{
		ReadIssueOps: func(_, id string) (issueopscontract.IssueOpsRecord, error) { return record, readErr },
	})
	envelope := generatedEnvelope("/bin/issueops", sha, 2)
	base := []string{"phase", "--to", "plan"}

	// No envelope: the invocation is not generated and passes through untouched.
	clean, generated, err := cli.prepareGeneratedCommandInvocation(append([]string{"phase", "--id", "c1"}, base[1:]...), Dependencies{})
	if err != nil || generated || len(clean) != 5 {
		t.Fatalf("plain invocation: clean=%v generated=%v err=%v", clean, generated, err)
	}

	// Envelope without an id cannot be bound to a cycle.
	_, generated, err = cli.prepareGeneratedCommandInvocation(append(append([]string{}, base...), envelope...), Dependencies{})
	var provenanceErr *commandparsecontract.GeneratedCommandProvenanceError
	if !generated || !errors.As(err, &provenanceErr) || provenanceErr.Code != "generated_command_provenance_invalid" || !strings.Contains(err.Error(), "requires an IssueOps id") {
		t.Fatalf("missing id: generated=%v err=%v", generated, err)
	}

	withID := append(append([]string{"phase", "--id", "c1"}, base[1:]...), envelope...)
	readErr = errors.New("record unreadable")
	if _, _, err := cli.prepareGeneratedCommandInvocation(withID, Dependencies{}); !errors.Is(err, readErr) {
		t.Fatalf("record read failure must surface, got %v", err)
	}
	readErr = nil

	// A cycle without execution state and without delegation has no lease authority.
	record = issueopscontract.IssueOpsRecord{ID: "c1"}
	if _, _, err := cli.prepareGeneratedCommandInvocation(withID, Dependencies{}); err == nil || !strings.Contains(err.Error(), "requires IssueOps execution state") {
		t.Fatalf("no execution state: %v", err)
	}

	// With authority but no observer configured the binary cannot be proven.
	record = issueopscontract.IssueOpsRecord{ID: "c1", Execution: execution}
	_, generated, err = cli.prepareGeneratedCommandInvocation(withID, Dependencies{})
	if !generated || !errors.As(err, &provenanceErr) || provenanceErr.Code != "generated_command_provenance_observation_failed" {
		t.Fatalf("missing observer must fail closed: generated=%v err=%v", generated, err)
	}
	// An observer that errors hides its diagnostic behind the generic observation failure.
	_, _, err = cli.prepareGeneratedCommandInvocation(withID, Dependencies{Provenance: failingProvenanceObserver{}})
	if !errors.As(err, &provenanceErr) || provenanceErr.Code != "generated_command_provenance_observation_failed" || strings.Contains(err.Error(), "adapter secret") {
		t.Fatalf("observer failure: %v", err)
	}
	// A matching observation is admitted, and the envelope is stripped from the args.
	observer := issueOpsProvenanceObserverStub{evidence: provenanceport.Receipt{ExecutablePath: "/bin/issueops", ExecutableSHA256: sha}}
	clean, generated, err = cli.prepareGeneratedCommandInvocation(withID, Dependencies{Provenance: observer})
	if err != nil || !generated || len(clean) != 5 {
		t.Fatalf("matching observation: clean=%v generated=%v err=%v", clean, generated, err)
	}
	// The same binary under a stale lease generation is refused.
	stale := append(append([]string{"phase", "--id", "c1"}, base[1:]...), generatedEnvelope("/bin/issueops", sha, 1)...)
	if _, _, err := cli.prepareGeneratedCommandInvocation(stale, Dependencies{Provenance: observer}); err == nil {
		t.Fatal("stale lease generation must be refused")
	}
}

type failingProvenanceObserver struct{}

func (failingProvenanceObserver) Observe(context.Context) (provenanceport.Receipt, error) {
	return provenanceport.Receipt{}, errors.New("adapter secret path /private")
}

func TestGeneratedDelegatedBootstrapAuthorityRequiresMatchingActiveParentTopology(t *testing.T) {
	root := t.TempDir()
	parentTree := filepath.Join(root, "parent-tree")
	otherTree := filepath.Join(root, "other-tree")
	for _, dir := range []string{parentTree, otherTree} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	newParent := func() issueopscontract.IssueOpsRecord {
		return issueopscontract.IssueOpsRecord{
			ID: "parent", Branch: "1-parent",
			ChildCycles: []issueopscontract.IssueOpsChildCycleRef{{CycleID: "child"}},
			Execution: &issueopscontract.Execution{
				Workspace: issueopscontract.Workspace{Root: parentTree},
				Lease:     issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusActive, Generation: 1},
			},
		}
	}
	newChild := func() issueopscontract.IssueOpsRecord {
		return issueopscontract.IssueOpsRecord{
			ID: "child", Branch: "2-child",
			Delegation: &issueopscontract.IssueOpsDelegationContract{ParentCycleID: "parent"},
			BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{
				Branch: "2-child", BaseBranch: "1-parent", ParentWorktree: parentTree,
			},
		}
	}
	branchPrepare := func(branch, base, tree string) []string {
		return []string{"branch", "prepare", "--id", "child", "--provider", "github", "--issue-url", "https://github.com/a/b/issues/2",
			"--branch", branch, "--base-branch", base, "--parent-worktree", tree,
			"--host", "codex", "--session-id", "s", "--cwd", tree}
	}
	executionPrepare := []string{"execution", "prepare", "--id", "child", "--mode", "delegated", "--host", "codex", "--session-id", "s", "--cwd", parentTree}

	tests := []struct {
		name    string
		args    []string
		mutate  func(child, parent *issueopscontract.IssueOpsRecord)
		parent  func(id string) (issueopscontract.IssueOpsRecord, error)
		wantErr string
	}{
		{name: "branch prepare matches parent topology", args: branchPrepare("2-child", "1-parent", parentTree)},
		{name: "execution prepare matches recorded topology", args: executionPrepare},
		{name: "non-bootstrap command is not delegated authority", args: []string{"phase", "--id", "child", "--to", "plan"}, wantErr: "requires IssueOps execution state"},
		{name: "child without delegation contract", args: branchPrepare("2-child", "1-parent", parentTree),
			mutate: func(child, _ *issueopscontract.IssueOpsRecord) { child.Delegation = nil }, wantErr: "requires IssueOps execution state"},
		{name: "parent unreadable", args: branchPrepare("2-child", "1-parent", parentTree),
			parent: func(string) (issueopscontract.IssueOpsRecord, error) {
				return issueopscontract.IssueOpsRecord{}, errors.New("gone")
			}, wantErr: "active referenced parent execution"},
		{name: "parent lease not active", args: branchPrepare("2-child", "1-parent", parentTree),
			mutate: func(_, parent *issueopscontract.IssueOpsRecord) {
				parent.Execution.Lease.Status = issueopscontract.LeaseStatusReleased
			}, wantErr: "active referenced parent execution"},
		{name: "parent has no execution", args: branchPrepare("2-child", "1-parent", parentTree),
			mutate: func(_, parent *issueopscontract.IssueOpsRecord) { parent.Execution = nil }, wantErr: "active referenced parent execution"},
		{name: "parent does not reference child", args: branchPrepare("2-child", "1-parent", parentTree),
			mutate: func(_, parent *issueopscontract.IssueOpsRecord) { parent.ChildCycles = nil }, wantErr: "active referenced parent execution"},
		{name: "branch differs from child record", args: branchPrepare("9-other", "1-parent", parentTree), wantErr: "does not match parent topology"},
		{name: "base differs from parent branch", args: branchPrepare("2-child", "main", parentTree), wantErr: "does not match parent topology"},
		{name: "parent worktree differs", args: branchPrepare("2-child", "1-parent", otherTree), wantErr: "does not match parent topology"},
		{name: "execution prepare without recorded branch prepare", args: executionPrepare,
			mutate: func(child, _ *issueopscontract.IssueOpsRecord) { child.BranchPrepare = nil }, wantErr: "execution preparation does not match parent topology"},
		{name: "execution prepare base differs", args: executionPrepare,
			mutate: func(child, _ *issueopscontract.IssueOpsRecord) { child.BranchPrepare.BaseBranch = "main" }, wantErr: "execution preparation does not match parent topology"},
		{name: "execution prepare parent worktree differs", args: executionPrepare,
			mutate: func(child, _ *issueopscontract.IssueOpsRecord) { child.BranchPrepare.ParentWorktree = otherTree }, wantErr: "execution preparation does not match parent topology"},
		{name: "execution prepare branch differs", args: executionPrepare,
			mutate: func(child, _ *issueopscontract.IssueOpsRecord) { child.BranchPrepare.Branch = "9-other" }, wantErr: "execution preparation does not match parent topology"},
		{name: "repeated branch flag is ambiguous", args: append(branchPrepare("2-child", "1-parent", parentTree), "--branch", "3-sneaky"), wantErr: "malformed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent, child := newParent(), newChild()
			if test.mutate != nil {
				test.mutate(&child, &parent)
			}
			cli := stubRuntimeCommand(IssueOpsCLIDeps{ReadIssueOps: func(_, id string) (issueopscontract.IssueOpsRecord, error) {
				if test.parent != nil {
					return test.parent(id)
				}
				return parent, nil
			}})
			authority, err := cli.generatedDelegatedBootstrapAuthority(test.args, child)
			if test.wantErr == "" {
				if err != nil || authority.ID != "parent" {
					t.Fatalf("expected parent authority, got %q err=%v", authority.ID, err)
				}
				return
			}
			var provenanceErr *commandparsecontract.GeneratedCommandProvenanceError
			if !errors.As(err, &provenanceErr) || provenanceErr.Code != "generated_command_provenance_invalid" || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want invalid provenance containing %q", err, test.wantErr)
			}
		})
	}
}
