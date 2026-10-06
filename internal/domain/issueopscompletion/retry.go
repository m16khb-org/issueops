package issueopscompletion

import (
	completioncontract "issueops/internal/contract/issueopscompletion"
	"slices"
	"strings"
)

// CanRetryCompletion checks the terminal receipt before resolving report paths.
func CanRetryCompletion(record Snapshot, command completioncontract.Command) bool {
	completion := record.Completion
	return record.Phase == "done" && record.Lease.Generation == command.Generation && record.Lease.Status == "released" &&
		record.Lease.Holder == nil && record.Lease.ClaimTokenSHA256 == "" && strings.TrimSpace(record.Lease.ReleasedAt) != "" &&
		completion != nil && strings.TrimSpace(completion.CompletedAt) != "" &&
		(completion.Generation == 0 || completion.Generation == command.Generation) &&
		strings.EqualFold(completion.FinalHead, strings.TrimSpace(command.FinalHead))
}

// MatchesRetryEvidence runs after the application observes report path identity.
// Keeping this separate preserves the existing order of filesystem observations.
func MatchesRetryEvidence(completion completioncontract.Completion, command completioncontract.Command, reportPathsMatch bool) bool {
	return reportPathsMatch && slices.Equal(completion.Verification, command.Verification) &&
		completion.RemoteArtifactURL == strings.TrimSpace(command.RemoteArtifactURL)
}
