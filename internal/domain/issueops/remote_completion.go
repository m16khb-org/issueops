package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

// ProjectRemoteCompletion keeps only the verified artifact URL in the report
// contract. Local evidence is retained in records and tracked materials.
func ProjectRemoteCompletion(record model.IssueOpsRecord) model.RemoteCompletionSection {
	result := model.RemoteCompletionSection{}
	if record.RemoteArtifact != nil {
		result.RemoteArtifactURL = record.RemoteArtifact.URL
	}
	if result.RemoteArtifactURL == "" && record.Execution != nil && record.Execution.Completion != nil {
		result.RemoteArtifactURL = record.Execution.Completion.RemoteArtifactURL
	}
	return result
}

func ValidateCompletionMerge(record model.IssueOpsRecord) error {
	if record.RemoteArtifact == nil {
		return fmt.Errorf("cannot verify merge evidence before a verified remote artifact")
	}
	return nil
}

func ValidateReflectCompletion(record model.IssueOpsRecord) error {
	if strings.TrimSpace(record.IssueURL) == "" {
		return fmt.Errorf("cannot reflect completion before a linked issue")
	}
	if record.RemoteArtifact == nil {
		return fmt.Errorf("cannot reflect completion before a verified remote artifact")
	}
	return nil
}

func ValidateCloseIssue(record model.IssueOpsRecord) error {
	if strings.TrimSpace(record.IssueURL) == "" {
		return fmt.Errorf("cannot close before a linked issue")
	}
	return nil
}

func MarkRemoteCompletionReflected(record model.IssueOpsRecord, now string) model.IssueOpsRecord {
	receipt := model.IssueOpsRemoteCompletion{}
	if record.RemoteCompletion != nil {
		receipt = *record.RemoteCompletion
	}
	receipt.ReflectedAt = now
	record.RemoteCompletion = &receipt
	record.UpdatedAt = now
	return record
}

func MarkRemoteIssueClosed(record model.IssueOpsRecord, now string) model.IssueOpsRecord {
	receipt := model.IssueOpsRemoteCompletion{}
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
