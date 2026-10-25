package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"issueops/internal/contract/issueops"
	"issueops/internal/domain/commandparse"
	"issueops/internal/domain/shelltoken"
)

const (
	syncBaseWorkOID   = "1111111111111111111111111111111111111111"
	syncBaseBaseOID   = "2222222222222222222222222222222222222222"
	syncBaseMergeOID  = "3333333333333333333333333333333333333333"
	syncBaseRemoteOID = "1111111111111111111111111111111111111111"
	syncBaseFinalHead = "4444444444444444444444444444444444444444"
)

type syncBaseGitCall struct {
	dir  string
	args []string
}

// fakeSyncBaseGit은 주입된 Git 표면 전체를 대체해 호출 순서와 인자를 기록한다.
// 실제 git을 쓰지 않으므로 fetch→merge-tree→merge→push 계약을 결정적으로
// 검증할 수 있다. base fetch는 다른 관측과 겹쳐 돌므로 run은 잠금 아래에서
// 기록한다(케이스가 fake를 값 복사하므로 잠금은 fake 밖에 둔다).
var fakeSyncBaseGitMu sync.Mutex

type fakeSyncBaseGit struct {
	gitDir string
	calls  []syncBaseGitCall

	currentBranch string
	remoteRef     string
	headOID       string
	fetchHeadOID  string
	mergeHeadOID  string
	statusOut     string

	mergeHead      bool
	cherryPickHead bool
	rebaseHead     bool
	ancestor       map[string]bool
	// notAncestor names "ancestor..descendant" pairs that are not ancestry,
	// overriding the per-ancestor answer above.
	notAncestor map[string]bool

	fetchCode int
	fetchOut  string
	// trees answers rev-parse <oid>^{tree}.
	trees         map[string]string
	mergeTreeCode int
	mergeTreeOut  string
	mergeCode     int
	mergeOut      string
	// postMergeHead는 merge 성공 시 HEAD가 전이될 OID다 — 실제 git처럼
	// merge 이후에만 HEAD가 바뀌어야 apply의 fingerprint 재계산이 preview와
	// 일치한다(정적 선설정은 TOCTOU 게이트를 오탐시킨다).
	postMergeHead string
	unmergedOut   string
	diffCheckCode int
	commitCode    int
	pushCode      int
	pushOut       string
	aborted       bool
}

func (g *fakeSyncBaseGit) run(_ context.Context, dir string, args ...string) (int, string) {
	fakeSyncBaseGitMu.Lock()
	defer fakeSyncBaseGitMu.Unlock()
	g.calls = append(g.calls, syncBaseGitCall{dir: dir, args: append([]string(nil), args...)})
	switch args[0] {
	case "branch":
		return 0, g.currentBranch
	case "ls-remote":
		return 0, g.remoteRef
	case "rev-parse":
		return g.revParse(args[1:])
	case "status":
		return 0, g.statusOut
	case "fetch":
		return g.fetchCode, g.fetchOut
	case "merge-base":
		if g.notAncestor[args[len(args)-2]+".."+args[len(args)-1]] {
			return 1, ""
		}
		if g.ancestor[args[len(args)-2]] {
			return 0, ""
		}
		return 1, ""
	case "merge-tree":
		return g.mergeTreeCode, g.mergeTreeOut
	case "merge":
		if len(args) > 1 && args[1] == "--abort" {
			g.aborted = true
			return 0, ""
		}
		if g.mergeCode == 0 && g.postMergeHead != "" {
			g.headOID = g.postMergeHead
		}
		return g.mergeCode, g.mergeOut
	case "ls-files":
		return 0, g.unmergedOut
	case "diff":
		return g.diffCheckCode, "f.go:1: leftover conflict marker"
	case "commit":
		return g.commitCode, ""
	case "push":
		return g.pushCode, g.pushOut
	}
	return 0, ""
}

func (g *fakeSyncBaseGit) revParse(args []string) (int, string) {
	switch target := args[len(args)-1]; target {
	case "MERGE_HEAD":
		if !g.mergeHead {
			return 1, ""
		}
		return 0, g.mergeHeadOID
	case "CHERRY_PICK_HEAD":
		if !g.cherryPickHead {
			return 1, ""
		}
		return 0, syncBaseBaseOID
	case "REBASE_HEAD":
		if !g.rebaseHead {
			return 1, ""
		}
		return 0, syncBaseBaseOID
	case "rebase-merge", "rebase-apply", "MERGE_MSG":
		return 0, filepath.Join(g.gitDir, target)
	case "HEAD":
		return 0, g.headOID
	case "FETCH_HEAD":
		return 0, g.fetchHeadOID
	default:
		if tree, ok := g.trees[target]; ok {
			return 0, tree
		}
		return 0, ""
	}
}

func (g *fakeSyncBaseGit) verbs() []string {
	tracked := map[string]bool{"fetch": true, "merge-tree": true, "merge": true, "push": true, "commit": true}
	seen := []string{}
	for _, call := range g.calls {
		if tracked[call.args[0]] {
			seen = append(seen, call.args[0])
		}
	}
	return seen
}

func (g *fakeSyncBaseGit) callWith(verb string) []string {
	for _, call := range g.calls {
		if call.args[0] == verb {
			return call.args
		}
	}
	return nil
}

type syncBaseFixture struct {
	stateRoot string
	record    issueops.IssueOpsRecord
	worktree  string
	branch    string
	actor     issueops.NativeActor
	git       *fakeSyncBaseGit
}

