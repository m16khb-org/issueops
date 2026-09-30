package issueops

import (
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

func SplitDecisionMissing(record issueopscontract.IssueOpsRecord) []string {
	for _, link := range record.IssueLinks {
		switch strings.ToLower(strings.TrimSpace(link.Type)) {
		case "child", "splits-from":
			return nil
		}
	}
	for _, decision := range record.Decisions {
		if strings.EqualFold(strings.TrimSpace(decision.Kind), "scope") {
			return nil
		}
	}
	return []string{"split_decision"}
}

func DomainReviewMissing(record issueopscontract.IssueOpsRecord) []string {
	if record.DomainReview == nil || strings.TrimSpace(record.DomainReview.ReviewedAt) == "" {
		return []string{"domain_review"}
	}
	return nil
}

func TargetBranchMatchMissing(record issueopscontract.IssueOpsRecord) []string {
	if record.RemoteArtifact == nil || record.BranchPrepare == nil {
		return nil
	}
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	if base == "" {
		return nil
	}
	if strings.TrimSpace(record.RemoteArtifact.TargetBranch) != base {
		return []string{"target_branch_match"}
	}
	return nil
}
