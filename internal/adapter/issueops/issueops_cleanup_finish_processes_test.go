package issueops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// quietCleanupProcesses는 점유가 없는 워크트리 관측이다. 요청자(777)의 계보는
// 항상 관측된다 — 관측 실패는 별도 게이트다.
func quietCleanupProcesses() CleanupProcessDeps {
	return CleanupProcessDeps{
		Observe: func(string) (port.CleanupWorkspaceOccupancy, error) {
			return port.CleanupWorkspaceOccupancy{Ancestry: map[int][]int{777: {700, 1}}}, nil
		},
		Signal:  func(int, syscall.Signal) error { return syscall.ESRCH },
		Sleep:   func(time.Duration) {},
		SelfPID: 777,
	}
}

func worldCleanupProcesses(world *fakeCleanupProcessWorld) CleanupProcessDeps {
	return CleanupProcessDeps{
		Observe: world.observe, Signal: world.signal, Sleep: func(time.Duration) {}, SelfPID: 777,
	}
}

func codexOccupant() issueops.CleanupWorkspaceProcess {
	return issueops.CleanupWorkspaceProcess{PID: 4321, Command: "codex", StartedAt: "2026-08-27T00:00:01Z", Executable: "codex", Descendants: 1, Collateral: 1}
}

func occupiedWorld(t *testing.T, occupants ...issueops.CleanupWorkspaceProcess) *fakeCleanupProcessWorld {
	world := &fakeCleanupProcessWorld{t: t, occupants: map[int]issueops.CleanupWorkspaceProcess{}, ancestry: map[int][]int{777: {700, 1}}, diesOn: map[int]syscall.Signal{}}
	for _, occupant := range occupants {
		world.occupants[occupant.PID] = occupant
		world.ancestry[occupant.PID] = []int{1}
	}
	return world
}

// AC-01: 점유자가 있다는 사실은 더 이상 preview를 막지 않는다. 대신 receipt를
// 결과에 싣는다(#477).
func TestCleanupFinishPreviewListsOccupantsWithoutBlocking(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	deps.Processes = worldCleanupProcesses(occupiedWorld(t, codexOccupant()))

	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatalf("occupants alone must not block the preview: %v missing=%v", err, result.Missing)
	}
	if containsString(result.Missing, "workspace_processes_quiescent") || len(result.Missing) != 0 {
		t.Fatalf("preview must be ready with occupants listed: %v", result.Missing)
	}
	if len(result.WorkspaceProcesses) != 1 || result.WorkspaceProcesses[0] != codexOccupant() {
		t.Fatalf("occupants must be reported with receipt and descendant counts: %+v", result.WorkspaceProcesses)
	}
	if result.Fingerprint == "" || !strings.Contains(result.NextCommand, result.Fingerprint) {
		t.Fatalf("preview must issue a fingerprint-bound apply command: %+v", result)
	}
}

// AC-02: fingerprint는 점유 receipt 집합에 결속된다.
func TestCleanupFinishFingerprintBindsOccupantReceipts(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	preview := func(occupants []issueops.CleanupWorkspaceProcess) string {
		deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
		deps.Processes = worldCleanupProcesses(occupiedWorld(t, occupants...))
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		if err != nil {
			t.Fatal(err)
		}
		return result.Fingerprint
	}
	base := preview([]issueops.CleanupWorkspaceProcess{codexOccupant()})
	if again := preview([]issueops.CleanupWorkspaceProcess{codexOccupant()}); again != base {
		t.Fatal("identical occupancy must reproduce the fingerprint")
	}
	reused := codexOccupant()
	reused.StartedAt = "2026-08-27T00:05:00Z"
	if preview([]issueops.CleanupWorkspaceProcess{reused}) == base {
		t.Fatal("a changed receipt (pid reuse) must change the fingerprint")
	}
	if preview(nil) == base {
		t.Fatal("quiet occupancy must not share the occupied fingerprint")
	}
}

// AC-03: apply는 HUP+TERM → (KILL) → 재관측 순서로 점유를 없앤 뒤에야
// git worktree remove로 넘어간다.
func TestCleanupFinishApplyStopsOccupantsInOrder(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123"}
	trace := []string{}
	world := occupiedWorld(t, codexOccupant())
	world.diesOn[4321] = syscall.SIGHUP
	processes := worldCleanupProcesses(world)
	processes.Signal = func(pid int, sig syscall.Signal) error {
		trace = append(trace, fmt.Sprintf("sig:%d:%s", pid, sig))
		return world.signal(pid, sig)
	}
	deps := finishDeps(git)
	deps.Processes = processes
	deps.Git = func(dir string, args ...string) (int, string) {
		trace = append(trace, "git:"+args[0])
		return git.run(dir, args...)
	}

	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil || !result.OK || !result.RecordDeleted {
		t.Fatalf("apply must converge: err=%v result=%+v", err, result)
	}
	if len(result.WorkspaceProcessesStopped) != 1 || result.WorkspaceProcessesStopped[0].PID != 4321 {
		t.Fatalf("apply must report what it stopped: %+v", result)
	}
	hup, remove := slices.Index(trace, "sig:4321:hangup"), slices.Index(trace, "git:worktree")
	if hup < 0 || remove < 0 || hup >= remove {
		t.Fatalf("order must be signals → git worktree remove: %v", trace)
	}
	if slices.Contains(trace, "sig:4321:killed") {
		t.Fatalf("a process released by HUP must not be SIGKILLed: %v", trace)
	}
}

