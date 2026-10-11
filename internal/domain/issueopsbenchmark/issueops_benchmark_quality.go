package issueopsbenchmark

import (
	"path/filepath"
	"strings"

	issueopscontract "issueops/internal/contract/issueopsbenchmark"
)

// issueOpsCriticalFailureRule은 fixture의 critical failure 문구(소문자)에 marker가
// 포함될 때 artifact가 그 실패 조건을 만족하는지 판정한다. rule 순서대로 평가하며
// 하나라도 일치하면 해당 문구를 한 번만 보고한다.
type issueOpsCriticalFailureRule struct {
	marker    string
	triggered func(ruleText string, artifact issueopscontract.IssueOpsBenchmarkArtifact) bool
}

var issueOpsCriticalFailureRules = []issueOpsCriticalFailureRule{
	{"works in source repo", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool { return !implementationInWorktree(a) }},
	{"skips branch prompt", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool {
		return strings.TrimSpace(a.BranchName) == ""
	}},
	{"worker starts without context check", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool {
		return !workerPromptHasContextGate(a)
	}},
	{"unbounded code-reviewer", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool { return !reviewPromptIsBounded(a) }},
	{"removes dirty worktree", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool {
		return containsAllFold(a.WorktreeCleanup, "dirty", "remove")
	}},
	{"not written in korean", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool {
		return !containsHangul(a.IssueDraft) || !containsHangul(a.PRDraft)
	}},
	{"missing issue/pr guideline reference", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool { return !hasIssueOpsGuidelineRef(a) }},
	{"excessive emoji", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool {
		return hasExcessiveEmoji(a.IssueDraft + "\n" + a.PRDraft)
	}},
	{"domain contract", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool {
		return !issueOpsDomainContractEvidenceComplete(a)
	}},
	{"api doc", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool {
		return !issueOpsAPIDocGateEvidenceComplete(a)
	}},
	{"live evidence", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool {
		return !issueOpsLiveEvidenceMatrixComplete(a)
	}},
	{"review feedback", func(ruleText string, a issueopscontract.IssueOpsBenchmarkArtifact) bool {
		return !strings.Contains(ruleText, "review-agent threads") && !issueOpsReviewFeedbackEvidenceComplete(a)
	}},
	{"completion hygiene", func(_ string, a issueopscontract.IssueOpsBenchmarkArtifact) bool {
		return !issueOpsCompletionHygieneComplete(a)
	}},
}

// issueOpsCriticalFailureTriggered는 소문자 rule 문구가 등록된 rule 중 하나라도 만족하는지 반환한다.
func issueOpsCriticalFailureTriggered(ruleText string, artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	for _, r := range issueOpsCriticalFailureRules {
		if strings.Contains(ruleText, r.marker) && r.triggered(ruleText, artifact) {
			return true
		}
	}
	return false
}

func detectIssueOpsCriticalFailures(fixture issueopscontract.IssueOpsBenchmarkFixture, artifact issueopscontract.IssueOpsBenchmarkArtifact, routingComplete bool) []string {
	var failures []string
	for _, rule := range fixture.CriticalFailures {
		if issueOpsCriticalFailureTriggered(strings.ToLower(rule), artifact) {
			failures = append(failures, rule)
		}
	}
	return append(failures, detectIssueOpsQualityCriticalFailures(fixture, artifact, routingComplete)...)
}

func implementationInWorktree(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	worktreePath := strings.TrimSpace(artifact.WorktreePath)
	location := strings.TrimSpace(artifact.ImplementationLocation)
	return worktreePath != "" && location != "" && (location == worktreePath || strings.HasPrefix(location, worktreePath+string(filepath.Separator)))
}

func issueOpsDomainContractEvidenceComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.DomainContractEvidence, "invariant", "exact mechanism", "equivalent behavior", "source") &&
		containsAnyFold(artifact.DomainContractEvidence, "file:", "line", ":")
}

func issueOpsAPIDocGateEvidenceComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.APIDocGateEvidence, "changed endpoint", "public error", "static check", "review") &&
		containsAnyFold(artifact.APIDocGateEvidence, "openapi", "swagger", "api-doc", "api doc")
}

func issueOpsLiveEvidenceMatrixComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.LiveEvidenceMatrix, "environment", "repo config", "runtime", "remediation order") &&
		containsAnyFold(artifact.LiveEvidenceMatrix, "dev", "stg", "prod", "local", "production")
}

func issueOpsReviewFeedbackEvidenceComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.ReviewFeedbackEvidence, "classification", "verification", "thread reply", "resolution") &&
		containsAnyFold(artifact.ReviewFeedbackEvidence, "valid", "stale", "noise", "contract_change", "defect") &&
		issueOpsReviewAgentThreadEvidenceComplete(artifact)
}

func issueOpsCompletionHygieneComplete(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return containsAllFold(artifact.CompletionHygiene, "final diff", "target branch", "remote artifact", "single commit", "cleanup") &&
		containsAnyFold(artifact.CompletionHygiene, "pr", "mr", "issue") &&
		issueOpsDraftIssueCompletionEvidenceComplete(artifact)
}

func workerPromptHasContextGate(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	prompt := artifact.SubagentPrompts
	return containsAllFold(prompt, "pwd", "branch", "head") &&
		(ContainsFold(prompt, "worktree") || strings.TrimSpace(artifact.WorktreePath) != "") &&
		containsAnyFold(prompt, "stop", "halt", "중단")
}

func reviewPromptIsBounded(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	prompt := artifact.SubagentPrompts
	if !containsAnyFold(prompt, "review", "code-reviewer", "verifier") {
		return true
	}
	if containsAnyFold(prompt, "verifier", "direct bounded review", "bounded review") {
		return true
	}
	return containsAnyFold(prompt, "do not spawn subagents", "no nested subagents", "nested subagent fan-out 금지") &&
		containsAnyFold(prompt, "minute", "minutes", "time budget", "분", "시간 예산")
}

// The section concepts follow the reader-first body contract owned by
// internal/domain/artifacttemplate: five required issue sections and four
// required PR sections.
var issueOpsIssueSectionConcepts = [][]string{
	{"summary", "요약"},
	{"background", "배경"},
	{"acceptance criteria", "완료 기준", "수용 기준", "인수 기준"},
	{"scope", "범위"},
	{"verification", "검증"},
}

var issueOpsPRSectionConcepts = [][]string{
	{"summary", "요약"},
	{"changes", "변경 내용"},
	{"verified", "확인한 것"},
	{"reviewer focus", "리뷰 포인트"},
}

func hasIssueOpsGuidelineRef(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	const guideline = "skills/issueops-create-issue/SKILL.md; skills/issueops-create-pr/SKILL.md"
	return ContainsFold(artifact.GuidelineRef, guideline) ||
		ContainsFold(artifact.IssueDraft, guideline) ||
		ContainsFold(artifact.PRDraft, guideline)
}

func hasExcessiveEmoji(s string) bool {
	count := 0
	for _, r := range s {
		if isEmojiRune(r) {
			count++
		}
	}
	return count > 3
}

func isEmojiRune(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1FAFF) ||
		(r >= 0x2600 && r <= 0x27BF)
}
