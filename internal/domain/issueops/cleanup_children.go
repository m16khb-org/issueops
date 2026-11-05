package issueops

import (
	"fmt"
	"slices"
	"strings"

	model "issueops/internal/contract/issueops"
)

// Constants naming the evidence basis of child cleanup.
// ChildCleanupAlreadyClosed is the basis reported when every child issue was
// already closed remotely.
const (
	childCleanupParentMergeVerified = "parent_merge_verified"
	ChildCleanupAlreadyClosed       = "children_already_closed"
)

// PrepareChildCleanup validates the preconditions for closing child tasks and
// starts the result: merge evidence must be given or requested and the parent
// issue must be linked. Failures set Missing on the returned result.
func PrepareChildCleanup(record model.Record, req model.CloseChildrenRequest) (model.CloseChildrenResult, error) {
	result := model.CloseChildrenResult{OK: true, ID: record.ID, Merged: req.Merged, Confirmed: req.Confirm, DryRun: !req.Confirm}
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

// ChildCleanupEvidenceBasis names the evidence that allows child cleanup. A
// verified parent merge is the basis; without it, an unmerged parent artifact
// cannot be replaced by child closure evidence, and a parent without its own
// artifact requires observation of every child.
func ChildCleanupEvidenceBasis(record model.Record, merged bool) (string, error) {
	if merged {
		return childCleanupParentMergeVerified, nil
	}
	if record.RemoteArtifact != nil {
		return "", fmt.Errorf("cannot close child tasks without merge evidence: parent artifact is not verified merged")
	}
	return "", nil
}

// ValidateClosedChildObservation fails unless the observed remote state of
// child url is closed. An empty state is reported as unobserved.
func ValidateClosedChildObservation(url, state string) error {
	if strings.EqualFold(state, "closed") {
		return nil
	}
	if state == "" {
		state = "unobserved"
	}
	return fmt.Errorf("cannot close child tasks without merge evidence: child %s is %s remotely", url, state)
}

// ValidateChildCloseConfirmation fails when confirm is set but the provider
// did not verify the hierarchy or the close of the child.
func ValidateChildCloseConfirmation(result model.CloseChildResult, confirm bool) error {
	if confirm && (!result.HierarchyVerified || !result.Closed) {
		return fmt.Errorf("provider did not verify child close for %s", result.URL)
	}
	return nil
}

// ApplyChildCleanupReceipts stamps the issue links at indices as closed and
// close-verified at now with reason "completed". It keeps an earlier ClosedAt
// and copies the link slice before changing it.
func ApplyChildCleanupReceipts(record model.Record, indices []int, now string) model.Record {
	record.IssueLinks = append([]model.IssueLink(nil), record.IssueLinks...)
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

// ValidateChildCleanupMergeObservation binds remote merge evidence to the
// current closure inputs after the cycle lock is reacquired. It fails when
// the identity, branch, issue links, or remote artifact changed since
// observation; metadata unrelated to child closure may advance independently.
func ValidateChildCleanupMergeObservation(observed, current model.Record) error {
	changed := observed.ID != current.ID || observed.Repo != current.Repo || observed.Branch != current.Branch || observed.IssueURL != current.IssueURL || !slices.Equal(observed.IssueLinks, current.IssueLinks)
	if (observed.RemoteArtifact == nil) != (current.RemoteArtifact == nil) {
		changed = true
	} else if observed.RemoteArtifact != nil {
		before, after := observed.RemoteArtifact, current.RemoteArtifact
		changed = changed || before.Provider != after.Provider || before.Kind != after.Kind || before.URL != after.URL || before.TargetBranch != after.TargetBranch
	}
	if changed {
		return fmt.Errorf("child cleanup inputs changed during merge observation; retry")
	}
	return nil
}