func newReleasedSyncBaseFixture(t *testing.T, branch string) syncBaseFixture {
	t.Helper()
	stateRoot := t.TempDir()
	claimable := newClaimableExecutionFixture(t, stateRoot, branch)
	prepareExecutionCompletionFixture(t, stateRoot, &claimable)
	actor := executionActor("codex", "sync-base-"+branch)
	record, err := ReadIssueOps(stateRoot, claimable.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Lease.Status = issueops.LeaseStatusReleased
	record.Execution.Lease.Holder = nil
	record.Execution.Lease.ClaimTokenSHA256 = ""
	record.Execution.Lease.ReleasedAt = "2026-07-25T00:00:00Z"
	record.Execution.Completion = &issueops.ExecutionCompletion{
		Generation:             record.Execution.Lease.Generation,
		FinalHead:              syncBaseFinalHead,
		VerificationReportPath: filepath.Join(claimable.worktree, "verified-execution-report.json"),
		Verification:           []string{"go test ./... -count=1"},
		RemoteArtifactURL:      "https://github.com/example/issueops/pull/69",
		CompletedAt:            "2026-07-25T00:00:00Z",
	}
	record, err = writeIssueOps(context.Background(), stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	git := &fakeSyncBaseGit{
		gitDir:        t.TempDir(),
		currentBranch: branch,
		remoteRef:     syncBaseRemoteOID + "\trefs/heads/" + branch,
		headOID:       syncBaseWorkOID,
		fetchHeadOID:  syncBaseBaseOID,
		mergeHeadOID:  syncBaseBaseOID,
		ancestor:      map[string]bool{syncBaseRemoteOID: true},
	}
	if record.BranchPrepare != nil && record.BranchPrepare.BaseSHA != "" {
		// The base tip keeps the sealed base unless a case rewrites it.
		git.ancestor[record.BranchPrepare.BaseSHA] = true
	}
	return syncBaseFixture{stateRoot: stateRoot, record: record, worktree: claimable.worktree, branch: branch, actor: actor, git: git}
}

func TestReleasedSyncBaseFixtureRepresentsCurrentCompletion(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "318-released-fixture")
	record, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if record.Execution.Lease.Status != issueops.LeaseStatusReleased || record.Execution.Lease.Holder != nil ||
		record.Execution.Completion == nil || record.Execution.Completion.Generation != record.Execution.Lease.Generation {
		t.Fatalf("fixture is not a released current completion: %+v", record.Execution)
	}
}

func (f syncBaseFixture) request(mode string) issueops.ExecutionSyncBaseRequest {
	return issueops.ExecutionSyncBaseRequest{ID: f.record.ID, Mode: mode, CompletionGeneration: 1, Actor: f.actor, CWD: f.worktree}
}

func (f syncBaseFixture) run(t *testing.T, req issueops.ExecutionSyncBaseRequest) (issueops.ExecutionSyncBaseResult, error) {
	t.Helper()
	return VerifiedSyncExecutionBase(NativeActorVerifier())(context.Background(), f.stateRoot, req, issueops.ExecutionSyncBaseDeps{Git: f.git.run})
}

func (f syncBaseFixture) rewrite(t *testing.T, mutate func(*issueops.IssueOpsRecord)) {
	t.Helper()
	record, err := ReadIssueOps(f.stateRoot, f.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&record)
	if _, err := writeIssueOps(context.Background(), f.stateRoot, record); err != nil {
		t.Fatal(err)
	}
}

func (f syncBaseFixture) sealResolution(t *testing.T, conflicts ...string) {
	t.Helper()
	if len(conflicts) == 0 {
		conflicts = []string{"internal/a.go"}
	}
	f.rewrite(t, func(record *issueops.IssueOpsRecord) {
		record.Execution.SyncBaseResolution = &issueops.ExecutionSyncBaseResolution{
			Generation: record.Execution.Lease.Generation, CompletionGeneration: record.Execution.Completion.Generation,
			BaseOID: syncBaseBaseOID, Actor: f.actor, ConflictFiles: conflicts, StartedAt: "2026-08-04T00:00:00Z",
		}
	})
}

// 각 전제의 거부 코드를 검증한다. execution/worktree 부재나 확정 blocker의
// missing은 부분 진단이며, 관측하지 않은 원격 게이트까지 나열하지 않는다.
func TestExecutionSyncBaseGatesRejectEveryMissingPrecondition(t *testing.T) {
	t.Parallel()

	baseline := newReleasedSyncBaseFixture(t, "114-gates")
	cases := []struct {
		name    string
		mode    string
		mutate  func(*testing.T, *syncBaseFixture)
		missing string
	}{
		{"completion", issueops.ExecutionSyncBasePreview, func(t *testing.T, f *syncBaseFixture) {
			f.rewrite(t, func(r *issueops.IssueOpsRecord) { r.Execution.Completion = nil })
		}, "completion_present"},
		{"remote artifact", issueops.ExecutionSyncBasePreview, func(t *testing.T, f *syncBaseFixture) {
			f.rewrite(t, func(r *issueops.IssueOpsRecord) { r.RemoteArtifact = nil })
		}, "remote_artifact_present"},
		{"remote branch", issueops.ExecutionSyncBasePreview, func(_ *testing.T, f *syncBaseFixture) {
			f.git.remoteRef = ""
		}, "remote_branch_present"},
		{"pending intent", issueops.ExecutionSyncBasePreview, func(t *testing.T, f *syncBaseFixture) {
			f.rewrite(t, func(r *issueops.IssueOpsRecord) {
				r.Execution.Pending = &issueops.ExternalIntent{
					OperationID: "op-1", Kind: "remote_pr_create", Marker: "m", StartedAt: "2026-07-25T00:00:00Z",
				}
			})
		}, "pending_intent_absent"},
		{"cwd canonical", issueops.ExecutionSyncBasePreview, nil, "cwd_canonical"},
		{"detached head", issueops.ExecutionSyncBasePreview, func(_ *testing.T, f *syncBaseFixture) {
			f.git.currentBranch = ""
		}, "head_on_recorded_branch"},
		{"base fetch", issueops.ExecutionSyncBasePreview, func(_ *testing.T, f *syncBaseFixture) {
			f.git.fetchCode = 1
		}, "base_fetch"},
		{"merge state clean", issueops.ExecutionSyncBaseApply, func(_ *testing.T, f *syncBaseFixture) {
			f.git.mergeHead = true
		}, "merge_state_clean"},
		{"worktree clean", issueops.ExecutionSyncBaseApply, func(_ *testing.T, f *syncBaseFixture) {
			f.git.statusOut = " M internal/x.go"
		}, "worktree_clean"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := baseline
			gitState := *baseline.git
			fixture.git = &gitState
			if _, err := writeIssueOps(context.Background(), fixture.stateRoot, baseline.record); err != nil {
				t.Fatal(err)
			}
			if tc.mutate != nil {
				tc.mutate(t, &fixture)
			}
			req := fixture.request(tc.mode)
			req.Confirm, req.Fingerprint = true, strings.Repeat("a", 64)
			switch tc.missing {
			case "cwd_canonical":
				req.CWD = t.TempDir()
			}
			result, err := fixture.run(t, req)
			if err == nil || !containsString(result.Missing, tc.missing) {
				t.Fatalf("expected missing %q: err=%v missing=%v", tc.missing, err, result.Missing)
			}
			// An agent ran sync-base from the source checkout as the merge
			// skill's replace step is; the refusal names where to run it.
			if tc.missing == "cwd_canonical" && !strings.Contains(err.Error(), "run it from the canonical worktree") {
				t.Fatalf("cwd refusal does not name the worktree: %v", err)
			}
		})
	}

	t.Run("worktree present", func(t *testing.T) {
		fixture := baseline
		fixture.git.calls = nil
		if err := os.RemoveAll(fixture.worktree); err != nil {
			t.Fatal(err)
		}
		result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
		if err == nil || !containsString(result.Missing, "worktree_present") {
			t.Fatalf("absent canonical worktree must fail closed: %v %v", err, result.Missing)
		}
		if len(fixture.git.calls) != 0 {
			t.Fatalf("absent worktree must return partial diagnostics without Git: %v", fixture.git.calls)
		}
	})
}

