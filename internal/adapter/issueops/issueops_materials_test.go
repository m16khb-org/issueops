package issueops

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"issueops/internal/adapter/preflight"
	"issueops/internal/contract/issueops"
	"issueops/internal/domain/artifactreadability"
)

// tracked copies live next to gates.md; the sealed originals stay under the
// ignored artifact/ directory.
func trackedCopy(t *testing.T, worktree, issue, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(worktree, ".issueops", "issues", issue, name))
	if err != nil {
		return ""
	}
	return string(data)
}

func materialsCycleForTest(t *testing.T) (string, issueops.IssueOpsRecord, string) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "13-materials"
	for _, args := range [][]string{{"checkout", "-q", "-b", branch}, {"push", "-q", "-u", "origin", branch}, {"checkout", "-q", "main"}} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v: %s", args, stderr)
		}
	}
	worktree := issueOpsWorktreePathForTest(repo, "materials")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	if _, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/13"); err != nil {
		t.Fatal(err)
	}
	if _, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider: "github", IssueURL: "https://github.com/example/repo/issues/13", Branch: branch, BaseBranch: "main", LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	// Both prepare paths point plan_path at the sealed plan (direct:
	// issueops_artifact_stage.go, Orca: execution_prepare_bridge.go).
	sealedPlan := filepath.Join(worktree, ".issueops", "issues", "13", "artifact", "plan.md")
	writeIssueOpsFile(t, worktree, ".issueops/issues/13/artifact/plan.md", planBodyForTest())
	if _, err = LinkIssueOpsPlan(stateRoot, record.ID, sealedPlan); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsCompatibilityReviewForTest(t, stateRoot, record.ID)
	record, err = ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = issueOpsExecutionForTest(record.Repo, worktree, record.Branch)
	record.Execution.Workspace.ArtifactDir = ".issueops/issues/13/artifact"
	if record, err = writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, worktree
}

func TestPhaseTransitionWritesTrackedMaterials(t *testing.T) {
	stateRoot, record, worktree := materialsCycleForTest(t)
	_, materials, err := AdvanceIssueOpsPhaseWithActorReport(stateRoot, record.ID, string(IssueOpsPhaseImplement), issueOpsActorForTest(worktree))
	if err != nil {
		t.Fatalf("implement: %v", err)
	}
	for _, name := range []string{"plan.md", "intent.md", "plan-review.md"} {
		if trackedCopy(t, worktree, "13", name) == "" {
			t.Fatalf("implement entry must write the tracked %s: %+v", name, materials)
		}
		if !slices.Contains(materials.Written, ".issueops/issues/13/"+name) {
			t.Fatalf("the report must list %s: %+v", name, materials)
		}
	}
	if trackedCopy(t, worktree, "13", "plan.md") != planBodyForTest() {
		t.Fatalf("plan.md must copy the sealed plan")
	}
	if info, err := os.Stat(filepath.Join(worktree, ".issueops", "issues", "13", "plan.md")); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("tracked copies are ordinary files: %v %v", info, err)
	}
	intent := trackedCopy(t, worktree, "13", "intent.md")
	review := trackedCopy(t, worktree, "13", "plan-review.md")

	writeIssueOpsFile(t, worktree, "internal/demo.go", "package demo\nconst Value = 1\n")
	_, again, err := AdvanceIssueOpsPhaseWithActorReport(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean), issueOpsActorForTest(worktree))
	if err != nil {
		t.Fatalf("ai-slop-clean: %v", err)
	}
	if len(again.Written) != 0 || trackedCopy(t, worktree, "13", "intent.md") != intent || trackedCopy(t, worktree, "13", "plan-review.md") != review {
		t.Fatalf("unchanged materials must not be rewritten: %+v", again)
	}

	// An Orca cycle passes the same two transitions; its prepare bridge points
	// plan_path at the sealed plan the same way, so the copies are the same.
	orca, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	orca.Execution.Mode = issueops.ExecutionModeOrca
	orcaRoot := t.TempDir()
	orca.Execution.Workspace.Root = orcaRoot
	orca.PlanPath = filepath.Join(orcaRoot, ".issueops", "issues", "13", "artifact", "plan.md")
	writeIssueOpsFile(t, orcaRoot, ".issueops/issues/13/artifact/plan.md", planBodyForTest())
	orcaMaterials := writeTrackedMaterials(orca)
	for _, name := range []string{"plan.md", "intent.md", "plan-review.md"} {
		if trackedCopy(t, orcaRoot, "13", name) == "" {
			t.Fatalf("an Orca cycle gets the same tracked %s: %+v", name, orcaMaterials)
		}
	}
}

