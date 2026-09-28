package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestPRGitReadinessMissingPreservesObservationOrder(t *testing.T) {
	if missing, warnings := PRGitReadiness(model.IssueOpsRecord{}, PRGitFacts{}); !reflect.DeepEqual(missing, []string{"repo"}) || len(warnings) != 0 {
		t.Fatalf("missing root=%v warnings=%v", missing, warnings)
	}
	if missing, _ := PRGitReadiness(model.IssueOpsRecord{}, PRGitFacts{RootAvailable: true}); !reflect.DeepEqual(missing, []string{"repo_git"}) {
		t.Fatalf("invalid git root=%v", missing)
	}
	record := model.IssueOpsRecord{ID: "io-1", Branch: "feature", SourceMisdirectWarnings: 2}
	facts := PRGitFacts{
		RootAvailable: true, GitWorktree: true, CurrentBranch: "other", BaseRemoteRef: "origin/main", BaseAdvanced: true,
		Upstream: "origin/feature", SyncUpstream: true, FetchFailed: true, FetchStderr: "offline", UpstreamCounts: "1 2",
	}
	missing, warnings := PRGitReadiness(record, facts)
	wantMissing := []string{"current_fingerprint", "branch_match", "worktree_clean", "upstream_fetch", "upstream_synced"}
	wantWarnings := []string{
		"current branch other does not match IssueOps branch feature",
		"base_advanced: origin/main is not an ancestor of HEAD; run issueops execution sync-base --id io-1 --preview",
		"failed to fetch upstream: offline",
		"branch divergence against upstream: ahead=1 behind=2",
		"source_misdirect_warnings:2",
	}
	if !reflect.DeepEqual(missing, wantMissing) || !reflect.DeepEqual(warnings, wantWarnings) {
		t.Fatalf("missing=%v warnings=%v", missing, warnings)
	}
}

func TestPRGitReadinessLocalSkipsSynchronization(t *testing.T) {
	facts := PRGitFacts{RootAvailable: true, GitWorktree: true, FingerprintVerified: true, WorktreeClean: true, Upstream: "origin/feature"}
	missing, warnings := PRGitReadiness(model.IssueOpsRecord{}, facts)
	if len(missing) != 0 || len(warnings) != 0 {
		t.Fatalf("local readiness missing=%v warnings=%v", missing, warnings)
	}
	facts.Upstream = ""
	if missing, _ := PRGitReadiness(model.IssueOpsRecord{}, facts); !reflect.DeepEqual(missing, []string{"upstream"}) {
		t.Fatalf("no upstream=%v", missing)
	}
}
