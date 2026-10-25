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
	// BaseRewritten: the base ref holds neither the sealed base nor its
	// content (a squash-merged parent's content counts as held).
	BaseRewritten  bool
	Upstream       string
	SyncUpstream   bool
	FetchFailed    bool
	FetchStderr    string
	UpstreamCounts string
}

func PRGitReadiness(record model.IssueOpsRecord, facts PRGitFacts) ([]string, []string) {
	missing := []string{}
	warnings := []string{}
	if !facts.RootAvailable {
		missing = append(missing, "repo")
	} else if !facts.GitWorktree {
		missing = append(missing, "repo_git")
	} else {
		missing, warnings = prWorktreeReadiness(record, facts, missing, warnings)
	}
	if record.SourceMisdirectWarnings > 0 {
		warnings = append(warnings, fmt.Sprintf("source_misdirect_warnings:%d", record.SourceMisdirectWarnings))
	}
	return missing, warnings
}

// prWorktreeReadiness는 git worktree가 확인된 뒤의 브랜치/청결/upstream 사실을 누적한다.
func prWorktreeReadiness(record model.IssueOpsRecord, facts PRGitFacts, missing, warnings []string) ([]string, []string) {
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
	if facts.BaseRewritten {
		warnings = append(warnings, "base_rewritten: "+facts.BaseRemoteRef+" no longer contains the sealed base or its content (rewritten, e.g. force-pushed); a PR from this branch would bring the dropped commits back. Ask the user before opening or merging it: keep the base as is, or start a new cycle from the rewritten base")
	} else if facts.BaseAdvanced {
		warnings = append(warnings, "base_advanced: "+facts.BaseRemoteRef+" is not an ancestor of HEAD; run issueops execution sync-base --id "+record.ID+" --preview")
	}
	if facts.Upstream == "" {
		return append(missing, "upstream"), warnings
	}
	if facts.SyncUpstream {
		missing, warnings = prUpstreamSyncReadiness(facts, missing, warnings)
	}
	return missing, warnings
}

func prUpstreamSyncReadiness(facts PRGitFacts, missing, warnings []string) ([]string, []string) {
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
	return missing, warnings
}