// 구현 메모 A: plan_path가 봉인 디렉터리 밖(이미 추적되는 파일)이면 plan.md 사본을
// 만들지 않는다. 추적 중인 plan.md를 덮어쓰지 않기 위해서다.
func TestTrackedPlanCopySkipsAPlanOutsideTheSealedDirectory(t *testing.T) {
	root := t.TempDir()
	record := issueops.IssueOpsRecord{
		IssueURL:  "https://github.com/example/repo/issues/13",
		PlanPath:  filepath.Join(root, ".issueops", "issues", "13", "plan.md"),
		Execution: &issueops.Execution{Workspace: issueops.Workspace{Root: root, ArtifactDir: ".issueops/issues/13/artifact"}},
	}
	writeIssueOpsFile(t, root, ".issueops/issues/13/plan.md", "사람이 추적하는 계획\n")
	writeIssueOpsFile(t, root, ".issueops/issues/13/artifact/plan.md", "봉인 계획\n")
	materials := writeTrackedMaterials(record)
	if got := trackedCopy(t, root, "13", "plan.md"); got != "사람이 추적하는 계획\n" || slices.Contains(materials.Written, ".issueops/issues/13/plan.md") {
		t.Fatalf("a tracked plan must not be overwritten: %q %+v", got, materials)
	}

	noIssue := writeTrackedMaterials(issueops.IssueOpsRecord{Execution: &issueops.Execution{Workspace: issueops.Workspace{Root: root}}})
	if len(noIssue.Written) != 0 || len(noIssue.Warnings) != 1 || !strings.Contains(noIssue.Warnings[0], "tracked_materials_skipped") {
		t.Fatalf("without an issue number nothing is written and the skip is reported: %+v", noIssue)
	}
}

// 이 변경 전에 구현에 들어간 사이클은 사본이 없다. 정리 단계가 그 사실을 알린다.
func TestTrackedMaterialsMissingWarning(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution.Workspace.ArtifactDir = ".issueops/issues/80/artifact"
	})
	writeIssueOpsFile(t, worktree, ".issueops/issues/80/artifact/plan.md", "봉인 계획\n")
	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(&fakeFinishGit{branchOID: "abc123"}))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(preview.Warnings, func(w string) bool { return strings.HasPrefix(w, "tracked_materials_missing") }) {
		t.Fatalf("finish preview must warn about the missing tracked plan: %+v", preview.Warnings)
	}

	stateRoot2, record2 := completionTestRecord(t)
	root := t.TempDir()
	mutateFinishRecord(t, stateRoot2, record2.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = &issueops.Execution{
			Mode:      issueops.ExecutionModeDirect,
			Workspace: issueops.Workspace{SourceRoot: rec.Repo, Root: root, Branch: "81-completion", BaseHead: "deadbeef", Driver: "git", LinkedAt: "2026-07-24T00:00:00Z", ArtifactDir: ".issueops/issues/81/artifact"},
			Lease:     issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased},
		}
	})
	writeIssueOpsFile(t, root, ".issueops/issues/81/artifact/plan.md", "봉인 계획\n")
	_, _, report, err := ReflectIssueCompletion(stateRoot2, record2.ID, readableResult, true, false, &fakeCompletionProvider{updateRes: portUpdateResult(false)})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(report.Warnings, func(f artifactreadability.Finding) bool { return f.Code == "tracked_materials_missing" }) {
		t.Fatalf("reflect-completion must warn about the missing tracked plan: %+v", report.Warnings)
	}
}
