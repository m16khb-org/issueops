package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

const (
	ChildCleanupParentMergeVerified = "parent_merge_verified"
	ChildCleanupAlreadyClosed       = "children_already_closed"
)

func PrepareChildCleanup(record model.IssueOpsRecord, req model.IssueOpsCloseChildrenRequest) (model.IssueOpsCloseChildrenResult, error) {
	result := model.IssueOpsCloseChildrenResult{OK: true, ID: record.ID, Merged: req.Merged, Confirmed: req.Confirm, DryRun: !req.Confirm}
	if !req.Merged && !req.MergeEvidenceRequested {
		result.Missing = []string{"merge_evidence"}
		return result, fmt.Errorf("cannot close child tasks without merge evidence")
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		result.Missing = []string{"parent_issue"}
		return result, fmt.Errorf("cannot close child tasks before linked parent issue")
	}
	return result, nil
}

// An unmerged parent artifact cannot be replaced by child closure evidence.
// A parent without its own artifact requires observation of every child.
func ChildCleanupEvidenceBasis(record model.IssueOpsRecord, merged bool) (string, error) {
	if merged {
		return ChildCleanupParentMergeVerified, nil
	}
	if record.RemoteArtifact != nil {
		return "", fmt.Errorf("cannot close child tasks without merge evidence: parent artifact is not verified merged")
	}
	return "", nil
}

func ValidateClosedChildObservation(url, state string) error {
	if strings.EqualFold(state, "closed") {
		return nil
	}
	if state == "" {
		state = "unobserved"
	}
	return fmt.Errorf("cannot close child tasks without merge evidence: child %s is %s remotely", url, state)
}

func ValidateChildCloseConfirmation(result model.IssueOpsCloseChildResult, confirm bool) error {
	if confirm && (!result.HierarchyVerified || !result.Closed) {
		return fmt.Errorf("provider did not verify child close for %s", result.URL)
	}
	return nil
}

func ApplyChildCleanupReceipts(record model.IssueOpsRecord, indices []int, now string) model.IssueOpsRecord {
	record.IssueLinks = append([]model.IssueOpsIssueLink(nil), record.IssueLinks...)
	for _, index := range indices {
		link := &record.IssueLinks[index]
		if strings.TrimSpace(link.ClosedAt) == "" {
			link.ClosedAt = now
		}
		link.CloseVerifiedAt = now
		link.CloseReason = "completed"
	}
	record.UpdatedAt = now
	return record
}