// preview는 released·비-holder에서도 진단 채널로 열려 있어야 하고, 예상 충돌
// 파일과 fingerprint를 함께 발급해야 한다.
func TestExecutionSyncBasePreviewReportsConflictsAndIssuesFingerprint(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-preview")
	fixture.rewrite(t, func(r *issueops.IssueOpsRecord) {
		r.Execution.Lease.Status = issueops.LeaseStatusReleased
		r.Execution.Lease.Holder = nil
		r.Execution.Lease.ReleasedAt = "2026-07-25T00:00:00Z"
	})
	fixture.git.mergeTreeCode = 1
	fixture.git.mergeTreeOut = "treeoid\x00internal/a.go\x00internal/b.go\x00\x00CONFLICT (content)\n"

	req := fixture.request(issueops.ExecutionSyncBasePreview)
	req.Actor = issueops.NativeActor{}
	result, err := fixture.run(t, req)
	if err != nil {
		t.Fatalf("released preview must stay open as a diagnosis channel: %v", err)
	}
	if len(result.Fingerprint) != 64 || !result.MergeNeeded || !result.RemoteBranchPresent {
		t.Fatalf("preview did not expose the merge inventory: %#v", result)
	}
	if len(result.ConflictFiles) != 2 || result.ConflictFiles[0] != "internal/a.go" {
		t.Fatalf("preview did not list predicted conflicts: %#v", result.ConflictFiles)
	}
	if !strings.Contains(result.NextCommand, "--apply --confirm --fingerprint "+result.Fingerprint) {
		t.Fatalf("preview must hand over one finite apply command: %q", result.NextCommand)
	}
	assertReleasedSyncBaseCommand(t, result.NextCommand, fixture, "--apply")
	for _, verb := range fixture.git.verbs() {
		if verb == "merge" || verb == "push" || verb == "commit" {
			t.Fatalf("preview must not touch the worktree: %v", fixture.git.verbs())
		}
	}
}

// An agent got merge_needed:false and an --apply command at once. With the
// base already merged and the branch pushed there is nothing to apply.
func TestExecutionSyncBasePreviewWithNothingToSyncDoesNotHandOutApply(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-noop")
	fixture.git.ancestor[syncBaseBaseOID] = true
	fixture.git.remoteRef = syncBaseWorkOID + "\trefs/heads/114-noop"
	result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	if result.MergeNeeded || strings.Contains(result.NextCommand, "--apply") || !strings.Contains(result.NextCommand, "issueops next --id") {
		t.Fatalf("merge_needed=%v next_command=%q", result.MergeNeeded, result.NextCommand)
	}
}

// 무충돌 fast 경로: fetch→merge-tree→merge→push 순서와 인자를 전수 검증한다.
func TestExecutionSyncBaseApplyRunsFetchMergePushInOrder(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-apply")
	preview, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	fixture.git.calls = nil
	fixture.git.postMergeHead = syncBaseMergeOID

	req := fixture.request(issueops.ExecutionSyncBaseApply)
	req.Confirm, req.Fingerprint = true, preview.Fingerprint
	result, err := fixture.run(t, req)
	if err != nil {
		t.Fatalf("clean fast path must complete: %v", err)
	}
	if !result.Merged || !result.Pushed || result.MergeCommit != syncBaseMergeOID {
		t.Fatalf("apply did not complete the merge and push: %#v", result)
	}
	want := []string{"fetch", "merge-tree", "merge", "push"}
	got := fixture.git.verbs()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("git sequence = %v, want %v", got, want)
	}
	if args := fixture.git.callWith("fetch"); strings.Join(args, " ") != "fetch --quiet origin main" {
		t.Fatalf("fetch must precede the merge with the recorded base branch: %v", args)
	}
	if args := fixture.git.callWith("merge-tree"); strings.Join(args, " ") !=
		"merge-tree --write-tree --name-only -z "+syncBaseWorkOID+" "+syncBaseBaseOID {
		t.Fatalf("merge-tree args = %v", args)
	}
	if args := fixture.git.callWith("merge"); strings.Join(args, " ") != "merge --no-ff --no-edit "+syncBaseBaseOID {
		t.Fatalf("merge must be a non-fast-forward merge of the fetched base: %v", args)
	}
	if args := fixture.git.callWith("push"); strings.Join(args, " ") !=
		"push origin refs/heads/"+fixture.branch+":refs/heads/"+fixture.branch {
		t.Fatalf("push must be an explicit non-forced refspec: %v", args)
	}
}

