package issueopscycle

import (
	"strings"

	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
)

type ObservedPRFacts struct {
	CurrentFingerprint string
	SchemaMissing      string
	PlanExists         bool
	PlanInWorktree     bool
	WorktreeValid      bool
}

func ObservedPRReadinessMissing(record model.IssueOpsRecord, facts ObservedPRFacts) []string {
	missing := []string{}
	// Local readiness includes absent/non-pass reviews. Only fingerprint staleness
	// can be determined from the observed Git change set.
	if reviewMissing := ImplementationReviewMissing(record, facts.CurrentFingerprint); strings.HasSuffix(reviewMissing, "_stale") {
		missing = append(missing, reviewMissing)
	}
	if docsMissing := ProjectDocsReviewMissing(record, facts.CurrentFingerprint); strings.HasSuffix(docsMissing, "_stale") {
		missing = append(missing, docsMissing)
	}
	if facts.SchemaMissing != "" {
		missing = append(missing, facts.SchemaMissing)
	}
	missing = append(missing, cycledomain.AISlopCleanFingerprintReadinessMissing(record, facts.CurrentFingerprint)...)
	if strings.TrimSpace(record.PlanPath) != "" && !facts.PlanExists {
		missing = append(missing, "plan_exists")
	}
	if !facts.PlanInWorktree {
		missing = append(missing, "plan_in_worktree")
	}
	if strings.TrimSpace(record.WorktreePath) == "" {
		missing = append(missing, "worktree_path")
	} else if !facts.WorktreeValid {
		missing = append(missing, "worktree_exists")
	}
	return append(missing, cycledomain.TargetBranchMatchMissing(record)...)
}
