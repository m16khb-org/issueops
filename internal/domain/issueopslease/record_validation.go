package issueopslease

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	leasecontract "issueops/internal/contract/issueopslease"
)

func ValidatePersistedRecord(record leasecontract.Record) error {
	if record.Execution == nil {
		return nil
	}
	execution := record.Execution
	if execution.Mode != "direct" && execution.Mode != "orca" {
		return fmt.Errorf("execution mode must be direct or orca")
	}
	for _, field := range []struct{ name, value string }{
		{"source_root", execution.Workspace.SourceRoot},
		{"root", execution.Workspace.Root},
		{"branch", execution.Workspace.Branch},
		{"base_head", execution.Workspace.BaseHead},
		{"driver", execution.Workspace.Driver},
		{"linked_at", execution.Workspace.LinkedAt},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("workspace %s is required", field.name)
		}
	}
	if execution.Workspace.SourceRoot == execution.Workspace.Root {
		return fmt.Errorf("canonical worktree must be isolated from source_root")
	}
	if execution.Mode == "direct" && execution.Workspace.Driver != "git" {
		return fmt.Errorf("direct execution workspace driver must be git")
	}
	if execution.Mode == "orca" && execution.Workspace.Driver != "orca" {
		return fmt.Errorf("Orca execution workspace driver must be orca")
	}
	if err := validateLease(execution.Lease); err != nil {
		return err
	}
	if execution.Selection == nil {
		return fmt.Errorf("execution selection receipt is required")
	}
	if err := ValidatePersistedSelection(*execution.Selection, execution.Mode); err != nil {
		return err
	}
	if err := ValidatePersistedSidecars(*execution); err != nil {
		return err
	}
	return nil
}

func ValidatePersistedSelection(selection leasecontract.Selection, mode string) error {
	if selection.RequestedMode != "auto" && selection.RequestedMode != "direct" && selection.RequestedMode != "orca" {
		return fmt.Errorf("selection requested_mode must be auto, direct, or orca")
	}
	if selection.ResolvedMode != mode {
		return fmt.Errorf("selection resolved_mode must equal execution mode")
	}
	if err := validateSelectionProbeFlags(selection); err != nil {
		return err
	}
	if err := validateSelectionModeAgreement(selection, mode); err != nil {
		return err
	}
	if err := validateSelectionFallback(selection, mode); err != nil {
		return err
	}
	if err := validateSelectionDirectReason(selection); err != nil {
		return err
	}
	if !validHexDigest(selection.ReadinessFingerprint, 64) || strings.TrimSpace(selection.SelectedAt) == "" {
		return fmt.Errorf("selection receipt requires fingerprint and selected_at")
	}
	return nil
}

func validateSelectionProbeFlags(selection leasecontract.Selection) error {
	if selection.ProbeAvailable && !selection.ProbeAttempted {
		return fmt.Errorf("selection probe_available requires probe_attempted")
	}
	if selection.ProbeReady && !selection.ProbeAvailable {
		return fmt.Errorf("selection probe_ready requires probe_available")
	}
	if !selection.ProbeAttempted && strings.TrimSpace(selection.ProbeCode) != "" {
		return fmt.Errorf("unattempted selection probe must not contain probe_code")
	}
	if selection.RequestedMode != "direct" && !selection.ProbeAttempted {
		return fmt.Errorf("auto and Orca selections require a readiness probe")
	}
	return nil
}

func validateSelectionModeAgreement(selection leasecontract.Selection, mode string) error {
	if (selection.RequestedMode == "direct" && mode != "direct") ||
		(selection.RequestedMode == "orca" && mode != "orca") {
		return fmt.Errorf("explicit selection mode must equal execution mode")
	}
	if mode == "orca" && !selection.ProbeReady {
		return fmt.Errorf("Orca selection requires a ready probe")
	}
	return nil
}

func validateSelectionFallback(selection leasecontract.Selection, mode string) error {
	if selection.RequestedMode == "auto" && mode == "direct" {
		probeCode := strings.TrimSpace(selection.ProbeCode)
		fallbackCode := strings.TrimSpace(selection.FallbackCode)
		if selection.ProbeReady || fallbackCode == "" || fallbackCode != probeCode || fallbackCode != selection.FallbackCode {
			return fmt.Errorf("auto direct selection requires the exact probe failure fallback_code")
		}
	}
	if mode == "orca" && strings.TrimSpace(selection.FallbackCode) != "" {
		return fmt.Errorf("Orca selection must not contain fallback_code")
	}
	return nil
}