// 충돌은 merge-in-progress를 남기고 정지한다 — push도 이벤트도 없다.
func TestExecutionSyncBaseApplyStopsAtConflictWithoutPush(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-conflict")
	preview, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	fixture.git.mergeTreeCode, fixture.git.mergeTreeOut = 1, "treeoid\x00internal/a.go\x00\x00"
	fixture.git.mergeCode, fixture.git.mergeOut = 1, "CONFLICT (content): Merge conflict in internal/a.go"

	req := fixture.request(issueops.ExecutionSyncBaseApply)
	req.Confirm, req.Fingerprint = true, preview.Fingerprint
	result, err := fixture.run(t, req)
	if err != nil {
		t.Fatalf("conflict stop is an actionable outcome, not a gate failure: %v", err)
	}
	if result.Merged || result.Pushed || !result.MergeInProgress || len(result.ConflictFiles) != 1 {
		t.Fatalf("conflict stop did not preserve the merge-in-progress contract: %#v", result)
	}
	if !strings.Contains(result.NextCommand, "--finalize") {
		t.Fatalf("conflict stop must name the resolution command: %q", result.NextCommand)
	}
	assertReleasedSyncBaseCommand(t, result.NextCommand, fixture, "--finalize")
	assertReleasedSyncBaseCommand(t, result.AbortCommand, fixture, "--abort")
	persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Execution.SyncBaseEvents) != 0 {
		t.Fatalf("conflict stop must not record a durable event: %#v", persisted.Execution.SyncBaseEvents)
	}
	resolution := persisted.Execution.SyncBaseResolution
	if resolution == nil || resolution.Generation != persisted.Execution.Lease.Generation ||
		!sameNativeActor(&resolution.Actor, &fixture.actor) ||
		len(resolution.ConflictFiles) != 1 || resolution.ConflictFiles[0] != "internal/a.go" {
		t.Fatalf("conflict stop must seal bounded resolution authority: %#v", resolution)
	}
}

// push 실패는 로컬 merge commit을 남기고, 재실행은 merge 없이 push만 수행해
// 멱등 수렴한다(설계 v2 push 계약).
func TestExecutionSyncBaseApplyPushFailureConvergesIdempotently(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-push-retry")
	preview, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	fixture.git.postMergeHead = syncBaseMergeOID
	fixture.git.pushCode, fixture.git.pushOut = 1, "! [rejected] non-fast-forward"

	req := fixture.request(issueops.ExecutionSyncBaseApply)
	req.Confirm, req.Fingerprint = true, preview.Fingerprint
	result, err := fixture.run(t, req)
	if err == nil || result.FailedStep != "push" || !result.PushRetryRequired {
		t.Fatalf("push failure must surface as a typed error: err=%v result=%#v", err, result)
	}
	assertReleasedSyncBaseCommand(t, result.NextCommand, fixture, "--preview")

	// 병합은 이미 반영됐다 — 재preview는 merge 불필요 + ahead를 보고해야 한다.
	// The push failed, so the remote branch still lacks the base.
	fixture.git.ancestor[syncBaseBaseOID] = true
	fixture.git.notAncestor = map[string]bool{syncBaseBaseOID + ".." + syncBaseRemoteOID: true}
	fixture.git.pushCode, fixture.git.pushOut = 0, ""
	fixture.git.calls = nil
	retryPreview, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	if retryPreview.MergeNeeded || !retryPreview.PushRetryRequired {
		t.Fatalf("preview must report push-only convergence: %#v", retryPreview)
	}
	fixture.git.calls = nil
	retry := fixture.request(issueops.ExecutionSyncBaseApply)
	retry.Confirm, retry.Fingerprint = true, retryPreview.Fingerprint
	final, err := fixture.run(t, retry)
	if err != nil {
		t.Fatalf("apply re-run must converge: %v", err)
	}
	if !final.Pushed || final.Merged {
		t.Fatalf("apply re-run must skip the merge and only push: %#v", final)
	}
	for _, verb := range fixture.git.verbs() {
		if verb == "merge" || verb == "merge-tree" {
			t.Fatalf("re-run must not merge again: %v", fixture.git.verbs())
		}
	}
	persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Execution.SyncBaseEvents) != 1 {
		t.Fatalf("exactly one durable event must survive the retry: %#v", persisted.Execution.SyncBaseEvents)
	}
}

// QA: the base already on the pushed branch plus unpushed work commits is not
// a failed sync push; the preview hands out next, not an apply.
func TestExecutionSyncBasePreviewTreatsUnpushedWorkOnASyncedBranchAsNoOp(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-synced-ahead")
	const pushedOID = "4444444444444444444444444444444444444444"
	fixture.git.remoteRef = pushedOID + "\trefs/heads/114-synced-ahead"
	fixture.git.ancestor[pushedOID] = true
	fixture.git.ancestor[syncBaseBaseOID] = true
	preview, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	if preview.MergeNeeded || preview.PushRetryRequired || !strings.HasPrefix(preview.NextCommand, "issueops next --id ") {
		t.Fatalf("synced branch with own commits must not offer an apply: %#v", preview)
	}
	// Without the base on the pushed branch the same ahead state is a retry.
	fixture.git.notAncestor = map[string]bool{syncBaseBaseOID + ".." + pushedOID: true}
	retry, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil || !retry.PushRetryRequired || !strings.Contains(retry.NextCommand, "--apply") {
		t.Fatalf("unpushed sync must still offer the push retry: err=%v %#v", err, retry)
	}
}

