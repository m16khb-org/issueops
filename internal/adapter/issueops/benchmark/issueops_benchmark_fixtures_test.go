package benchmark

import (
	issueopscontract "issueops/internal/contract/issueops"
	"strings"
	"testing"
)

func TestScoreIssueOpsBenchmarkArtifactAcceptsKoreanSectionLabels(t *testing.T) {
	fixture := issueopscontract.IssueOpsBenchmarkFixture{ID: "korean-sections", CriticalFailures: []string{"works in source repo"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.IssueDraft = "## 요약\n\n캐시 미적용으로 동일 입력에 외부 LLM을 반복 호출한다.\n\n## 배경\n\n현재 호출 로그.\n\n## 수용 기준\n\n동일 입력은 캐시 적중한다.\n\n## 범위\n\n캐시 저장과 wrapper 호출부만 바꾼다.\n\n## 검증\n\ngo test ./... -count=1\n\nGuideline: skills/issueops-create-issue/SKILL.md; skills/issueops-create-pr/SKILL.md\n"
	artifact.PRDraft = "## 요약\n\n이슈의 캐시 요구사항을 충족한다.\nIssue: https://example.com/acme/issueops/issues/1\n\n## 변경 내용\n\n캐시 저장소 추가.\n\n## 확인한 것\n\ngo test ./... -count=1로 캐시 적중을 확인했다.\n\n## 리뷰 포인트\n\n캐시 키가 입력을 모두 반영하는지 봐 주세요.\n\nGuideline: skills/issueops-create-issue/SKILL.md; skills/issueops-create-pr/SKILL.md\n"

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if !score.Passed {
		t.Fatalf("Korean-only section labels should pass deterministic scoring: %+v", score)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresCanonicalRemoteIssueSections(t *testing.T) {
	fixture := issueopscontract.IssueOpsBenchmarkFixture{ID: "canonical-issue", CriticalFailures: []string{"works in source repo"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.IssueDraft = strings.ReplaceAll(artifact.IssueDraft, "## 범위\n\ncore renderer, CLI, MCP schema를 갱신한다.\n\n", "")

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if score.Passed {
		t.Fatalf("issue quality must fail when canonical scope section is missing: %+v", score)
	}
	if row := adequacyRow(score, "issue_quality"); row.Score != 0 {
		t.Fatalf("issue_quality row must drop, got %+v", row)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresCanonicalPRSections(t *testing.T) {
	fixture := issueopscontract.IssueOpsBenchmarkFixture{ID: "canonical-pr", CriticalFailures: []string{"works in source repo"}}
	artifact := completeBenchmarkArtifactForTest()
	artifact.PRDraft = strings.ReplaceAll(artifact.PRDraft, "## 리뷰 포인트\n\n리뷰 포인트\n\n", "")

	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if score.Passed {
		t.Fatalf("PR/MR quality must fail when canonical reviewer focus section is missing: %+v", score)
	}
	if row := adequacyRow(score, "pr_mr_quality"); row.Score != 0 {
		t.Fatalf("pr_mr_quality row must drop, got %+v", row)
	}
}

func TestScoreIssueOpsBenchmarkArtifactRequiresPRChangesAndVerifiedSections(t *testing.T) {
	fixture := issueopscontract.IssueOpsBenchmarkFixture{ID: "canonical-pr-changes-verified", CriticalFailures: []string{"works in source repo"}}
	for _, tc := range []struct {
		name   string
		remove string
	}{
		{"changes", "## 변경 내용\n\n변경 내용\n\n"},
		{"verified", "## 확인한 것\n\n확인한 것\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			artifact := completeBenchmarkArtifactForTest()
			artifact.PRDraft = strings.ReplaceAll(artifact.PRDraft, tc.remove, "")

			score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
			if score.Passed {
				t.Fatalf("PR/MR quality must fail when %s section is missing: %+v", tc.name, score)
			}
			if row := adequacyRow(score, "pr_mr_quality"); row.Score != 0 {
				t.Fatalf("pr_mr_quality row must drop, got %+v", row)
			}
		})
	}
}

func completeBenchmarkArtifactForTest() issueopscontract.IssueOpsBenchmarkArtifact {
	return issueopscontract.IssueOpsBenchmarkArtifact{
		ProblemSummary:         "The request needs measurable IssueOps quality gates before prompt optimization.\n선택 라벨: enhancement(score 0.90), 거절 라벨: documentation(score 0.20), threshold 0.70, 수동 override 없음.\n",
		IssueDraft:             "## 요약\n\n요약\n\n## 배경\n\n배경\n\n## 완료 기준\n\n완료 기준\n\n## 범위\n\ncore renderer, CLI, MCP schema를 갱신한다.\n\n## 검증\n\n검증\n\nGuideline: skills/issueops-create-issue/SKILL.md; skills/issueops-create-pr/SKILL.md\n",
		Plan:                   "Run: go test ./... -count=1\n",
		TDDPlan:                "Write failing test before implementation.\n",
		TaskBreakdown:          "Worker A owns internal/core/issueops_benchmark.go. Worker B owns cmd/issueops/issueops.go. Large issue is unsafe as one work item because one issue would hide risky behavior changes. It uses provider-native child work items and records them with issueops link-child. Execution order: Wave 1 [p] benchmark fixture update is parallelizable with prompt documentation. Wave 2 [s] scorer wiring is sequential and depends on the fixture prerequisite.",
		SubagentPrompts:        "You are not alone in the codebase. Do not revert others. Own internal/core only. Expected output: tests and implementation. Before work, report pwd, branch, HEAD, and worktree path; stop on mismatch. For narrow review, use verifier or direct bounded review. If code-reviewer is required, do not spawn subagents and use a 5 minute time budget.",
		PRDraft:                "## 요약\n\n요약\nIssue: https://example.com/acme/issueops/issues/1\n\n## 변경 내용\n\n변경 내용\n\n## 확인한 것\n\n확인한 것\n\n## 리뷰 포인트\n\n리뷰 포인트\n\nGuideline: skills/issueops-create-issue/SKILL.md; skills/issueops-create-pr/SKILL.md\n",
		GuidelineRef:           "skills/issueops-create-issue/SKILL.md; skills/issueops-create-pr/SKILL.md",
		PhaseChoices:           "Proceed to plan | revise current phase | jump to issue | pause",
		BranchName:             "feature/1-issueops-quality-benchmark",
		WorktreePath:           "/repo.worktrees/feature-1-issueops-quality-benchmark",
		ImplementationLocation: "/repo.worktrees/feature-1-issueops-quality-benchmark",
		WorktreeCleanup:        "clean worktree; cleanup choices offered after merge",
		DomainContractEvidence: "Invariant: preserve the user-visible contract. Exact mechanism: compare the documented mechanism with source file:line evidence. Equivalent behavior: record when another path enforces the same invariant. Source: internal/core/example.go:12.",
		APIDocGateEvidence:     "Changed endpoint list is reviewed. Public error responses are mapped. Static check: api_doc_static_check. Review: api_doc_review for OpenAPI/Swagger/API doc parity.",
		LiveEvidenceMatrix:     "Environment matrix covers dev, stg, and prod. Repo config evidence is compared with runtime evidence. Remediation order is recorded before edits.",
		ReviewFeedbackEvidence: "Classification: valid defect from Kodus or Gemini Code Assist review-agent feedback. Verification: command and file:line evidence. Thread reply: posted with verdict. Resolution: resolveReviewThread/resolved=true re-checked after fix.",
		CompletionHygiene:      "Draft issue completion record stored with final diff, evidence, labels, children, PR URL, and unresolved follow-ups. Final diff reviewed, target branch verified, remote artifact issue/PR/MR refreshed, single commit policy checked, cleanup status recorded.",
		PioneerSkillEvidence:   "Durable state record: issueops state id and readiness gates recorded\nPhase routing: problem issue plan implement feedback pr cleanup\nFlow evidence: issue plan TDD subagent decision feedback PR linked\nHook boundary: hooks do not create issues edit files or run tests\nCleanup/readiness evidence: strict readiness and cleanup choices recorded",
	}
}
