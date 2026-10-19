package issueopscli

import (
	"context"
	issueopscore "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/testsupport"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func captureStdoutForContract(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func captureProjectCLIStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	return testsupport.CaptureStderrAndError(t, fn)
}

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}

func recordIssueOpsCLIIntentForTest(t *testing.T, id string) {
	t.Helper()
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"intent", "record",
			"--id", id,
			"--raw-request", "refactor issueops flow",
			"--interpreted-intent", "keep intent and design evidence before implementation",
			"--success-criteria", "intent is recorded",
			"--success-criteria", "design is reviewed",
			"--json",
		})
	})
}

// stageArchitectureForCLITest는 필수 절을 채운 아키텍처 문서를 스테이징한다. 이미 스테이징돼
// 있거나 봉인된 뒤라면 CLI가 돌려주는 오류를 무시한다.
func stageArchitectureForCLITest(t *testing.T, id string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "architecture.md")
	body := strings.Join(issueopsdomain.RequiredArchitectureSections, "\n내용\n") + "\n내용\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"artifact", "stage", "--id", id, "--name", "architecture", "--file", path, "--json"})
	})
}

func recordIssueOpsCLIArchitectureForTest(t *testing.T, id string) {
	t.Helper()
	stageArchitectureForCLITest(t, id)
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"architecture", "review",
			"--id", id,
			"--problem-summary", "IssueOps must preserve the work contract",
			"--proposed-architecture", "Gate implementation on a reviewed design contract",
			"--refactor-plan", "Keep IssueOps state and adapter changes scoped to the active cycle",
			"--alternative", "documentation-only guidance",
			"--risk", "existing tests must create explicit design evidence",
			"--verification", "architecture review checked alternatives and risks",
			"--verification", "go test ./cmd/issueops/issueopscli",
			"--reviewer-context", "subagent",
			"--approved",
			"--json",
		})
	})
}

func recordIssueOpsCoreIntentForCLITest(t *testing.T, id string) {
	t.Helper()
	if _, err := planningRecorderForTest(nil).Intent(issueOpsStateRootForTest(), id, issueopscontract.IssueOpsIntentRecordRequest{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsCLIPlanPrepForTest(t *testing.T, id string) {
	t.Helper()
	waived := issueopscontract.IssueOpsPlanPrepItemRequest{WaiveReason: "cli lifecycle test"}
	if _, err := planningRecorderForTest(nil).PlanPrep(issueOpsStateRootForTest(), id, issueopscontract.IssueOpsPlanPrepRequest{
		PriorDecisions: waived,
		RelatedIssues:  waived,
		WebResearch:    waived,
		CodebaseSurvey: waived,
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsCoreArchitectureForCLITest(t *testing.T, id string) {
	t.Helper()
	stageArchitectureForCLITest(t, id)
	if _, err := planningRecorderForTest(nil).Architecture(issueOpsStateRootForTest(), id, reviewcontract.ArchitectureReviewRequest{
		ProblemSummary:       "IssueOps must preserve the work contract",
		ProposedArchitecture: "Gate implementation on a reviewed design contract",
		RefactorPlan:         "Keep IssueOps state and adapter changes scoped to the active cycle",
		Alternatives:         []string{"documentation-only guidance"},
		Risks:                []string{"existing tests must create explicit design evidence"},
		Verification:         []string{"architecture review checked alternatives and risks", "go test ./cmd/issueops/issueopscli"},
		Approved:             true,
		ReviewerContext:      "subagent",
	}); err != nil {
		t.Fatal(err)
	}
}

func seedIssueOpsCLIExecution(t *testing.T, record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, issueopscontract.IssueOpsActor) {
	t.Helper()
	baseHead := strings.TrimSpace(record.BranchPrepare.BaseSHA)
	if len(baseHead) != 40 {
		baseHead = strings.Repeat("a", 40)
	}
	const now = "2026-07-22T00:00:00Z"
	actor := issueopscontract.IssueOpsActor{Host: "codex", SessionID: "issueops-cli-test", AgentID: "test-agent", CWD: record.WorktreePath}
	receipt, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	actor.NativeProcessAncestry = []issueopscontract.NativeProcessReceipt{receipt}
	// 이 fixture는 execution prepare 없이 Execution을 만들어 봉인된 architecture가 없다. 비git
	// 워크트리에서는 봉인 파일이 구현 변경으로 잡히므로, 봉인 digest 게이트는 부모 승계 표시로 건너뛴다.
	if record.ArchitectureReview != nil {
		record.ArchitectureReview.ReviewerContext = issueopsdomain.ParentReviewPattern
	}
	record.Execution = &issueopscontract.Execution{
		Workspace: issueopscontract.Workspace{
			SourceRoot: record.Repo, Root: record.WorktreePath, Branch: record.Branch,
			BaseHead: baseHead, LinkedAt: now,
		},
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusActive, ClaimedAt: now,
			Holder: &issueopscontract.NativeActor{
				Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID,
				SessionProcess: &receipt,
			},
		},
	}
	written, err := (issueopscore.CycleRecordStore{StateRoot: issueOpsStateRootForTest()}).Save(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	return written, actor
}

func withIssueOpsCLIActor(args []string, actor issueopscontract.IssueOpsActor) []string {
	return append(args,
		"--host", actor.Host,
		"--session-id", actor.SessionID,
		"--agent-id", actor.AgentID,
		"--cwd", actor.CWD,
	)
}
