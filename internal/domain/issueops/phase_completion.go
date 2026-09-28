package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

func PRCompletionMissing(record model.IssueOpsRecord, baseMissing []string) []string {
	missing := append([]string{}, baseMissing...)
	if record.RemoteArtifact == nil || strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		missing = append(missing, "remote_artifact")
	}
	return append(missing, TargetBranchMatchMissing(record)...)
}

func DoneCompletionMissing(record model.IssueOpsRecord, remoteMissing []string) []string {
	missing := []string{}
	if IssueOpsPhaseRank(record.Phase) < IssueOpsPhaseRank(model.IssueOpsPhasePR) {
		missing = append(missing, "prior_phase_pr")
	}
	return append(missing, remoteMissing...)
}
