package issueops

import (
	"context"
	reviewcontract "issueops/internal/contract/issueopsreview"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/preflight"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

type claimableExecutionFixture struct {
	record    issueops.IssueOpsRecord
	worktree  string
	tokenPath string
}

func executionActor(host, sessionID string) issueops.NativeActor {
	receipt, err := ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		panic(err)
	}
	return issueops.NativeActor{
		Host: host, SessionID: sessionID, SessionProcess: &receipt,
		ProcessAncestry: []issueops.NativeProcessReceipt{receipt},
	}
}

func newClaimableExecutionFixture(t *testing.T, stateRoot, branch string) claimableExecutionFixture {
	t.Helper()
	repo := initIssueOpsRepo(t)
	worktree := issueOpsWorktreePathForTest(repo, branch)
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", "-b", branch, worktree, "main"); code != 0 {
		t.Fatalf("git worktree add: %s", stderr)
	}
	baseHead := strings.TrimSpace(preflight.GitOut(worktree, "rev-parse", "HEAD"))
	record, err := startIssueOpsFixture(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.WorktreePath = worktree
	record.BranchPrepare = &issueops.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: "https://github.com/example/issueops/issues/69",
		Branch: branch, BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	record.Execution = &issueops.Execution{
		Workspace: issueops.Workspace{
			SourceRoot: repo, Root: worktree, Branch: branch, BaseHead: baseHead,
			LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusClaimable},
	}
	token, tokenPath, err := createClaimToken(record)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Lease.ClaimTokenSHA256 = tokenSHA256(token)
	if _, err := writeIssueOps(context.Background(), stateRoot, record); err != nil {
		t.Fatal(err)
	}
	return claimableExecutionFixture{record: record, worktree: worktree, tokenPath: tokenPath}
}

func prepareExecutionCompletionFixture(t *testing.T, stateRoot string, fixture *claimableExecutionFixture) {
	t.Helper()
	fixture.record.Phase = issueops.IssueOpsPhasePR
	fixture.record.IssueURL = "https://github.com/example/issueops/issues/69"
	fixture.record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/example/issueops/pull/69",
		Labels: []string{"enhancement"}, Assignees: []string{"maintainer"},
		VerifiedAt: "2026-07-22T00:00:00Z", TargetBranch: "main",
	}
	if _, err := writeIssueOps(context.Background(), stateRoot, fixture.record); err != nil {
		t.Fatal(err)
	}
}

func initIssueOpsRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	remote := t.TempDir()
	if code, _, stderr := preflight.GitCmd(remote, "init", "--bare", "-q"); code != 0 {
		t.Fatalf("git init bare failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "init", "-q", "-b", "main"); code != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	for _, args := range [][]string{
		{"config", "user.name", "IssueOps Test"},
		{"config", "user.email", "issueops@example.test"},
		{"remote", "add", "origin", remote},
	} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeIssueOpsFile(t, repo, "README.md", "readme\n")
	writeIssueOpsFile(t, repo, "plans/demo.md", planBodyForTest())
	writeIssueOpsFile(t, repo, ".issueops/CAUTIONS.md", "# cautions\n")
	if code, _, stderr := preflight.GitCmd(repo, "add", "README.md", "plans/demo.md", ".issueops/CAUTIONS.md"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "-m", "initial"); code != 0 {
		t.Fatalf("git commit failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", "main"); code != 0 {
		t.Fatalf("git push failed: %s", stderr)
	}
	return repo
}

func issueOpsWorktreePathForTest(repo, slug string) string {
	return filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", slug)
}

func makeIssueOpsWorktreeDirForTest(t *testing.T, repo, slug string) string {
	t.Helper()
	worktree := issueOpsWorktreePathForTest(repo, slug)
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/"+slug+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return worktree
}

// planBodyForTest는 link-plan이 요구하는 네 필수 절을 모두 가진 최소 계획 본문이다.
func planBodyForTest() string {
	return "# plan\n" + strings.Join(issueopsdomain.RequiredPlanSections, "\n본문\n") + "\n본문\n"
}

func writeIssueOpsFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsIntentForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	if _, err := RecordIssueOpsIntent(stateRoot, id, issueops.IssueOpsIntentRecordRequest{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
	}); err != nil {
		t.Fatal(err)
	}
}

func setIssueOpsPlanPrepForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	rec, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	waived := issueops.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "lifecycle fixture"}
	rec.PlanPrep = &issueops.IssueOpsPlanPrep{
		PriorDecisions: waived,
		RelatedIssues:  waived,
		WebResearch:    waived,
		CodebaseSurvey: waived,
	}
	if _, err := writeIssueOps(context.Background(), stateRoot, rec); err != nil {
		t.Fatal(err)
	}
}

// recordIssueOpsGrillArtifactsForTest satisfies the grill completion gate
// (split_decision + domain_review) so lifecycle tests can still advance past the
// grill->plan boundary. split_decision is recorded as a no-split scope decision.
func recordIssueOpsGrillArtifactsForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	if _, err := addIssueOpsDecisionForTest(stateRoot, id, issueops.IssueOpsDecisionRecordRequest{
		Title: "no split",
		Body:  "single focused work item; no provider-native child tasks needed",
		Kind:  "scope",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordIssueOpsDomainReview(stateRoot, id, issueops.IssueOpsDomainReviewRequest{
		ModelFit:          "change fits the existing IssueOps phase model",
		Terminology:       []string{"phase ledger", "completion gate"},
		OpenUncertainties: []string{"none blocking"},
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsApprovedArchitectureForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	recordIssueOpsGrillArtifactsForTest(t, stateRoot, id)
	setIssueOpsPlanPrepForTest(t, stateRoot, id)
	if _, err := RecordIssueOpsArchitectureReview(stateRoot, id, reviewcontract.ArchitectureReviewRequest{
		ProblemSummary:       "IssueOps must preserve the work contract",
		ProposedArchitecture: "Gate implementation on a reviewed design contract",
		RefactorPlan:         "Keep IssueOps state and adapter changes scoped to the active cycle",
		Alternatives:         []string{"documentation-only guidance"},
		Risks:                []string{"existing tests must create explicit design evidence"},
		Verification:         []string{"architecture review checked alternatives and risks", "go test ./internal/core/issueops"},
		Approved:             true,
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsPreparedExecutionForTest(t *testing.T, stateRoot, id, worktree string) issueops.IssueOpsRecord {
	t.Helper()
	recordIssueOpsCompatibilityReviewForTest(t, stateRoot, id)
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = issueOpsExecutionForTest(record.Repo, worktree, record.Branch)
	sealStagedArchitectureForTest(t, stateRoot, record)
	record, err = writeIssueOps(context.Background(), stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	record, err = AdvanceIssueOpsPhaseWithActor(stateRoot, id, string(issueops.IssueOpsPhaseImplement), issueOpsActorForTest(worktree))
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func recordIssueOpsCompatibilityReviewForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	if _, err := RecordIssueOpsCompatibilityReview(stateRoot, id, reviewcontract.CompatibilityReviewRequest{
		BackwardCompatibility: []string{"existing IssueOps state records remain readable"},
		SideEffects:           []string{"phase order changes are limited to IssueOps lifecycle readiness"},
		RollbackPlan:          "Revert the compatibility-review phase and readiness gate.",
		Verification:          []string{"compatibility review checked backward compatibility and side effects", "go test ./internal/core/issueops"},
		Approved:              true,
	}); err != nil {
		t.Fatal(err)
	}
	// The devil's-advocate verdict is a fail-closed implement-entry gate, so bring
	// the cycle to implement-readiness with a pass verdict alongside compatibility.
	if _, err := RecordIssueOpsDevilsAdvocateReview(stateRoot, id, reviewcontract.DevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "subagent", Findings: []string{"attacked gate 3: no second caller exists"}}); err != nil {
		t.Fatal(err)
	}
}

func issueOpsIntentContractForTest() *issueops.IssueOpsIntentContract {
	return &issueops.IssueOpsIntentContract{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
		RecordedAt:        "2026-06-05T00:00:00Z",
	}
}

func issueOpsCompatibilityReviewForTest() *reviewcontract.CompatibilityReview {
	return &reviewcontract.CompatibilityReview{
		BackwardCompatibility: []string{"existing IssueOps state records remain readable"},
		SideEffects:           []string{"phase order changes are limited to IssueOps lifecycle readiness"},
		RollbackPlan:          "Revert the compatibility-review phase and readiness gate.",
		Verification:          []string{"compatibility review checked backward compatibility and side effects", "go test ./internal/core/issueops"},
		Approved:              true,
		ReviewedAt:            "2026-06-26T00:00:00Z",
	}
}

func issueOpsDevilsAdvocateReviewForTest() *issueops.IssueOpsDevilsAdvocateReview {
	return &issueops.IssueOpsDevilsAdvocateReview{
		Verdict:         "pass",
		Findings:        []string{"attacked gate 3: no second caller exists"},
		ReviewerPattern: "devils-advocate-review",
		ReviewerContext: "subagent",
		RecordedAt:      "2026-06-29T00:00:00Z",
	}
}

func issueOpsExecutionForTest(repo, worktree, branch string) *issueops.Execution {
	return &issueops.Execution{
		Workspace: issueops.Workspace{
			SourceRoot: repo,
			Root:       worktree,
			Branch:     branch,
			BaseHead:   "0123456789012345678901234567890123456789",
			LinkedAt:   "2026-07-22T00:00:00Z",
		},
		Lease: issueops.WriteLease{
			Generation: 1,
			Status:     issueops.LeaseStatusActive,
			Holder: &issueops.NativeActor{
				Host:           "codex",
				SessionID:      "test-session",
				AgentID:        "test-agent",
				SessionProcess: func() *issueops.NativeProcessReceipt { receipt := liveFixtureReceipt(); return &receipt }(),
			},
			ClaimedAt: "2026-07-22T00:00:00Z",
		},
	}
}

func issueOpsActorForTest(worktree string) issueops.IssueOpsActor {
	return issueops.IssueOpsActor{
		Host: "codex", SessionID: "test-session", AgentID: "test-agent", CWD: worktree,
		NativeProcessAncestry: []issueops.NativeProcessReceipt{liveFixtureReceipt()},
	}
}

func startIssueOpsChildForTest(stateRoot string, parent issueops.IssueOpsRecord, req issueops.IssueOpsChildStartRequest) (issueops.IssueOpsChildStartResult, error) {
	return StartIssueOpsChildWithActor(stateRoot, req, issueOpsActorForTest(parent.WorktreePath))
}

func acceptIssueOpsChildForTest(stateRoot string, parent issueops.IssueOpsRecord, childID string, evidence []string) (issueops.IssueOpsChildValidationResult, error) {
	return AcceptIssueOpsChildWithActor(stateRoot, parent.ID, childID, evidence, issueOpsActorForTest(parent.WorktreePath))
}

func rejectIssueOpsChildForTest(stateRoot string, parent issueops.IssueOpsRecord, childID, reason string, evidence []string) (issueops.IssueOpsChildValidationResult, error) {
	return RejectIssueOpsChildWithActor(stateRoot, parent.ID, childID, reason, evidence, issueOpsActorForTest(parent.WorktreePath))
}

func dropIssueOpsChildForTest(stateRoot string, parent issueops.IssueOpsRecord, childID, reason string) (issueops.IssueOpsChildValidationResult, error) {
	return DropIssueOpsChildWithActor(stateRoot, parent.ID, childID, reason, issueOpsActorForTest(parent.WorktreePath))
}

func executionPrepareRecord(t *testing.T) (string, issueops.IssueOpsRecord) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "16-demo"
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	if code, _, stderr := preflight.GitCmd(repo, "update-ref", "refs/remotes/origin/"+branch, baseHead); code != 0 {
		t.Fatalf("create remote branch fixture: %s", stderr)
	}
	record := issueops.IssueOpsRecord{
		OK: true, SchemaVersion: issueops.IssueOpsSchemaVersion,
		ID: newIssueOpsID(repo, branch), Repo: repo, Branch: branch, Phase: issueops.IssueOpsPhasePlan,
		IssueURL:           "https://github.com/acme/repo/issues/16",
		ArchitectureReview: &reviewcontract.ArchitectureReview{Approved: true, ReviewedAt: "2026-07-11T00:00:00Z"},
		BranchPrepare: &issueops.IssueOpsBranchPrepare{
			Provider: "github", IssueURL: "https://github.com/acme/repo/issues/16", Branch: branch,
			BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true, CreatedAt: "2026-07-11T00:00:00Z",
		},
		CreatedAt: "2026-07-11T00:00:00Z", UpdatedAt: "2026-07-11T00:00:00Z",
	}
	written, err := writeIssueOps(context.Background(), stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, written
}

func issueOpsArchitectureReviewForTest() *reviewcontract.ArchitectureReview {
	return &reviewcontract.ArchitectureReview{
		ProblemSummary:       "IssueOps must preserve the work contract",
		ProposedArchitecture: "Gate implementation on a reviewed design contract",
		RefactorPlan:         "Keep IssueOps state and adapter changes scoped to the active cycle",
		Alternatives:         []string{"documentation-only guidance"},
		Risks:                []string{"existing tests must create explicit design evidence"},
		Verification:         []string{"architecture review checked alternatives and risks", "go test ./internal/core/issueops"},
		Approved:             true,
		// 이 fixture는 digest 게이트가 관심사가 아니므로 부모 승계 표시로 그 게이트를 건너뛴다.
		ReviewerContext: issueopsdomain.ParentReviewPattern,
		ReviewedAt:      "2026-06-05T00:00:00Z",
	}
}

func issueOpsWeakApprovedArchitectureReviewForTest() *reviewcontract.ArchitectureReview {
	return &reviewcontract.ArchitectureReview{
		ProblemSummary:       "IssueOps must preserve the work contract",
		ProposedArchitecture: "Gate implementation on a reviewed design contract",
		Verification:         []string{"go test ./internal/core/issueops"},
		Approved:             true,
		ReviewedAt:           "2026-06-05T00:00:00Z",
	}
}

// quiescentWorkspaceInspector는 "워크트리를 점유한 프로세스가 없다"는 결정적
// 관측이다. 기본 구현은 시스템 전역 lsof라 호스트의 열린 파일 목록과 3초
// 프로브 상한에 묶이고, 전체 스위트가 패키지를 병렬로 돌리면 그 상한을 넘겨
// lease 테스트가 깨진다. lease 상태 기계만 검증하는 테스트는 이 관측자를 쓴다.
func quiescentWorkspaceInspector() executionWorkspaceProcessInspector {
	return func(string, map[int]bool) ([]workspaceProcess, error) {
		return nil, nil
	}
}

// sealStagedArchitectureForTest는 execution prepare가 하는 봉인을 fixture에서 흉내 낸다.
// 스테이징된 architecture가 있을 때만 봉인 디렉터리를 정하고 파일을 쓴다.
func sealStagedArchitectureForTest(t *testing.T, stateRoot string, record issueops.IssueOpsRecord) {
	t.Helper()
	staged, err := readStagedArtifacts(stateRoot, record.ID)
	if err != nil || staged["architecture"] == "" {
		return
	}
	if record.Execution.Workspace.ArtifactDir == "" {
		record.Execution.Workspace.ArtifactDir = ".issueops/issues/1/artifact"
	}
	if err := writeSealedArtifact(record.Execution.Workspace.Root, sealedArtifactPath(record, record.Execution.Workspace.Root, "architecture"), []byte(staged["architecture"])); err != nil {
		t.Fatal(err)
	}
}
