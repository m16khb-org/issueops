package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// CleanupAbandonRemoteReads keeps each requested effect's prerequisite separate
// so one refused effect does not hide observations of the remaining effects.
type CleanupAbandonRemoteReads struct {
	Artifact, Issue, Branch                      bool
	ArtifactMissing, IssueMissing, BranchMissing string
}

func PlanCleanupAbandonRemoteReads(record model.IssueOpsRecord, req model.CleanupAbandonRequest, branch string, providerAvailable bool) CleanupAbandonRemoteReads {
	var plan CleanupAbandonRemoteReads
	if req.ClosePR {
		switch {
		case record.RemoteArtifact == nil:
			plan.ArtifactMissing = "remote_artifact_required"
		case !providerAvailable:
			plan.ArtifactMissing = "remote_provider_unavailable"
		default:
			plan.Artifact = true
		}
	}
	if req.CloseIssue {
		switch {
		case strings.TrimSpace(record.IssueURL) == "":
			plan.IssueMissing = "issue_url_required"
		case !providerAvailable:
			plan.IssueMissing = "remote_provider_unavailable"
		default:
			plan.Issue = true
		}
	}
	if req.DeleteRemoteBranch {
		switch {
		case branch == "":
			plan.BranchMissing = "branch_recorded"
		case record.BranchPrepare != nil && branch == strings.TrimSpace(record.BranchPrepare.BaseBranch):
			plan.BranchMissing = "branch_not_base"
		default:
			plan.Branch = true
		}
	}
	return plan
}

type CleanupAbandonRemoteFacts struct {
	ArtifactState, IssueState, BranchOID   string
	ArtifactError, IssueError, BranchError error
}

func BuildCleanupAbandonRemotePreview(req model.CleanupAbandonRequest, inventory model.CleanupAbandonInventory, plan CleanupAbandonRemoteReads, facts CleanupAbandonRemoteFacts) (model.CleanupAbandonInventory, CleanupAbandonObservation) {
	var observed CleanupAbandonObservation
	inventory.ClosePR, inventory.CloseIssue, inventory.DeleteRemoteBranch = req.ClosePR, req.CloseIssue, req.DeleteRemoteBranch
	effects := []string{}
	if req.ClosePR {
		switch {
		case plan.ArtifactMissing != "":
			observed.RemoteMissing = append(observed.RemoteMissing, plan.ArtifactMissing)
		case facts.ArtifactError != nil:
			observed.RemoteMissing = append(observed.RemoteMissing, "remote_artifact_readable")
		default:
			observed.RemoteArtifactState = facts.ArtifactState
			switch strings.ToLower(strings.TrimSpace(facts.ArtifactState)) {
			case "merged":
				observed.RemoteMissing = append(observed.RemoteMissing, "remote_artifact_unmerged")
			case "closed":
				effects = append(effects, "close_pr:already_closed")
			default:
				effects = append(effects, "close_pr")
			}
		}
	}
	if req.CloseIssue {
		switch {
		case plan.IssueMissing != "":
			observed.RemoteMissing = append(observed.RemoteMissing, plan.IssueMissing)
		case facts.IssueError != nil:
			observed.RemoteMissing = append(observed.RemoteMissing, "issue_readable")
		default:
			observed.IssueState = facts.IssueState
			if strings.EqualFold(strings.TrimSpace(facts.IssueState), "closed") {
				effects = append(effects, "close_issue:already_closed")
			} else {
				effects = append(effects, "close_issue")
			}
		}
	}
	if req.DeleteRemoteBranch {
		switch {
		case plan.BranchMissing != "":
			observed.RemoteMissing = append(observed.RemoteMissing, plan.BranchMissing)
		case facts.BranchError != nil:
			observed.RemoteMissing = append(observed.RemoteMissing, "remote_branch_readable")
		default:
			inventory.RemoteBranchOID = facts.BranchOID
			if facts.BranchOID == "" {
				effects = append(effects, "remote_branch_delete:absent")
			} else {
				effects = append(effects, "remote_branch_delete")
			}
		}
	}
	if len(observed.RemoteMissing) == 0 && (req.ClosePR || req.CloseIssue || req.DeleteRemoteBranch) {
		observed.RemoteEffects = effects
	}
	return inventory, observed
}
