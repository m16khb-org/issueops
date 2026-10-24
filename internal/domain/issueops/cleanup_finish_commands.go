package issueops

import (
	"fmt"
	"slices"
	"strings"

	model "issueops/internal/contract/issueops"
)

// CleanupFinishRemedyCommand names the command that fills the first missing
// cleanup gate an agent can fill, in the cleanup order: child tasks (the
// remote-branch step requires them closed), remote branch, completion
// reflection, then the issue close.
func CleanupFinishRemedyCommand(id string, missing []string) string {
	switch {
	case slices.Contains(missing, "child_tasks_closed"):
		return fmt.Sprintf("issueops cleanup close-children --id %s --merged --json", id)
	case slices.Contains(missing, "remote_branch_absent"):
		return fmt.Sprintf("issueops cleanup remote-branch --id %s --preview --json", id)
	case slices.Contains(missing, "completion_reflected"):
		return fmt.Sprintf("issueops remote reflect-completion --id %s --body-file <PROGRESS_REPORT_FILE> --confirm --json", id)
	case slices.Contains(missing, "issue_closed"):
		return fmt.Sprintf("issueops remote close-issue --id %s --json", id)
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
	return fmt.Sprintf("cleanup 완료: worktree=%s branch=%s oid=%s stopped=%d%s at=%s", none(inventory.WorktreeRoot), none(inventory.Branch), none(inventory.BranchOID), len(result.WorkspaceProcessesStopped), kept, now)
}
