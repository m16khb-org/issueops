package issueops

import (
	"os"
	"strings"

	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/adapter/issueops/readinesspaths"
	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
	cycleport "issueops/internal/port/issueopscycle"
)

func IssueOpsPlanReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return cycleapp.ReadinessFromMissing(record, issueopsdomain.PlanReadinessMissing(record))
}

func IssueOpsAISlopCleanReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	// 구현 중 플랜 편집(체크박스 등)은 ai-slop-clean 진입을 막지 않는다 — plan
	// binding은 implement 진입 게이트다.
	ready := issueOpsImplementationReadiness(record, false)
	missing := append([]string{}, ready.Missing...)
	if !implementation.HasEvidence(record) {
		missing = append(missing, "implementation_changes")
	}
	missing = stringlist.UniqueSorted(missing)
	ready.Missing = missing
	ready.Ready = len(missing) == 0
	return ready
}

func IssueOpsCompatibilityReviewReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return cycleapp.ReadinessFromMissing(record, cycleapp.CompatibilityReadinessMissing(record, issueOpsReadinessObservations()))
}

func IssueOpsImplementationReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsImplementationReadiness(record, true)
}

func issueOpsImplementationReadiness(record issueops.IssueOpsRecord, checkPlanBinding bool) issueops.IssueOpsReadiness {
	return cycleapp.ReadinessFromMissing(record, cycleapp.ImplementationReadinessMissing(record, checkPlanBinding, issueOpsReadinessObservations()))
}

func issueOpsReadinessObservations() cycleport.ReadinessObservations {
	return cycleport.ReadinessObservations{
		WorktreePathValid:    issueOpsWorktreePathValid,
		PlanPathExists:       issueOpsPlanPathExists,
		PlanInLinkedWorktree: issueOpsPlanInLinkedWorktree,
		WorkspaceMatches:     samePath,
		LinkedPlanDigest:     ReviewPlanSource{}.LinkedDigest,
	}
}

func issueOpsStrictGitRoot(record issueops.IssueOpsRecord) string {
	return readinesspaths.StrictGitRoot(record)
}

func issueOpsWorktreePathValid(path string) bool {
	return readinesspaths.WorktreePathValid(path)
}

func issueOpsPlanPathExists(repo, path string) bool {
	return readinesspaths.PlanPathExists(repo, path)
}

// issueOpsPlanSectionsMissing은 link-plan이 계획 본문에서 빠진 필수 절을 찾는
// 경로다. 읽을 수 없는 계획은 절이 전부 없는 것으로 본다.
func issueOpsPlanSectionsMissing(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return append([]string(nil), issueopsdomain.RequiredPlanSections...)
	}
	return issueopsdomain.MissingPlanSections(string(raw))
}

func issueOpsPlanInLinkedWorktree(record issueops.IssueOpsRecord) bool {
	return readinesspaths.PlanInLinkedWorktree(record)
}

func issueOpsPlanPathInsideWorktree(worktree, planPath string) bool {
	return readinesspaths.PlanPathInsideWorktree(worktree, planPath)
}

func issueOpsCurrentHead(record issueops.IssueOpsRecord) string {
	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	if code, out, _ := GitCmd(gitRoot, "rev-parse", "HEAD"); code == 0 {
		return strings.TrimSpace(out)
	}
	return ""
}
