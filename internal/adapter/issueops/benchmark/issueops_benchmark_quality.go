package benchmark

import (
	issueopscontract "issueops/internal/contract/issueops"
	"os"
	"strings"
)

func detectIssueOpsCriticalFailures(fixture issueopscontract.IssueOpsBenchmarkFixture, artifact issueopscontract.IssueOpsBenchmarkArtifact) []string {
	var failures []string
	for _, rule := range fixture.CriticalFailures {
		ruleText := strings.ToLower(rule)
		switch {
		case strings.Contains(ruleText, "works in source repo") && !implementationInWorktree(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "skips branch prompt") && strings.TrimSpace(artifact.BranchName) == "":
			failures = append(failures, rule)
		case strings.Contains(ruleText, "worker starts without context check") && !workerPromptHasContextGate(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "unbounded code-reviewer") && !reviewPromptIsBounded(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "removes dirty worktree") && containsAllFold(artifact.WorktreeCleanup, "dirty", "remove"):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "not written in korean") && (!containsHangul(artifact.IssueDraft) || !containsHangul(artifact.PRDraft)):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "missing issue/pr guideline reference") && !hasIssueOpsGuidelineRef(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "excessive emoji") && hasExcessiveEmoji(artifact.IssueDraft+"\n"+artifact.PRDraft):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "domain contract") && !issueOpsDomainContractEvidenceComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "api doc") && !issueOpsAPIDocGateEvidenceComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "live evidence") && !issueOpsLiveEvidenceMatrixComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "review feedback") && !strings.Contains(ruleText, "review-agent threads") && !issueOpsReviewFeedbackEvidenceComplete(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "completion hygiene") && !issueOpsCompletionHygieneComplete(artifact):
			failures = append(failures, rule)
		}
	}
	return append(failures, detectIssueOpsQualityCriticalFailures(fixture, artifact)...)
}

func implementationInWorktree(artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	worktreePath := strings.TrimSpace(artifact.WorktreePath)
	location := strings.TrimSpace(artifact.ImplementationLocation)
	return worktreePath != "" && location != "" && (location == worktreePath || strings.HasPrefix(location, worktreePath+string(os.PathSeparator)))
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
		(containsFold(prompt, "worktree") || strings.TrimSpace(artifact.WorktreePath) != "") &&
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
	return containsFold(artifact.GuidelineRef, guideline) ||
		containsFold(artifact.IssueDraft, guideline) ||
		containsFold(artifact.PRDraft, guideline)
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