// 성공한 apply는 durable 이벤트를 남기고 Completion.FinalHead는 불변이다.
func TestExecutionSyncBaseRecordsDurableEventAndKeepsFinalHeadImmutable(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-durable")
	preview, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	fixture.git.postMergeHead = syncBaseMergeOID
	req := fixture.request(issueops.ExecutionSyncBaseApply)
	req.Confirm, req.Fingerprint = true, preview.Fingerprint
	if _, err := fixture.run(t, req); err != nil {
		t.Fatal(err)
	}
	persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	events := persisted.Execution.SyncBaseEvents
	if len(events) != 1 {
		t.Fatalf("apply must append exactly one durable event: %#v", events)
	}
	event := events[0]
	if event.Mode != issueops.ExecutionSyncBaseEventApply || event.BaseOID != syncBaseBaseOID ||
		event.MergeCommit != syncBaseMergeOID || event.BaseBranch != "main" ||
		!strings.Contains(event.Actor, "sync-base-114-durable") || strings.TrimSpace(event.At) == "" {
		t.Fatalf("durable event is not the merge receipt: %#v", event)
	}
	if persisted.Execution.Completion.FinalHead != syncBaseFinalHead {
		t.Fatalf("Completion.FinalHead must stay immutable: %q", persisted.Execution.Completion.FinalHead)
	}
}

// finalize는 미해소 인덱스와 잔존 충돌 마커를 각각 거부한다.
func TestExecutionSyncBaseFinalizeRejectsUnresolvedConflictsAndMarkers(t *testing.T) {
	t.Parallel()

	t.Run("unmerged index", func(t *testing.T) {
		fixture := newReleasedSyncBaseFixture(t, "114-finalize-unmerged")
		fixture.git.mergeHead = true
		fixture.sealResolution(t)
		fixture.git.unmergedOut = "100644 " + syncBaseBaseOID + " 1\tinternal/a.go\x00"
		result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBaseFinalize))
		if err == nil || !containsString(result.Missing, "conflict_resolution_complete") {
			t.Fatalf("unresolved paths must block finalize: %v %v", err, result.Missing)
		}
		if fixture.git.callWith("commit") != nil || fixture.git.callWith("push") != nil {
			t.Fatal("blocked finalize must not commit or push")
		}
	})
	t.Run("conflict markers", func(t *testing.T) {
		fixture := newReleasedSyncBaseFixture(t, "114-finalize-markers")
		fixture.git.mergeHead = true
		fixture.sealResolution(t)
		fixture.git.diffCheckCode = 1
		result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBaseFinalize))
		if err == nil || !containsString(result.Missing, "conflict_markers_absent") {
			t.Fatalf("leftover conflict markers must block finalize: %v %v", err, result.Missing)
		}
	})
	t.Run("clean finalize", func(t *testing.T) {
		fixture := newReleasedSyncBaseFixture(t, "114-finalize-clean")
		fixture.git.mergeHead = true
		fixture.sealResolution(t)
		fixture.git.headOID = syncBaseMergeOID
		result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBaseFinalize))
		if err != nil {
			t.Fatalf("resolved finalize must complete: %v", err)
		}
		if !result.Merged || !result.Pushed {
			t.Fatalf("finalize did not commit and push: %#v", result)
		}
		// finalize는 재fetch하지 않는다 — base tip은 MERGE_HEAD가 확정한다.
		if fixture.git.callWith("fetch") != nil {
			t.Fatal("finalize must not re-fetch the base")
		}
		persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(persisted.Execution.SyncBaseEvents) != 1 ||
			persisted.Execution.SyncBaseEvents[0].Mode != issueops.ExecutionSyncBaseEventFinalize {
			t.Fatalf("finalize must record its own durable event: %#v", persisted.Execution.SyncBaseEvents)
		}
		if persisted.Execution.SyncBaseResolution != nil {
			t.Fatalf("finalize retained resolution authority: %#v", persisted.Execution.SyncBaseResolution)
		}
	})
	t.Run("foreign actor", func(t *testing.T) {
		fixture := newReleasedSyncBaseFixture(t, "114-finalize-foreign")
		fixture.git.mergeHead = true
		fixture.sealResolution(t)
		request := fixture.request(issueops.ExecutionSyncBaseFinalize)
		request.Actor.SessionID = "other-session"
		result, err := fixture.run(t, request)
		if err == nil || !containsString(result.Missing, "sync_base_resolution_actor") {
			t.Fatalf("foreign resolution actor must fail closed: %v %v", err, result.Missing)
		}
		if fixture.git.callWith("ls-remote") != nil || fixture.git.callWith("fetch") != nil {
			t.Fatal("foreign resolution actor must not query the remote")
		}
	})
}

func TestExecutionSyncBaseAbortWithdrawsTheMergeWithoutEvent(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-abort")
	fixture.git.mergeHead = true
	fixture.sealResolution(t)
	result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBaseAbort))
	if err != nil {
		t.Fatalf("abort must be available to the holder: %v", err)
	}
	if !result.Aborted || result.MergeInProgress || !fixture.git.aborted {
		t.Fatalf("abort did not withdraw the merge: %#v", result)
	}
	persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Execution.SyncBaseEvents) != 0 {
		t.Fatalf("abort is a withdrawal and must not be recorded: %#v", persisted.Execution.SyncBaseEvents)
	}
	if persisted.Execution.SyncBaseResolution != nil {
		t.Fatalf("abort retained resolution authority: %#v", persisted.Execution.SyncBaseResolution)
	}

	// 진행 중 머지가 없으면 abort/finalize 모두 전제 미충족이다.
	clean := newReleasedSyncBaseFixture(t, "114-abort-noop")
	clean.sealResolution(t)
	result, err = clean.run(t, clean.request(issueops.ExecutionSyncBaseAbort))
	if err == nil || !containsString(result.Missing, "merge_in_progress") {
		t.Fatalf("abort without a merge in progress must fail closed: %v %v", err, result.Missing)
	}
}

