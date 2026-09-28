package issueops

import (
	"os"
	"strings"

	"issueops/internal/adapter/issueops/delegation"
	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/adapter/issueops/readinesspaths"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
)

func IssueOpsPlanReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsReadinessFrom(record, issueopsdomain.PlanReadinessMissing(record))
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
	missing := issueOpsBaseImplementationMissing(record)
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueOpsPlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	return issueops.IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      stringlist.UniqueSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

func IssueOpsImplementationReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsImplementationReadiness(record, true)
}

func issueOpsImplementationReadiness(record issueops.IssueOpsRecord, checkPlanBinding bool) issueops.IssueOpsReadiness {
	missing := issueOpsBaseImplementationMissing(record)
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueOpsPlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	missing = append(missing, issueOpsCompatibilityReviewMissing(record)...)
	missing = append(missing, issueOpsDevilsAdvocateReviewMissing(record, checkPlanBinding)...)
	if record.Execution == nil {
		missing = append(missing, "execution")
	} else {
		if err := issueopsdomain.ValidateExecution(*record.Execution); err != nil {
			missing = append(missing, "execution_valid")
		}
		if !samePath(record.WorktreePath, record.Execution.Workspace.Root) {
			missing = append(missing, "execution_worktree_match")
		}
		if record.Execution.Lease.Status != issueops.LeaseStatusActive || record.Execution.Lease.Holder == nil {
			missing = append(missing, "execution_write_lease")
		}
	}
	return issueops.IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      stringlist.UniqueSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

// issueOpsDevilsAdvocateReviewMissing is the fail-closed implement-entry gate for
// the design-review devil's advocate: a review must be recorded, a stop/revise verdict
// must be explicitly waived, and (when checkPlanBinding) the verdict must have
// been recorded against the plan content that is about to be implemented —
// otherwise `devils_advocate_review_stale` blocks entry until a fresh review is
// recorded on the final plan.
func issueOpsDevilsAdvocateReviewMissing(record issueops.IssueOpsRecord, checkPlanBinding bool) []string {
	review := record.DevilsAdvocateReview
	if review == nil || strings.TrimSpace(review.RecordedAt) == "" {
		return []string{"devils_advocate_review"}
	}
	missing := []string{}
	if (review.Verdict == "stop" || review.Verdict == "revise") && !review.Waived {
		missing = append(missing, "devils_advocate_review")
	}
	if checkPlanBinding && strings.TrimSpace(record.PlanPath) != "" && !issueOpsDevilsAdvocateDigestExempt(*review) {
		if strings.TrimSpace(review.ReviewedPlanDigest) == "" {
			missing = append(missing, "devils_advocate_review_stale")
		} else if digest, err := issueOpsLinkedPlanDigest(record); err != nil || !strings.EqualFold(digest, review.ReviewedPlanDigest) {
			// A plan that cannot be identified (symlink, empty file, unreadable) is
			// not the plan that was reviewed either — fail closed, same as Record
			// and the owner preflight do. plan_exists is weaker (Stat follows
			// symlinks), so this is not a duplicate report.
			missing = append(missing, "devils_advocate_review_stale")
		}
	}
	return missing
}

// issueOpsDevilsAdvocateDigestExempt: a delegated child inherits the parent's
// verdict by policy (delegation.ParentReviewPattern) and never reviewed its own
// plan, so plan binding does not apply to it.
func issueOpsDevilsAdvocateDigestExempt(review issueops.IssueOpsDevilsAdvocateReview) bool {
	return review.ReviewerPattern == delegation.ParentReviewPattern
}

func issueOpsCompatibilityReviewMissing(record issueops.IssueOpsRecord) []string {
	review := record.CompatibilityReview
	if review == nil {
		return []string{"compatibility_review"}
	}
	missing := []string{}
	if len(cleanIssueOpsTextValues(review.BackwardCompatibility)) == 0 {
		missing = append(missing, "backward_compatibility")
	}
	if len(cleanIssueOpsTextValues(review.SideEffects)) == 0 {
		missing = append(missing, "side_effects")
	}
	if strings.TrimSpace(review.RollbackPlan) == "" {
		missing = append(missing, "rollback_plan")
	}
	if len(cleanIssueOpsTextValues(review.Verification)) == 0 {
		missing = append(missing, "compatibility_verification")
	}
	if len(cleanIssueOpsTextValues(review.Blockers)) > 0 {
		missing = append(missing, "compatibility_blockers")
	}
	if !review.Approved {
		missing = append(missing, "compatibility_approval")
	}
	return missing
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
