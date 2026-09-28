package issueops

import (
	"fmt"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

// LinkedBranchCleanupGates는 관측 이전에 확정할 수 있는 전제만 본다. 외부
// 호출을 하기 전에 막을 수 있는 것은 여기서 막는다.
func LinkedBranchCleanupGates(record issueopscontract.IssueOpsRecord, canObserve, canDelete bool) []string {
	var missing []string
	if !canObserve {
		missing = append(missing, "linked_branch_observation_unavailable")
	}
	if !canDelete {
		missing = append(missing, "linked_branch_deletion_unavailable")
	}
	prepare := record.BranchPrepare
	if prepare == nil {
		return append(missing, "branch_prepare_missing")
	}
	// LinkedBranch는 GitHub의 개념이다. GitLab에는 대응물이 없으므로 여기서
	// 멈춘다 — 다른 provider에서 이 경로가 열리면 무엇을 지울지 정의되지 않는다.
	if prepare.Provider != "github" {
		missing = append(missing, "linked_branch_cleanup_is_github_only")
	}
	if strings.TrimSpace(prepare.IssueURL) == "" {
		missing = append(missing, "issue_url_missing")
	}
	if strings.TrimSpace(prepare.Branch) == "" {
		missing = append(missing, "branch_missing")
	}
	if strings.TrimSpace(prepare.BaseSHA) == "" {
		missing = append(missing, "sealed_base_missing")
	}
	if strings.TrimSpace(record.Repo) == "" {
		missing = append(missing, "repo_missing")
	}
	return missing
}

func ValidateLinkedBranchCleanupApply(req issueopscontract.CleanupLinkedBranchRequest, fingerprint string) (string, error) {
	if !req.Confirm {
		return "confirm", fmt.Errorf("cleanup linked-branch --apply requires --confirm")
	}
	if req.Fingerprint != fingerprint {
		return "stale_fingerprint", fmt.Errorf("cleanup linked-branch fingerprint is stale: rerun the preview")
	}
	return "", nil
}
