package issueopslease

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
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
	for name, value := range map[string]string{"source_root": execution.Workspace.SourceRoot, "root": execution.Workspace.Root, "branch": execution.Workspace.Branch, "base_head": execution.Workspace.BaseHead, "driver": execution.Workspace.Driver, "linked_at": execution.Workspace.LinkedAt} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("workspace %s is required", name)
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
	if execution.Selection != nil {
		if err := ValidatePersistedSelection(*execution.Selection, execution.Mode); err != nil {
			return err
		}
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
	if (selection.RequestedMode == "direct" && mode != "direct") ||
		(selection.RequestedMode == "orca" && mode != "orca") {
		return fmt.Errorf("explicit selection mode must equal execution mode")
	}
	if mode == "orca" && !selection.ProbeReady {
		return fmt.Errorf("Orca selection requires a ready probe")
	}
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
	if selection.RequestedMode == "direct" {
		if selection.ProbeAttempted || strings.TrimSpace(selection.ExplicitDirectReason) == "" {
			return fmt.Errorf("explicit direct selection requires a reason and no probe")
		}
		if selection.FallbackCode != "" {
			return fmt.Errorf("explicit direct selection must not contain fallback_code")
		}
	} else if strings.TrimSpace(selection.ExplicitDirectReason) != "" {
		return fmt.Errorf("non-direct selection must not contain explicit_direct_reason")
	}
	if !validHexDigest(selection.ReadinessFingerprint, 64) || strings.TrimSpace(selection.SelectedAt) == "" {
		return fmt.Errorf("selection receipt requires fingerprint and selected_at")
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
	if actor.Host != "codex" && actor.Host != "claude" && actor.Host != "omo" {
		return fmt.Errorf("native actor host must be codex, claude, or omo")
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
		for name, value := range map[string]string{"runtime_id": execution.Orca.RuntimeID, "repo_id": execution.Orca.RepoID, "worktree_id": execution.Orca.WorktreeID, "owner_host": execution.Orca.OwnerHost, "owner_model": execution.Orca.OwnerModel, "task_id": execution.Orca.TaskID, "dispatch_id": execution.Orca.DispatchID} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("Orca binding %s is required", name)
			}
		}
		digests := []string{execution.Orca.IssueBodySHA256, execution.Orca.ContextPacketSHA256, execution.Orca.OwnerPromptSHA256}
		present := 0
		for _, digest := range digests {
			if digest != "" {
				present++
			}
		}
		if present != 0 && present != len(digests) {
			return fmt.Errorf("Orca binding requires a complete sealed artifact identity")
		}
		switch execution.Orca.ArtifactIdentityVersion {
		case 0:
			if present != 0 {
				return fmt.Errorf("Orca binding sealed artifact identity requires artifact identity version")
			}
		case leasecontract.OrcaArtifactIdentityVersion:
			if present != len(digests) {
				return fmt.Errorf("Orca binding artifact identity version requires a complete sealed artifact identity")
			}
		default:
			return fmt.Errorf("unsupported Orca artifact identity version %d", execution.Orca.ArtifactIdentityVersion)
		}
		if present == len(digests) {
			for _, digest := range digests {
				if !validHexDigest(digest, 64) {
					return fmt.Errorf("Orca binding sealed artifact identity must contain SHA-256 digests")
				}
			}
		}
	}
	if execution.Pending != nil && (execution.Pending.OperationID == "" || execution.Pending.Kind == "" || execution.Pending.Marker == "" || execution.Pending.StartedAt == "") {
		return fmt.Errorf("pending external intent is incomplete")
	}
	if execution.Completion != nil {
		if err := validateCompletion(*execution.Completion); err != nil {
			return err
		}
		if execution.Completion.Generation > execution.Lease.Generation {
			return fmt.Errorf("execution completion generation exceeds the lease generation")
		}
	}
	for _, entry := range execution.CompletionHistory {
		if entry.Generation == 0 || strings.TrimSpace(entry.Reason) == "" || strings.TrimSpace(entry.ReopenedAt) == "" {
			return fmt.Errorf("execution completion history entry is incomplete")
		}
		if entry.Generation >= execution.Lease.Generation {
			return fmt.Errorf("execution completion history generation must precede the lease generation")
		}
		if err := validateCompletion(entry.Completion); err != nil {
			return fmt.Errorf("execution completion history: %w", err)
		}
		if entry.Completion.Generation != 0 && entry.Completion.Generation != entry.Generation {
			return fmt.Errorf("execution completion history generation conflicts with its completion")
		}
	}
	if execution.Failure != nil && (execution.Failure.Code == "" || execution.Failure.At == "" || len(execution.Failure.Message) > 4096) {
		return fmt.Errorf("execution failure is invalid")
	}
	if execution.SyncBaseResolution != nil {
		resolution := execution.SyncBaseResolution
		if execution.Lease.Status != "released" || execution.Completion == nil ||
			resolution.Generation == 0 || resolution.Generation != execution.Lease.Generation ||
			resolution.CompletionGeneration == 0 || resolution.CompletionGeneration != execution.Completion.Generation ||
			!validHexDigest(resolution.BaseOID, 40, 64) || strings.TrimSpace(resolution.StartedAt) == "" || len(resolution.ConflictFiles) == 0 {
			return fmt.Errorf("execution sync-base resolution is invalid")
		}
		if err := ValidatePersistedActor(resolution.Actor); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, path := range resolution.ConflictFiles {
			clean := filepath.Clean(path)
			if path == "" || path != clean || filepath.IsAbs(path) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || seen[clean] {
				return fmt.Errorf("execution sync-base resolution conflict path is invalid")
			}
			seen[clean] = true
		}
	}
	for _, event := range execution.SyncBaseEvents {
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
	}
	return nil
}

func validateCompletion(completion leasecontract.Completion) error {
	if !validHexDigest(completion.FinalHead, 40, 64) || strings.TrimSpace(completion.VerificationReportPath) == "" || len(completion.Verification) == 0 || strings.TrimSpace(completion.RemoteArtifactURL) == "" || strings.TrimSpace(completion.CompletedAt) == "" {
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
	valid := false
	for _, size := range sizes {
		if len(value) == size {
			valid = true
			break
		}
	}
	if !valid {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