// fingerprint TOCTOU: preview 발급 이후 상태가 바뀌면 apply가 멈춘다.
func TestExecutionSyncBaseApplyRejectsStaleFingerprint(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-toctou")
	preview, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	// 외부에서 base가 전진했다 — 같은 fingerprint로는 진행할 수 없다.
	fixture.git.fetchHeadOID = syncBaseMergeOID
	req := fixture.request(issueops.ExecutionSyncBaseApply)
	req.Confirm, req.Fingerprint = true, preview.Fingerprint
	result, err := fixture.run(t, req)
	if err == nil || !strings.Contains(err.Error(), "stale execution sync-base fingerprint") {
		t.Fatalf("stale fingerprint was accepted: %v %#v", err, result)
	}
	assertReleasedSyncBaseCommand(t, result.NextCommand, fixture, "--preview")
	if fixture.git.callWith("merge") != nil {
		t.Fatal("stale fingerprint must stop before any merge")
	}

	missingConfirm := fixture.request(issueops.ExecutionSyncBaseApply)
	missingConfirm.Fingerprint = preview.Fingerprint
	if _, err := fixture.run(t, missingConfirm); err == nil {
		t.Fatal("apply without --confirm must be rejected")
	}
}

// git 2.38 미만 등으로 merge-tree --write-tree가 없으면 fail-closed다.
func TestExecutionSyncBaseFailsClosedWhenMergeTreeIsUnavailable(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-mergetree")
	fixture.git.mergeTreeCode, fixture.git.mergeTreeOut = 129, "unknown option `write-tree'"
	result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err == nil || !containsString(result.Missing, "merge_tree_supported") {
		t.Fatalf("unsupported merge-tree must fail closed in preview: %v %v", err, result.Missing)
	}
}

func TestExecutionSyncBaseReleasedCompletionAuthorityRejectsInvalidState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*issueops.IssueOpsRecord)
		request func(*issueops.ExecutionSyncBaseRequest)
		missing string
	}{
		{name: "missing completion generation", request: func(req *issueops.ExecutionSyncBaseRequest) { req.CompletionGeneration = 0 }, missing: "completion_generation_present"},
		{name: "wrong completion generation", request: func(req *issueops.ExecutionSyncBaseRequest) { req.CompletionGeneration = 2 }, missing: "completion_generation_current"},
		{name: "history only", mutate: func(record *issueops.IssueOpsRecord) {
			record.Execution.Lease.Generation = 2
			record.Execution.CompletionHistory = []issueops.ExecutionCompletionHistory{{
				Generation: 1, Completion: *record.Execution.Completion, Reason: "prior reseed", ReopenedAt: "2026-07-26T00:00:00Z",
			}}
			record.Execution.Completion = nil
		}, missing: "completion_present"},
		{name: "claimable without current completion", mutate: func(record *issueops.IssueOpsRecord) {
			record.Execution.Lease.Status = issueops.LeaseStatusClaimable
			record.Execution.Lease.ClaimTokenSHA256 = strings.Repeat("b", 64)
			record.Execution.Completion = nil
		}, missing: "released_completion_authority"},
		{name: "pending intent", mutate: func(record *issueops.IssueOpsRecord) {
			record.Execution.Pending = &issueops.ExternalIntent{OperationID: "op-1", Kind: "remote_pr_create", Marker: "m", StartedAt: "2026-07-25T00:00:00Z"}
		}, missing: "pending_intent_absent"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newReleasedSyncBaseFixture(t, "318-authority-"+strings.ReplaceAll(test.name, " ", "-"))
			if test.mutate != nil {
				fixture.rewrite(t, test.mutate)
			}
			req := fixture.request(issueops.ExecutionSyncBaseApply)
			req.Confirm, req.Fingerprint = true, strings.Repeat("a", 64)
			if test.request != nil {
				test.request(&req)
			}
			result, err := fixture.run(t, req)
			if err == nil || !containsString(result.Missing, test.missing) {
				t.Fatalf("error=%v missing=%v want=%q", err, result.Missing, test.missing)
			}
			for _, verb := range []string{"ls-remote", "fetch", "merge-base", "merge-tree", "merge", "push"} {
				if args := fixture.git.callWith(verb); args != nil {
					t.Fatalf("blocked authority must not run %s: %v", verb, args)
				}
			}
		})
	}
}

func TestExecutionSyncBaseBlockersKeepLocalDiagnosticsWithoutNetwork(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{issueops.ExecutionSyncBasePreview, issueops.ExecutionSyncBaseApply} {
		t.Run(mode, func(t *testing.T) {
			fixture := newReleasedSyncBaseFixture(t, "318-efficiency-blocked-"+mode)
			fixture.rewrite(t, func(record *issueops.IssueOpsRecord) {
				record.Execution.Pending = &issueops.ExternalIntent{
					OperationID: "op-1", Kind: "remote_pr_create", Marker: "m", StartedAt: "2026-07-25T00:00:00Z",
				}
			})
			fixture.git.currentBranch = ""
			fixture.git.mergeHead = true
			fixture.git.statusOut = " M internal/x.go\n?? notes.txt"
			fixture.git.headOID = ""
			fixture.git.remoteRef, fixture.git.fetchCode = "", 1
			req := fixture.request(mode)
			req.CWD = t.TempDir()
			if mode == issueops.ExecutionSyncBasePreview {
				req.Actor = issueops.NativeActor{}
			}

			result, err := fixture.run(t, req)
			want := []string{"pending_intent_absent", "cwd_canonical", "head_on_recorded_branch"}
			if mode == issueops.ExecutionSyncBaseApply {
				want = append(want, "merge_state_clean", "worktree_clean")
			}
			want = append(want, "work_tip_resolved")
			if err == nil || result.OK || strings.Join(result.Missing, ",") != strings.Join(want, ",") {
				t.Fatalf("error=%v missing=%v want=%v", err, result.Missing, want)
			}
			if !result.MergeInProgress || result.Fingerprint != "" || result.BaseOID != "" {
				t.Fatalf("blocked local diagnostics=%+v", result)
			}
			if mode == issueops.ExecutionSyncBaseApply &&
				strings.Join(result.UntrackedWarnings, ",") != "notes.txt" {
				t.Fatalf("untracked warnings=%v", result.UntrackedWarnings)
			}
			counts := map[string]int{}
			for _, call := range fixture.git.calls {
				counts[strings.Join(call.args, " ")]++
			}
			for _, command := range []string{"branch --show-current", "rev-parse --verify --quiet MERGE_HEAD", "rev-parse HEAD"} {
				if counts[command] != 1 {
					t.Fatalf("local command %q count=%d", command, counts[command])
				}
			}
			for _, verb := range []string{"ls-remote", "fetch", "merge-base", "merge-tree", "merge", "push"} {
				if args := fixture.git.callWith(verb); args != nil {
					t.Fatalf("blocked invocation ran %s: %v", verb, args)
				}
			}
		})
	}
}