func validateSelectionDirectReason(selection leasecontract.Selection) error {
	if selection.RequestedMode != "direct" {
		if strings.TrimSpace(selection.ExplicitDirectReason) != "" {
			return fmt.Errorf("non-direct selection must not contain explicit_direct_reason")
		}
		return nil
	}
	if selection.ProbeAttempted || strings.TrimSpace(selection.ExplicitDirectReason) == "" {
		return fmt.Errorf("explicit direct selection requires a reason and no probe")
	}
	if selection.FallbackCode != "" {
		return fmt.Errorf("explicit direct selection must not contain fallback_code")
	}
	return nil
}

func validateLease(lease leasecontract.Lease) error {
	if lease.Generation == 0 {
		return fmt.Errorf("lease generation must start at 1")
	}
	switch lease.Status {
	case "active":
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" || lease.ClaimedAt == "" {
			return fmt.Errorf("active lease requires one holder and no token hash")
		}
		return ValidatePersistedActor(*lease.Holder)
	case "revoking":
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" {
			return fmt.Errorf("revoking lease requires a holder and no token hash")
		}
		return ValidatePersistedActor(*lease.Holder)
	case "claimable":
		if lease.Holder != nil || !validHexDigest(lease.ClaimTokenSHA256, 64) {
			return fmt.Errorf("claimable lease requires no holder and one token hash")
		}
	case "released":
		if lease.Holder != nil || lease.ClaimTokenSHA256 != "" {
			return fmt.Errorf("released lease must not retain a holder or token hash")
		}
	default:
		return fmt.Errorf("unsupported lease status %q", lease.Status)
	}
	return nil
}

func ValidatePersistedActor(actor leasecontract.Actor) error {
	if actor.Host != "codex" && actor.Host != "claude" && actor.Host != "omo" && actor.Host != "omp" {
		return fmt.Errorf("native actor host must be codex, claude, omo, or omp")
	}
	if strings.TrimSpace(actor.SessionID) == "" {
		return fmt.Errorf("native actor session_id is required")
	}
	if actor.SessionProcess == nil || actor.SessionProcess.PID <= 0 || actor.SessionProcess.StartedAt == "" || actor.SessionProcess.Executable == "" {
		return fmt.Errorf("native actor requires a PID reuse-safe session_process receipt")
	}
	return nil
}

func ValidatePersistedSidecars(execution leasecontract.Execution) error {
	if execution.Mode == "direct" && execution.Orca != nil {
		return fmt.Errorf("direct execution must not contain an Orca binding")
	}
	if execution.Orca != nil {
		if err := validateOrcaBinding(execution.Orca); err != nil {
			return err
		}
	}
	if err := validatePendingIntent(execution.Pending); err != nil {
		return err
	}
	if err := validateExecutionCompletionSidecars(execution); err != nil {
		return err
	}
	if err := validateFailureDetail(execution.Failure); err != nil {
		return err
	}
	if execution.SyncBaseResolution != nil {
		if err := validateSyncBaseResolution(execution, execution.SyncBaseResolution); err != nil {
			return err
		}
	}
	for _, event := range execution.SyncBaseEvents {
		if err := validateSyncBaseEvent(event); err != nil {
			return err
		}
	}
	return nil
}

func validatePendingIntent(pending *leasecontract.ExternalIntent) error {
	if pending != nil && (pending.OperationID == "" || pending.Kind == "" || pending.Marker == "" || pending.StartedAt == "") {
		return fmt.Errorf("pending external intent is incomplete")
	}
	return nil
}

func validateFailureDetail(failure *leasecontract.FailureDetail) error {
	if failure != nil && (failure.Code == "" || failure.At == "" || len(failure.Message) > 4096) {
		return fmt.Errorf("execution failure is invalid")
	}
	return nil
}

func validateOrcaBinding(orca *leasecontract.OrcaBinding) error {
	for _, field := range []struct{ name, value string }{
		{"runtime_id", orca.RuntimeID},
		{"repo_id", orca.RepoID},
		{"worktree_id", orca.WorktreeID},
		{"owner_host", orca.OwnerHost},
		{"owner_model", orca.OwnerModel},
		{"task_id", orca.TaskID},
		{"dispatch_id", orca.DispatchID},
		{"run_id", orca.RunID},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("Orca binding %s is required", field.name)
		}
	}
	if orca.LeaseGeneration == 0 {
		return fmt.Errorf("Orca binding lease_generation is required")
	}
	if orca.ArtifactIdentityVersion != leasecontract.OrcaArtifactIdentityVersion {
		return fmt.Errorf("unsupported Orca artifact identity version %d", orca.ArtifactIdentityVersion)
	}
	for _, digest := range []string{orca.IssueBodySHA256, orca.ContextPacketSHA256, orca.OwnerPromptSHA256} {
		if digest == "" {
			return fmt.Errorf("Orca binding artifact identity version requires a complete sealed artifact identity")
		}
		if !validHexDigest(digest, 64) {
			return fmt.Errorf("Orca binding sealed artifact identity must contain SHA-256 digests")
		}
	}
	return nil
}

