package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

type PRGitFacts struct {
	RootAvailable       bool
	GitWorktree         bool
	FingerprintVerified bool
	CurrentBranch       string
	WorktreeClean       bool
	BaseRemoteRef       string
	BaseAdvanced        bool
	Upstream            string
	SyncUpstream        bool
	FetchFailed         bool
	FetchStderr         string
	UpstreamCounts      string
}

func PRGitReadiness(record model.IssueOpsRecord, facts PRGitFacts) ([]string, []string) {
	missing := []string{}
	warnings := []string{}
	if !facts.RootAvailable {
		missing = append(missing, "repo")
	} else if !facts.GitWorktree {
		missing = append(missing, "repo_git")
	} else {
		if !facts.FingerprintVerified {
			missing = append(missing, "current_fingerprint")
		}
		if branch := strings.TrimSpace(record.Branch); branch != "" && facts.CurrentBranch != branch {
			missing = append(missing, "branch_match")
			warnings = append(warnings, "current branch "+facts.CurrentBranch+" does not match IssueOps branch "+branch)
		}
		if !facts.WorktreeClean {
			missing = append(missing, "worktree_clean")
		}
		if facts.BaseAdvanced {
			warnings = append(warnings, "base_advanced: "+facts.BaseRemoteRef+" is not an ancestor of HEAD; run issueops execution sync-base --id "+record.ID+" --preview")
		}
		if facts.Upstream == "" {
			missing = append(missing, "upstream")
		} else if facts.SyncUpstream {
			if facts.FetchFailed {
				missing = append(missing, "upstream_fetch")
				if facts.FetchStderr != "" {
					warnings = append(warnings, "failed to fetch upstream: "+facts.FetchStderr)
				}
			}
			counts := strings.Fields(facts.UpstreamCounts)
			if len(counts) != 2 || counts[0] != "0" || counts[1] != "0" {
				missing = append(missing, "upstream_synced")
				if len(counts) == 2 {
					warnings = append(warnings, "branch divergence against upstream: ahead="+counts[0]+" behind="+counts[1])
				}
			}
		}
	}
	if record.SourceMisdirectWarnings > 0 {
		warnings = append(warnings, fmt.Sprintf("source_misdirect_warnings:%d", record.SourceMisdirectWarnings))
	}
	return missing, warnings
}
