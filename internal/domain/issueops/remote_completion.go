package issueops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

type CompletionArtifact struct {
	Name, Body string
	Present    bool
}

func CompletionArtifactNames() []string { return []string{"plan", "spec", "verified-execution-loop"} }

func CompletionArtifactRoot(record model.IssueOpsRecord) string {
	if record.Execution != nil && strings.TrimSpace(record.Execution.Workspace.Root) != "" {
		return strings.TrimSpace(record.Execution.Workspace.Root)
	}
	return strings.TrimSpace(record.Repo)
}

func ProjectRemoteCompletion(record model.IssueOpsRecord, artifacts []CompletionArtifact) model.RemoteCompletionSection {
	result := model.RemoteCompletionSection{}
	if record.RemoteArtifact != nil {
		result.RemoteArtifactURL = record.RemoteArtifact.URL
	}
	if record.Execution != nil && record.Execution.Completion != nil {
		completion := record.Execution.Completion
		result.FinalHead = completion.FinalHead
		result.VerificationSummary = completion.Verification
		if result.RemoteArtifactURL == "" {
			result.RemoteArtifactURL = completion.RemoteArtifactURL
		}
	}
	if len(result.VerificationSummary) == 0 {
		result.VerificationSummary = record.AISlopCleanVerification
	}
	for _, artifact := range artifacts {
		if !artifact.Present {
			if artifact.Name == "plan" {
				result.MissingArtifacts = append(result.MissingArtifacts, artifact.Name)
			}
			continue
		}
		digest := sha256.Sum256([]byte(artifact.Body))
		result.ArtifactManifest = append(result.ArtifactManifest, model.CompletionArtifactDigest{Name: artifact.Name, SHA256: hex.EncodeToString(digest[:])})
		switch artifact.Name {
		case "plan":
			result.PlanBody = artifact.Body
		case "spec":
			result.SpecBody = artifact.Body
		case "verified-execution-loop":
			body := artifact.Body
			if len(body) > 4096 {
				body = body[:4096] + "\n\u2026 (\uc808\ub2e8)"
			}
			result.TuringSummary = body
		}
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
