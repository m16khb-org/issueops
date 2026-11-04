package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// PRCompletionMissing extends baseMissing with the gates for completing the
// pr phase: a remote artifact with a URL and a target branch that matches the
// prepared base.
func PRCompletionMissing(record model.Record, baseMissing []string) []string {
	missing := append([]string{}, baseMissing...)
	if record.RemoteArtifact == nil || strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		missing = append(missing, "remote_artifact")
	}
	return append(missing, TargetBranchMatchMissing(record)...)
}

// DoneCompletionMissing lists the gates blocking the done phase: the record
// must have reached pr, plus every entry in remoteMissing, which the caller
// derives from the remote artifact observation.
func DoneCompletionMissing(record model.Record, remoteMissing []string) []string {
	missing := []string{}
	if PhaseRank(record.Phase) < PhaseRank(model.PhasePR) {
		missing = append(missing, "prior_phase_pr")
	}
	return append(missing, remoteMissing...)
}
