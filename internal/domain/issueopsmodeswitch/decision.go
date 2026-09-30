// Package issueopsmodeswitch owns the pure mode-switch eligibility rules.
package issueopsmodeswitch

import (
	"fmt"
	"strings"
)

type Facts struct {
	CurrentMode            string
	RequestedMode          string
	WriterPresent          bool
	PendingIntent          bool
	WorktreePresent        bool
	WorktreeClean          bool
	NoUnpushedCommits      bool
	OrcaRemoteBranchExists bool
}

func NormalizeMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "direct":
		return "direct", nil
	case "orca":
		return "orca", nil
	default:
		return "", fmt.Errorf("execution switch-mode requires an explicit --mode direct or orca")
	}
}

func MissingGates(facts Facts) []string {
	missing := []string{}
	if facts.CurrentMode == facts.RequestedMode {
		missing = append(missing, "mode_actually_changes")
	}
	if facts.WriterPresent {
		missing = append(missing, "lease_holds_no_writer")
	}
	if facts.PendingIntent {
		missing = append(missing, "pending_intent_absent")
	}
	if facts.WorktreePresent {
		if !facts.WorktreeClean {
			missing = append(missing, "worktree_clean")
		}
		if !facts.NoUnpushedCommits {
			missing = append(missing, "worktree_commits_pushed")
		}
	}
	if facts.RequestedMode == "orca" && facts.OrcaRemoteBranchExists {
		missing = append(missing, "orca_branch_name_free")
	}
	return missing
}
