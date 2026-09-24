package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"issueops/internal/contract/issueops"
	"issueops/internal/domain/artifactreadability"
	"issueops/internal/port"
)

// ReflectIssueCompletion writes the human-written progress report into the
// linked remote issue's completion region after merge. merged must carry
// caller-verified provider readback evidence (the same discipline as cleanup
// close-children); without it the write is rejected. resultBody is judged by
// the readability check for progress reports: a confirm needs a body and
// refuses critical findings before the provider is called. On a confirmed
// successful update it stamps RemoteCompletion.ReflectedAt as a local cache
// of the remote state.
func ReflectIssueCompletion(stateRoot, id, resultBody string, merged, confirm bool, prov port.IssueProvider) (issueops.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, artifactreadability.Report, error) {
	var none port.IssueProviderUpdateIssueBodySectionResult
	var report artifactreadability.Report
	if prov == nil {
		return issueops.IssueOpsRecord{OK: false}, none, report, fmt.Errorf("no issue provider configured")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, none, report, err
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return issueops.IssueOpsRecord{OK: false}, none, report, fmt.Errorf("cannot reflect completion before a linked issue")
	}
	if record.RemoteArtifact == nil {
		return issueops.IssueOpsRecord{OK: false}, none, report, fmt.Errorf("cannot reflect completion before a verified remote artifact")
	}
	if !merged {
		return issueops.IssueOpsRecord{OK: false}, none, report, fmt.Errorf("cannot reflect completion without provider-verified merge evidence")
	}
	resultBody = strings.TrimSpace(resultBody)
	if confirm && resultBody == "" {
		return issueops.IssueOpsRecord{OK: false}, none, report, fmt.Errorf("--body-file is required with --confirm: write the progress report for human readers first")
	}
	report = artifactreadability.Check(artifactreadability.Input{Kind: artifactreadability.KindCompletion, Body: resultBody})
	if trackedMaterialsMissing(record) {
		report.Warnings = append(report.Warnings, artifactreadability.Finding{Code: "tracked_materials_missing", Message: trackedMaterialsMissingWarning})
	}
	if confirm && !report.OK {
		return issueops.IssueOpsRecord{OK: false}, none, report, artifactreadability.RefusalError(report)
	}
	completion := gatherCompletionSection(record)
	completion.ResultBody = resultBody
	result, err := prov.UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{
		Repo:       record.Repo,
		IssueURL:   record.IssueURL,
		Section:    port.IssueBodySectionCompletion,
		Completion: &completion,
		Confirm:    confirm,
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, result, report, err
	}
	if !confirm || !result.Updated {
		return record, result, report, nil
	}
	record, err = stampRemoteCompletion(stateRoot, id, func(rc *issueops.IssueOpsRemoteCompletion, now string) {
		rc.ReflectedAt = now
	})
	return record, result, report, err
}

// CloseIssueOpsRemoteIssue closes the linked parent issue after the caller
// verified merge evidence by provider readback. On a confirmed verified close
// it stamps RemoteCompletion.IssueClosedAt as a local cache.
func CloseIssueOpsRemoteIssue(stateRoot, id string, merged, confirm bool, prov port.IssueProvider) (issueops.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
	if prov == nil {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderCloseIssueResult{}, fmt.Errorf("no issue provider configured")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, port.IssueProviderCloseIssueResult{}, err
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderCloseIssueResult{}, fmt.Errorf("cannot close before a linked issue")
	}
	if !merged {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderCloseIssueResult{}, fmt.Errorf("cannot close the issue without provider-verified merge evidence")
	}
	result, err := prov.CloseIssue(port.IssueProviderCloseIssueRequest{
		Repo:     record.Repo,
		IssueURL: record.IssueURL,
		Confirm:  confirm,
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, result, err
	}
	if !confirm || !result.Closed {
		return record, result, nil
	}
	record, err = stampRemoteCompletion(stateRoot, id, func(rc *issueops.IssueOpsRemoteCompletion, now string) {
		// AlreadyClosed 재실행이 최초 close 시각을 덮지 않게 멱등으로 둔다(C3-F8).
		if rc.IssueClosedAt == "" {
			rc.IssueClosedAt = now
		}
	})
	return record, result, err
}

func stampRemoteCompletion(stateRoot, id string, apply func(*issueops.IssueOpsRemoteCompletion, string)) (issueops.IssueOpsRecord, error) {
	var record issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, e := ReadIssueOps(stateRoot, id)
		if e != nil {
			return e
		}
		if rec.RemoteCompletion == nil {
			rec.RemoteCompletion = &issueops.IssueOpsRemoteCompletion{}
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		apply(rec.RemoteCompletion, now)
		rec.UpdatedAt = now
		record, e = writeIssueOps(stateRoot, rec)
		return e
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	return record, nil
}

// gatherCompletionSection collects the one harness value the progress
// report region keeps: the verified PR/MR URL.
func gatherCompletionSection(record issueops.IssueOpsRecord) port.IssueProviderCompletionSection {
	completion := port.IssueProviderCompletionSection{}
	if record.RemoteArtifact != nil {
		completion.RemoteArtifactURL = record.RemoteArtifact.URL
	}
	if completion.RemoteArtifactURL == "" && record.Execution != nil && record.Execution.Completion != nil {
		completion.RemoteArtifactURL = record.Execution.Completion.RemoteArtifactURL
	}
	return completion
}
