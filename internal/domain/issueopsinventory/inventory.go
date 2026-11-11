// Package issueopsinventory holds the domain rules that project issueops
// cycle records into inventory list entries.
package issueopsinventory

import (
	"time"

	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

// ProjectEntry projects a cycle Record onto the flat ListEntry shown in
// inventory listings, including lease, pending, failure, cleanup, and remote
// completion state when present.
func ProjectEntry(record issueopsinventorycontract.Record) issueopsinventorycontract.ListEntry {
	entry := issueopsinventorycontract.ListEntry{
		ID:        record.ID,
		Repo:      record.Repo,
		Branch:    record.Branch,
		Phase:     record.Phase,
		IssueURL:  record.IssueURL,
		UpdatedAt: record.UpdatedAt,
		Invalid:   record.Invalid,
	}
	entry.LastRecordAt = LastRecordAt(record)
	if record.RemoteArtifact != nil {
		entry.RemoteArtifactURL = record.RemoteArtifact.URL
		entry.CompletionUnreflected = record.RemoteCompletion == nil ||
			record.RemoteCompletion.ReflectedAt == ""
	}
	if record.Execution != nil {
		entry.LeaseStatus = string(record.Execution.Lease.Status)
		entry.WorkspaceRoot = record.Execution.Workspace.Root
		if record.Execution.Lease.Holder != nil {
			entry.HolderHost = record.Execution.Lease.Holder.Host
			entry.HolderSession = record.Execution.Lease.Holder.SessionID
		}
		if record.Execution.Pending != nil {
			entry.PendingKind = record.Execution.Pending.Kind
			entry.PendingSince = record.Execution.Pending.StartedAt
		}
		if record.Execution.Failure != nil {
			entry.FailureCode = record.Execution.Failure.Code
			entry.FailureAt = record.Execution.Failure.At
		}
		entry.Claimable = record.Execution.Lease.Status == issueopsinventorycontract.LeaseStatusClaimable
	}
	if record.CleanupFinishFailure != nil {
		entry.CleanupFailureStep = record.CleanupFinishFailure.Step
		entry.CleanupFailureAt = record.CleanupFinishFailure.At
	}
	if record.CleanupAbandonFailure != nil {
		entry.CleanupFailureStep = record.CleanupAbandonFailure.Step
		entry.CleanupFailureAt = record.CleanupAbandonFailure.At
	}
	if record.IssueCreateIntent != nil && record.IssueCreateIntent.Status != "completed" {
		entry.IssueCreateStatus = record.IssueCreateIntent.Status
	}
	entry.CleanupCandidate = record.Phase == issueopsinventorycontract.PhaseDone
	return entry
}

// LastRecordAt returns the latest instant the record's stored write paths
// stamped, formatted as UTC RFC3339Nano, or "" when none parses. Record.UpdatedAt
// alone is not enough: lease claim, release, replace, completion, and PR/MR
// publication write their own timestamps without touching it. Values that do
// not parse as RFC3339 are ignored, and instants are compared as time.Time, not
// as strings, so differing UTC offsets order correctly.
func LastRecordAt(record issueopsinventorycontract.Record) string {
	candidates := []string{record.UpdatedAt}
	if execution := record.Execution; execution != nil {
		candidates = append(candidates,
			execution.Lease.ClaimedAt, execution.Lease.ReleasedAt, execution.Lease.ReplacedAt)
		if execution.Pending != nil {
			candidates = append(candidates, execution.Pending.StartedAt)
		}
		if execution.Completion != nil {
			candidates = append(candidates, execution.Completion.CompletedAt)
		}
		if execution.Failure != nil {
			candidates = append(candidates, execution.Failure.At)
		}
	}
	if record.IssueCreateIntent != nil {
		candidates = append(candidates, record.IssueCreateIntent.UpdatedAt)
	}
	if record.CleanupFinishFailure != nil {
		candidates = append(candidates, record.CleanupFinishFailure.At)
	}
	if record.CleanupAbandonFailure != nil {
		candidates = append(candidates, record.CleanupAbandonFailure.At)
	}
	if record.RemoteArtifact != nil {
		candidates = append(candidates, record.RemoteArtifact.VerifiedAt)
	}
	var latest time.Time
	found := false
	for _, candidate := range candidates {
		parsed, err := time.Parse(time.RFC3339Nano, candidate)
		if err != nil {
			continue
		}
		if !found || parsed.After(latest) {
			latest, found = parsed, true
		}
	}
	if !found {
		return ""
	}
	return latest.UTC().Format(time.RFC3339Nano)
}

// IdleDays returns the whole 24-hour periods between lastRecordAt and now, or
// nil when lastRecordAt is empty, unparseable, or later than now. Zero idle
// days is a non-nil pointer to 0 so "unknown" and "fresh" stay distinct.
func IdleDays(lastRecordAt string, now time.Time) *int {
	last, err := time.Parse(time.RFC3339Nano, lastRecordAt)
	if err != nil {
		return nil
	}
	elapsed := now.Sub(last)
	if elapsed < 0 {
		return nil
	}
	days := int(elapsed / (24 * time.Hour))
	return &days
}