func validateExecutionCompletionSidecars(execution leasecontract.Execution) error {
	if execution.Completion != nil {
		if err := validateCompletion(*execution.Completion); err != nil {
			return err
		}
		if execution.Completion.Generation > execution.Lease.Generation {
			return fmt.Errorf("execution completion generation exceeds the lease generation")
		}
	}
	for _, entry := range execution.CompletionHistory {
		if err := validateCompletionHistoryEntry(entry, execution.Lease.Generation); err != nil {
			return err
		}
	}
	return nil
}

func validateCompletionHistoryEntry(entry leasecontract.CompletionHistoryEntry, leaseGeneration uint64) error {
	if entry.Generation == 0 || strings.TrimSpace(entry.Reason) == "" || strings.TrimSpace(entry.ReopenedAt) == "" {
		return fmt.Errorf("execution completion history entry is incomplete")
	}
	if entry.Generation >= leaseGeneration {
		return fmt.Errorf("execution completion history generation must precede the lease generation")
	}
	if err := validateCompletion(entry.Completion); err != nil {
		return fmt.Errorf("execution completion history: %w", err)
	}
	if entry.Completion.Generation != entry.Generation {
		return fmt.Errorf("execution completion history generation conflicts with its completion")
	}
	return nil
}

func validateSyncBaseResolution(execution leasecontract.Execution, resolution *leasecontract.SyncBaseResolution) error {
	if !syncBaseResolutionMatchesExecution(execution, resolution) ||
		!validHexDigest(resolution.BaseOID, 40, 64) || strings.TrimSpace(resolution.StartedAt) == "" || len(resolution.ConflictFiles) == 0 {
		return fmt.Errorf("execution sync-base resolution is invalid")
	}
	if err := ValidatePersistedActor(resolution.Actor); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, path := range resolution.ConflictFiles {
		clean := filepath.Clean(path)
		if !validSyncBaseConflictPath(path, clean) || seen[clean] {
			return fmt.Errorf("execution sync-base resolution conflict path is invalid")
		}
		seen[clean] = true
	}
	return nil
}

func syncBaseResolutionMatchesExecution(execution leasecontract.Execution, resolution *leasecontract.SyncBaseResolution) bool {
	return execution.Lease.Status == "released" && execution.Completion != nil &&
		resolution.Generation != 0 && resolution.Generation == execution.Lease.Generation &&
		resolution.CompletionGeneration != 0 && resolution.CompletionGeneration == execution.Completion.Generation
}

func validSyncBaseConflictPath(path, clean string) bool {
	return path != "" && path == clean && !filepath.IsAbs(path) && clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func validateSyncBaseEvent(event leasecontract.SyncBaseEvent) error {
	if event.Mode != "apply" && event.Mode != "finalize" {
		return fmt.Errorf("execution sync-base event mode must be apply or finalize")
	}
	if !validHexDigest(event.BaseOID, 40, 64) || !validHexDigest(event.MergeCommit, 40, 64) {
		return fmt.Errorf("execution sync-base event requires full base and merge commit OIDs")
	}
	if strings.TrimSpace(event.BaseBranch) == "" || strings.TrimSpace(event.Actor) == "" || strings.TrimSpace(event.At) == "" {
		return fmt.Errorf("execution sync-base event is incomplete")
	}
	if event.ConflictFiles < 0 {
		return fmt.Errorf("execution sync-base event conflict count must not be negative")
	}
	return nil
}

func validateCompletion(completion leasecontract.Completion) error {
	if completion.Generation == 0 || !validHexDigest(completion.FinalHead, 40, 64) || strings.TrimSpace(completion.VerificationReportPath) == "" || len(completion.Verification) == 0 || strings.TrimSpace(completion.RemoteArtifactURL) == "" || strings.TrimSpace(completion.CompletedAt) == "" {
		return fmt.Errorf("execution completion is incomplete")
	}
	for _, evidence := range completion.Verification {
		if strings.TrimSpace(evidence) == "" {
			return fmt.Errorf("execution completion verification must be nonempty")
		}
	}
	return nil
}
func validHexDigest(value string, sizes ...int) bool {
	valid := slices.Contains(sizes, len(value))
	if !valid {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
