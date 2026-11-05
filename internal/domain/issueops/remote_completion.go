package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

// ProjectRemoteCompletion keeps only the verified artifact URL in the report
// contract. Local evidence is retained in records and tracked materials.
func ProjectRemoteCompletion(record model.Record) model.RemoteCompletionSection {
	result := model.RemoteCompletionSection{}
	if record.RemoteArtifact != nil {
		result.RemoteArtifactURL = record.RemoteArtifact.URL
	}
	if result.RemoteArtifactURL == "" && record.Execution != nil && record.Execution.Completion != nil {
		result.RemoteArtifactURL = record.Execution.Completion.RemoteArtifactURL
	}
	return result
}

// ValidateCompletionMerge requires a verified remote artifact before merge
// evidence can be checked.
func ValidateCompletionMerge(record model.Record) error {
	if record.RemoteArtifact == nil {
		return fmt.Errorf("cannot verify merge evidence before a verified remote artifact: publish the PR/MR first (remote create-pr, then remote verify-artifact), or end an unpublished cycle with cleanup abandon")
	}
	return nil
}

// ValidateReflectCompletion requires a linked issue and a verified remote
// artifact before the completion is reflected into the issue.
func ValidateReflectCompletion(record model.Record) error {
	if strings.TrimSpace(record.IssueURL) == "" {
		return fmt.Errorf("cannot reflect completion before a linked issue")
	}
	if record.RemoteArtifact == nil {
		return fmt.Errorf("cannot reflect completion before a verified remote artifact")
	}
	return nil
}

// ValidateCloseIssue requires a linked issue before the remote issue can be
// closed.
func ValidateCloseIssue(record model.Record) error {
	if strings.TrimSpace(record.IssueURL) == "" {
		return fmt.Errorf("cannot close before a linked issue")
	}
	return nil
}

// MarkRemoteCompletionReflected stamps now as the time the completion was
// reflected into the issue, keeping the other receipt fields.
func MarkRemoteCompletionReflected(record model.Record, now string) model.Record {
	receipt := model.RemoteCompletion{}
	if record.RemoteCompletion != nil {
		receipt = *record.RemoteCompletion
	}
	receipt.ReflectedAt = now
	record.RemoteCompletion = &receipt
	record.UpdatedAt = now
	return record
}

// MarkRemoteIssueClosed stamps now as the time the remote issue was closed.
// An earlier close time is kept.
func MarkRemoteIssueClosed(record model.Record, now string) model.Record {
	receipt := model.RemoteCompletion{}
	if record.RemoteCompletion != nil {
		receipt = *record.RemoteCompletion
	}
	if receipt.IssueClosedAt == "" {
		receipt.IssueClosedAt = now
	}
	record.RemoteCompletion = &receipt
	record.UpdatedAt = now
	return record
}

// MarkRemoteMerged records that merge-pr read the PR/MR back merged. QA: every
// agent saw next name merge-pr again after the merge; next does not read the
// provider, so the merge needs a receipt of its own.
func MarkRemoteMerged(record model.Record, now string) model.Record {
	receipt := model.RemoteCompletion{}
	if record.RemoteCompletion != nil {
		receipt = *record.RemoteCompletion
	}
	if receipt.MergedAt == "" {
		receipt.MergedAt = now
	}
	record.RemoteCompletion = &receipt
	record.UpdatedAt = now
	return record
}
