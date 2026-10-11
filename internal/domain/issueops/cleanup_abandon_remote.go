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
		effects = appendNonEmpty(effects, previewClosePR(&observed, plan, facts))
	}
	if req.CloseIssue {
		effects = appendNonEmpty(effects, previewCloseIssue(&observed, plan, facts))
	}
	if req.DeleteRemoteBranch {
		effects = appendNonEmpty(effects, previewDeleteRemoteBranch(&observed, &inventory, plan, facts))
	}
	if len(observed.RemoteMissing) == 0 && (req.ClosePR || req.CloseIssue || req.DeleteRemoteBranch) {
		observed.RemoteEffects = effects
	}
	return inventory, observed
}

func appendNonEmpty(effects []string, effect string) []string {
	if effect == "" {
		return effects
	}
	return append(effects, effect)
}

// previewClosePR는 PR 닫기 단계의 미충족 항목을 observed에 기록하고, 계획된 효과(없으면 "")를 돌려준다.
func previewClosePR(observed *CleanupAbandonObservation, plan CleanupAbandonRemoteReads, facts CleanupAbandonRemoteFacts) string {
	switch {
	case plan.ArtifactMissing != "":
		observed.RemoteMissing = append(observed.RemoteMissing, plan.ArtifactMissing)
		return ""
	case facts.ArtifactError != nil:
		observed.RemoteMissing = append(observed.RemoteMissing, "remote_artifact_readable")
		return ""
	}
	observed.RemoteArtifactState = facts.ArtifactState
	switch strings.ToLower(strings.TrimSpace(facts.ArtifactState)) {
	case "merged":
		observed.RemoteMissing = append(observed.RemoteMissing, "remote_artifact_unmerged")
		return ""
	case "closed":
		return "close_pr:already_closed"
	default:
		return "close_pr"
	}
}

// previewCloseIssue는 이슈 닫기 단계의 미충족 항목을 observed에 기록하고, 계획된 효과(없으면 "")를 돌려준다.
func previewCloseIssue(observed *CleanupAbandonObservation, plan CleanupAbandonRemoteReads, facts CleanupAbandonRemoteFacts) string {
	switch {
	case plan.IssueMissing != "":
		observed.RemoteMissing = append(observed.RemoteMissing, plan.IssueMissing)
		return ""
	case facts.IssueError != nil:
		observed.RemoteMissing = append(observed.RemoteMissing, "issue_readable")
		return ""
	}
	observed.IssueState = facts.IssueState
	if strings.EqualFold(strings.TrimSpace(facts.IssueState), "closed") {
		return "close_issue:already_closed"
	}
	return "close_issue"
}

// previewDeleteRemoteBranch는 원격 브랜치 삭제 단계의 미충족 항목을 observed에 기록하고, 계획된 효과(없으면 "")를 돌려준다.
func previewDeleteRemoteBranch(observed *CleanupAbandonObservation, inventory *model.CleanupAbandonInventory, plan CleanupAbandonRemoteReads, facts CleanupAbandonRemoteFacts) string {
	switch {
	case plan.BranchMissing != "":
		observed.RemoteMissing = append(observed.RemoteMissing, plan.BranchMissing)
		return ""
	case facts.BranchError != nil:
		observed.RemoteMissing = append(observed.RemoteMissing, "remote_branch_readable")
		return ""
	}
	inventory.RemoteBranchOID = facts.BranchOID
	if facts.BranchOID == "" {
		return "remote_branch_delete:absent"
	}
	return "remote_branch_delete"
}
