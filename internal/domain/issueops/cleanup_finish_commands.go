package issueops

import (
	"fmt"
	"slices"
	"strings"

	model "issueops/internal/contract/issueops"
)

func CleanupFinishRemedyCommand(id string, missing []string) string {
	if slices.Contains(missing, "completion_reflected") {
		return fmt.Sprintf("issueops remote reflect-completion --id %s --confirm --json", id)
	}
	return ""
}
func CleanupFinishApplyCommand(id, fingerprint, supersededBy string, keepRemote bool) string {
	replacement := ""
	if value := strings.TrimSpace(supersededBy); value != "" {
		replacement = " --superseded-by '" + strings.ReplaceAll(value, "'", "'\\''") + "'"
	}
	keep := ""
	if keepRemote {
		keep = " --keep-remote-branch"
	}
	return fmt.Sprintf("issueops cleanup finish --id %s --apply --confirm --fingerprint %s%s%s --json", id, fingerprint, replacement, keep)
}
func ValidateCleanupFinishApply(req model.CleanupFinishRequest, fingerprint string) error {
	if !req.Confirm {
		return fmt.Errorf("cleanup finish --apply requires --confirm")
	}
	if req.Fingerprint != fingerprint {
		return fmt.Errorf("stale cleanup fingerprint; run --preview again and retry with the new value")
	}
	return nil
}
func CleanupFinishAudit(inventory model.CleanupFinishInventory, result model.CleanupFinishResult, now string) string {
	kept := ""
	if branch := result.KeptRemoteBranch; branch != nil {
		tip := branch.RemoteOID
		if tip == "" {
			tip = branch.State
		}
		kept = fmt.Sprintf(" remote_branch_kept=%s@%s", branch.Branch, tip)
	}
	none := func(value string) string {
		if strings.TrimSpace(value) == "" {
			return "(없음)"
		}
		return value
	}
	return fmt.Sprintf("cleanup 완료: worktree=%s branch=%s oid=%s stopped=%d terminals=%d%s at=%s", none(inventory.WorktreeRoot), none(inventory.Branch), none(inventory.BranchOID), len(result.WorkspaceProcessesStopped), result.OrcaTerminalsStopped, kept, now)
}