func TestExecutionSyncBaseActiveHolderMismatchSkipsNetwork(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "318-efficiency-foreign-holder")
	fixture.rewrite(t, func(record *issueops.IssueOpsRecord) {
		record.Execution.Lease.Status = issueops.LeaseStatusActive
		holder := fixture.actor
		holder.SessionID = "other-session"
		record.Execution.Lease.Holder = &holder
		record.Execution.Lease.ClaimedAt = "2026-07-25T00:00:00Z"
		record.Execution.Completion = nil
	})
	result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBaseApply))
	if err == nil || !containsString(result.Missing, "lease_holder") || result.WorkOID != syncBaseWorkOID {
		t.Fatalf("error=%v result=%+v", err, result)
	}
	for _, verb := range []string{"ls-remote", "fetch", "merge-tree", "merge", "push"} {
		if args := fixture.git.callWith(verb); args != nil {
			t.Fatalf("foreign holder ran %s: %v", verb, args)
		}
	}
}

func TestExecutionSyncBaseReleasedCompletionAuthorityRejectsInvalidProcessReceipt(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "318-dead-process")
	req := fixture.request(issueops.ExecutionSyncBaseApply)
	req.Actor.SessionProcess = &issueops.NativeProcessReceipt{PID: 999999, StartedAt: "2026-01-01T00:00:00Z", Executable: "/missing/codex"}
	req.Actor.ProcessAncestry = []issueops.NativeProcessReceipt{*req.Actor.SessionProcess}
	if _, err := fixture.run(t, req); err == nil {
		t.Fatal("dead native process receipt was accepted")
	}

	req = fixture.request(issueops.ExecutionSyncBaseApply)
	req.Actor.ProcessAncestry = []issueops.NativeProcessReceipt{{PID: 1, StartedAt: "other", Executable: "codex"}}
	if _, err := fixture.run(t, req); err == nil || !strings.Contains(err.Error(), "not in the local process ancestry") {
		t.Fatalf("mismatched ancestry error=%v", err)
	}
	if len(fixture.git.calls) != 0 {
		t.Fatalf("invalid actor must stop before Git diagnostics: %v", fixture.git.calls)
	}
}

func TestExecutionSyncBaseActiveHolderAuthorityRemainsSupported(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "318-active-holder")
	fixture.rewrite(t, func(record *issueops.IssueOpsRecord) {
		record.Execution.Lease.Status = issueops.LeaseStatusActive
		record.Execution.Lease.Holder = &fixture.actor
		record.Execution.Lease.ClaimedAt = "2026-07-25T00:00:00Z"
		record.Execution.Completion = nil
		record.RemoteArtifact = nil
	})
	previewRequest := fixture.request(issueops.ExecutionSyncBasePreview)
	previewRequest.CompletionGeneration = 0
	preview, err := fixture.run(t, previewRequest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(preview.NextCommand, "--completion-generation") {
		t.Fatalf("active-holder preview added released authority generation: %q", preview.NextCommand)
	}
	fixture.git.postMergeHead = syncBaseMergeOID
	applyRequest := fixture.request(issueops.ExecutionSyncBaseApply)
	applyRequest.CompletionGeneration = 0
	applyRequest.Confirm, applyRequest.Fingerprint = true, preview.Fingerprint
	if _, err := fixture.run(t, applyRequest); err != nil {
		t.Fatalf("active holder sync-base changed: %v", err)
	}
}

