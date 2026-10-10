package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

// NormalizeMergeMethod defaults to squash and rejects anything the providers
// cannot express as one of their three merge strategies.
func NormalizeMergeMethod(raw string) (string, error) {
	switch method := strings.ToLower(strings.TrimSpace(raw)); method {
	case "":
		return model.RemoteMergeMethodSquash, nil
	case model.RemoteMergeMethodSquash, model.RemoteMergeMethodMerge, model.RemoteMergeMethodRebase:
		return method, nil
	default:
		return "", fmt.Errorf("merge method must be squash, merge, or rebase")
	}
}

// MergeTarget is the artifact a completed cycle may merge and the head its
// completion sealed.
type MergeTarget struct {
	Provider, URL, ExpectedHead string
}

// ResolveMergeTarget admits only a completed cycle: the worktree session owns
// everything up to `execution complete`, and merging an artifact whose
// completion is missing would merge unverified commits.
func ResolveMergeTarget(record model.IssueOpsRecord) (MergeTarget, error) {
	if record.Phase != model.IssueOpsPhaseDone {
		return MergeTarget{}, fmt.Errorf("merge-pr requires a completed cycle (phase done); current phase is %s", record.Phase)
	}
	if record.Execution == nil || record.Execution.Completion == nil || strings.TrimSpace(record.Execution.Completion.FinalHead) == "" {
		return MergeTarget{}, fmt.Errorf("merge-pr requires a recorded execution completion")
	}
	if record.Execution.Lease.Status != model.LeaseStatusReleased {
		return MergeTarget{}, fmt.Errorf("merge-pr requires a released execution lease; current lease is %s", record.Execution.Lease.Status)
	}
	artifact := record.RemoteArtifact
	if artifact == nil || strings.TrimSpace(artifact.URL) == "" || strings.TrimSpace(artifact.Provider) == "" {
		return MergeTarget{}, fmt.Errorf("merge-pr requires a verified remote artifact")
	}
	if completed := strings.TrimSpace(record.Execution.Completion.RemoteArtifactURL); completed != "" && completed != strings.TrimSpace(artifact.URL) {
		return MergeTarget{}, fmt.Errorf("completion artifact %s does not match the verified remote artifact %s", completed, artifact.URL)
	}
	return MergeTarget{Provider: artifact.Provider, URL: strings.TrimSpace(artifact.URL), ExpectedHead: strings.TrimSpace(record.Execution.Completion.FinalHead)}, nil
}

// MergeBlockers lists every reason the observed PR/MR must not be merged now.
// A merged artifact has no blockers; the caller reports it as already merged.
func MergeBlockers(state model.RemotePullRequestMergeState, expectedHead string) []model.RemoteMergeBlocker {
	blockers := []model.RemoteMergeBlocker{}
	if state.State == "merged" {
		return blockers
	}
	add := func(code, message string) {
		blockers = append(blockers, model.RemoteMergeBlocker{Code: code, Message: message})
	}
	if state.State != "open" {
		add(model.RemoteMergeBlockerNotOpen, fmt.Sprintf("the pull or merge request is %s", state.State))
	}
	if !strings.EqualFold(strings.TrimSpace(state.HeadOID), strings.TrimSpace(expectedHead)) {
		add(model.RemoteMergeBlockerHeadMismatch, fmt.Sprintf("head %s is not the completed final head %s", state.HeadOID, expectedHead))
	}
	switch state.Checks {
	case model.RemoteChecksFailing:
		add(model.RemoteMergeBlockerChecksFailing, "failing checks: "+strings.Join(state.Failing, ", "))
	case model.RemoteChecksPending:
		add(model.RemoteMergeBlockerChecksPending, "checks still running: "+strings.Join(state.Pending, ", "))
	}
	if state.Conflict {
		add(model.RemoteMergeBlockerConflict, "the source branch conflicts with the base branch")
	}
	if blocked := strings.TrimSpace(state.Blocked); blocked != "" {
		add(model.RemoteMergeBlocked, "the provider blocks the merge: "+blocked)
	}
	return blockers
}