// AC-04: 점유가 남으면 레코드를 보존한 채 workspace_processes_stop에서 멈추고,
// 그 failure receipt는 codec을 통과해 다시 읽힌다.
func TestCleanupFinishApplyFailsClosedWithWorkspaceProcessesStop(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123"}
	world := occupiedWorld(t, codexOccupant()) // never dies
	deps := finishDeps(git)
	deps.Processes = worldCleanupProcesses(world)

	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("surviving occupants must fail the stop step: err=%v result=%+v", err, result)
	}
	if git.removedWorktree || git.deletedBranch {
		t.Fatalf("git steps must not run after a failed stop: %+v", git)
	}
	if !strings.Contains(result.NextCommand, "--preview") {
		t.Fatalf("recovery must go through a fresh preview: %q", result.NextCommand)
	}
	kept, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatalf("record with the new failure step must stay readable: %v", err)
	}
	if kept.CleanupFinishFailure == nil || kept.CleanupFinishFailure.Step != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("failure receipt must name the stop step: %+v", kept.CleanupFinishFailure)
	}
}

// AC-05: 요청자 보호는 거부다. 자기 조상이 점유하면 preview가 막는다.
func TestCleanupFinishRequesterGatesRefuse(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	shell := issueops.CleanupWorkspaceProcess{PID: 700, Command: "zsh", StartedAt: "2026-08-27T00:00:00Z", Executable: "zsh"}
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	deps.Processes = worldCleanupProcesses(occupiedWorld(t, shell))
	result, _ := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if !containsString(result.Missing, "requester_occupies_worktree") {
		t.Fatalf("own shell occupancy must refuse, not exclude: %v", result.Missing)
	}
}

func TestCleanupFinishRequesterGatesRefuseSourceCheckout(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	// codec은 Execution.Workspace.Root == SourceRoot를 저장 단계에서 거부하므로,
	// 이 게이트에 닿는 것은 execution 없이 WorktreePath만 연결된 레코드다.
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = nil
		rec.WorktreePath = rec.Repo
	})
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err == nil || !containsString(result.Missing, "worktree_is_source_checkout") {
		t.Fatalf("the source checkout is never a cleanup target: err=%v missing=%v", err, result.Missing)
	}
}

func TestCleanupWorkspaceGatesRejectSourceCheckoutSymlinkAlias(t *testing.T) {
	t.Parallel()

	sourceParent := t.TempDir()
	source := filepath.Join(sourceParent, "source")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	aliasParent := t.TempDir()
	aliasLink := filepath.Join(aliasParent, "workspace")
	if err := os.Symlink(sourceParent, aliasLink); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(aliasLink, "source")
	record := issueops.IssueOpsRecord{Repo: source}
	_, missing := cleanupWorkspaceGatesForRecord(record, alias, quietCleanupProcesses())
	if !containsString(missing, "worktree_is_source_checkout") {
		t.Fatalf("a symlink alias of the source checkout must refuse: %v", missing)
	}
}

func TestCleanupWorkspaceGatesRejectUnresolvedSourceIdentity(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink("loop", loop); err != nil {
		t.Fatal(err)
	}
	record := issueops.IssueOpsRecord{Repo: source}
	_, missing := cleanupWorkspaceGatesForRecord(record, loop, quietCleanupProcesses())
	if !containsString(missing, "worktree_is_source_checkout") {
		t.Fatalf("an unresolvable source identity must fail closed: %v", missing)
	}
}

// AC-07: 결과와 감사 라인에 무엇을 종료했는지 남는다.
func TestCleanupFinishReportsStoppedProcessesAndAudit(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	world := occupiedWorld(t, codexOccupant())
	world.diesOn[4321] = syscall.SIGTERM
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	deps.Processes = worldCleanupProcesses(world)
	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil || result.Audit == "" {
		t.Fatalf("apply must succeed and report the audit line: err=%v result=%+v", err, result)
	}
	if len(result.WorkspaceProcessesStopped) != 1 {
		t.Fatalf("stopped inventory must be reported: %+v", result)
	}
	if !strings.Contains(result.Audit, "stopped=1") {
		t.Fatalf("audit line must record the stop counts: %q", result.Audit)
	}
}