func assertReleasedSyncBaseCommand(t *testing.T, text string, fixture syncBaseFixture, mode string) {
	t.Helper()
	if strings.Contains(text, "$ACTOR_FLAGS") {
		process := fixture.actor.SessionProcess
		actorFlags := "--host " + quoteShellArg(fixture.actor.Host) +
			" --session-id " + quoteShellArg(fixture.actor.SessionID) +
			" --session-pid " + strconv.Itoa(process.PID) +
			" --session-started-at " + quoteShellArg(process.StartedAt) +
			" --session-executable " + quoteShellArg(process.Executable) +
			" --cwd " + quoteShellArg(fixture.worktree)
		text = strings.Replace(text, "$ACTOR_FLAGS", actorFlags, 1)
	}
	tokens := shelltoken.SplitCommandTokens(text)
	if len(tokens) == 0 {
		t.Fatalf("command is empty: %q", text)
	}
	command, ok := commandparse.ParseExactIssueOpsArgs(tokens[1:])
	if !ok || command.Path != "execution sync-base" {
		t.Fatalf("command is not exact sync-base: %q", text)
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("missing sync-base command spec")
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok || len(flags[mode]) != 1 || len(flags["--completion-generation"]) != 1 || flags["--completion-generation"][0] != "1" || flags["--id"][0] != fixture.record.ID {
		t.Fatalf("released sync-base command flags=%v command=%q", flags, text)
	}
}

func TestExecutionSyncBaseRejectsUnknownMode(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-mode")
	if _, err := fixture.run(t, fixture.request("rebase")); err == nil {
		t.Fatal("unsupported mode must be rejected; rebase is explicitly out of scope")
	}
}

// The follow-up command carries the caller's complete actor vector so it runs
// as printed; a partial vector keeps the placeholder rather than a command that
// fails closed on partial ACTOR_FLAGS.
func TestExecutionSyncBaseActorFlagsRendersOnlyACompleteVector(t *testing.T) {
	actor := issueops.NativeActor{Host: "omp", SessionID: "s 1", SessionProcess: &issueops.NativeProcessReceipt{
		PID: 42, StartedAt: "2026-10-24T00:00:00Z", Executable: "/bin/omp",
	}}
	got := executionSyncBaseActorFlags(actor, "/tmp/w t")
	want := "--host 'omp' --session-id 's 1' --session-pid 42 --session-started-at '2026-10-24T00:00:00Z' --session-executable '/bin/omp' --cwd '/tmp/w t'"
	if got != want {
		t.Fatalf("flags = %q, want %q", got, want)
	}
	actor.SessionProcess = nil
	if got := executionSyncBaseActorFlags(actor, "/tmp/w"); got != "$ACTOR_FLAGS" {
		t.Fatalf("partial vector rendered %q", got)
	}
}

// Probed: main was force-pushed (a sealed-base commit dropped) and preview
// offered a clean merge that would carry the dropped commit back via the PR.
func TestExecutionSyncBaseRefusesABaseTipWithoutTheSealedBase(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-rewritten")
	sealed := fixture.record.BranchPrepare.BaseSHA
	fixture.git.notAncestor = map[string]bool{sealed + ".." + syncBaseBaseOID: true}
	_, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err == nil || !strings.Contains(err.Error(), "sealed_base_on_base_tip") || !strings.Contains(err.Error(), "no longer contains the sealed base") {
		t.Fatalf("preview over a rewritten base = %v", err)
	}
}

// Probed: with the base branch deleted on origin the refusal named only
// the base_fetch gate.
func TestExecutionSyncBaseFetchFailureNamesGitsMessage(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-nobase")
	fixture.git.fetchCode = 128
	fixture.git.fetchOut = "fatal: couldn't find remote ref main"
	_, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err == nil || !strings.Contains(err.Error(), "base_fetch; git fetch origin") || !strings.Contains(err.Error(), "couldn't find remote ref main") {
		t.Fatalf("preview with a failed base fetch = %v", err)
	}
}

// Stacked QA: the parent PR was squash-merged and the branch retargeted to
// main, so the sealed base (the parent's tip) is not an ancestor of main,
// but its content is in main. Merging main brings only this cycle's change.
// Without a retarget the same tree-equal shape is a same-branch history
// rewrite (a purge) and stays refused; a base tip merged in earlier that the
// new base dropped is refused too.
func TestExecutionSyncBaseAcceptsASquashMergedParentOnlyAfterARetarget(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T, name string, retarget bool, mergedEarlier string) syncBaseFixture {
		fixture := newReleasedSyncBaseFixture(t, name)
		fixture.rewrite(t, func(r *issueops.IssueOpsRecord) {
			if retarget {
				url := ""
				if r.RemoteArtifact != nil {
					url = r.RemoteArtifact.URL
				}
				r.BranchPrepare.Retargets = []issueops.IssueOpsBranchRetarget{{FromBase: "parent", ToBase: r.BranchPrepare.BaseBranch, Reason: "parent squash-merged", ArtifactURL: url, ObservedAt: "2026-07-25T00:00:00Z"}}
			}
			if mergedEarlier != "" {
				r.Execution.SyncBaseEvents = []issueops.ExecutionSyncBaseEvent{{Mode: issueops.ExecutionSyncBaseEventApply, BaseBranch: "parent", BaseOID: mergedEarlier, MergeCommit: strings.Repeat("c", 40), Actor: "codex", At: "2026-07-25T00:00:00Z"}}
			}
		})
		sealed := fixture.record.BranchPrepare.BaseSHA
		fixture.git.notAncestor = map[string]bool{sealed + ".." + syncBaseBaseOID: true, strings.Repeat("a", 40) + ".." + syncBaseBaseOID: true}
		fixture.git.mergeTreeOut = "basetree\n"
		fixture.git.trees = map[string]string{syncBaseBaseOID + "^{tree}": "basetree"}
		return fixture
	}
	fixture := setup(t, "114-stacked", true, "")
	if result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview)); err != nil || !result.MergeNeeded {
		t.Fatalf("retargeted squash-merged parent = %+v, %v", result, err)
	}
	fixture = setup(t, "114-purge", false, "")
	if _, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview)); err == nil || !strings.Contains(err.Error(), "sealed_base_on_base_tip") {
		t.Fatalf("same-branch tree-equal rewrite must stay refused: %v", err)
	}
	// The sealed base is held by ancestry; a parent tip merged in earlier is
	// neither an ancestor of main nor held by content (the parent dropped
	// something before its squash merge).
	fixture = setup(t, "114-earlier", true, strings.Repeat("a", 40))
	fixture.git.notAncestor = map[string]bool{strings.Repeat("a", 40) + ".." + syncBaseBaseOID: true}
	fixture.git.mergeTreeOut = "othertree\n"
	if _, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview)); err == nil || !strings.Contains(err.Error(), "sealed_base_on_base_tip") {
		t.Fatalf("an earlier merged base tip the new base dropped must be refused: %v", err)
	}
}

// Probe: a preview after an apply paused on conflicts handed out another apply.
// While the merge is open the way on is finalize (or abort).
func TestExecutionSyncBasePreviewDuringAMergeHandsOutFinalize(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "114-open-merge")
	fixture.git.mergeHead = true
	result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	if !result.MergeInProgress || !strings.Contains(result.NextCommand, " --finalize ") || strings.Contains(result.NextCommand, "--apply") || !strings.Contains(result.AbortCommand, " --abort ") {
		t.Fatalf("next=%q abort=%q", result.NextCommand, result.AbortCommand)
	}
}
