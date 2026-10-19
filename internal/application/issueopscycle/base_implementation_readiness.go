package issueopscycle

import (
	"strings"

	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
)

func BaseImplementationMissing(record model.IssueOpsRecord) []string {
	missing := cycledomain.BranchEvidenceMissing(record)
	missing = append(missing, cycledomain.IntentMissing(record)...)
	missing = append(missing, ArchitectureReviewMissing(record)...)
	if strings.TrimSpace(record.PlanPath) == "" {
		missing = append(missing, "plan_path")
	}
	return missing
}
