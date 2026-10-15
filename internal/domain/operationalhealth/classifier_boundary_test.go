package operationalhealth

import (
	"slices"
	"strings"
	"testing"
	"time"

	operationalhealthcontract "issueops/internal/contract/operationalhealth"
)

func hasFindingSummary(result operationalhealthcontract.Result, code, kind, summaryPart string) bool {
	return slices.ContainsFunc(result.Findings, func(f operationalhealthcontract.Finding) bool {
		return f.Code == code && f.ResourceKind == kind && strings.Contains(f.Summary, summaryPart)
	})
}

func TestClassifyGitWorktreeCanonicalExemption(t *testing.T) {
	tests := []struct {
		name      string
		worktree  operationalhealthcontract.GitWorktree
		wantResid bool
	}{
		{"flagged canonical", operationalhealthcontract.GitWorktree{Path: "/other", Branch: "x", Canonical: true}, false},
		{"repo root on canonical branch", operationalhealthcontract.GitWorktree{Path: "/repo", Branch: "main"}, false},
		{"repo root on another branch", operationalhealthcontract.GitWorktree{Path: "/repo", Branch: "feature"}, true},
		{"another path on canonical branch", operationalhealthcontract.GitWorktree{Path: "/elsewhere", Branch: "main"}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := healthySnapshot()
			snapshot.GitWorktrees = append(snapshot.GitWorktrees, test.worktree)
			result := Classify(snapshot, Options{Now: time.Now()})
			got := slices.ContainsFunc(result.Findings, func(f operationalhealthcontract.Finding) bool {
				return f.Code == operationalhealthcontract.FindingWorktreeResidue && f.ResourceKind == "git_worktree" && f.ResourceID == test.worktree.Path
			})
			if got != test.wantResid {
				t.Fatalf("worktree residue = %v, want %v: %+v", got, test.wantResid, result.Findings)
			}
		})
	}
}

func TestClassifyCycleGitWorktreeCheckSkippedForNonLiveWithoutPath(t *testing.T) {
	const summary = "cycle identity does not match exactly one Git worktree"
	t.Run("non-live with path is validated", func(t *testing.T) {
		snapshot := healthySnapshot()
		snapshot.GitWorktrees = snapshot.GitWorktrees[:1]
		result := Classify(snapshot, Options{Now: time.Now(), PreserveCycleIDs: []string{snapshot.Cycles[0].ID}})
		if !hasFindingSummary(result, operationalhealthcontract.FindingInventoryUnknown, "git_worktree", summary) {
			t.Fatalf("missing Git worktree must be reported: %+v", result.Findings)
		}
	})
	t.Run("non-live without path is skipped", func(t *testing.T) {
		snapshot := healthySnapshot()
		clearCycleHolder(&snapshot.Cycles[0])
		snapshot.Cycles[0].LeaseStatus = ""
		snapshot.Cycles[0].Generation, snapshot.Cycles[0].WorktreePath = 0, ""
		result := Classify(snapshot, Options{Now: time.Now(), PreserveCycleIDs: []string{snapshot.Cycles[0].ID}})
		if hasFindingSummary(result, operationalhealthcontract.FindingInventoryUnknown, "git_worktree", summary) {
			t.Fatalf("pathless preserved cycle must not be matched to a worktree: %+v", result.Findings)
		}
	})
}

func TestExecutionIdentityCompleteRequiresBranchWorktreeAndGeneration(t *testing.T) {
	base := healthySnapshot().Cycles[0]
	if !executionIdentityComplete(base) {
		t.Fatal("complete execution identity rejected")
	}
	for name, mutate := range map[string]func(*operationalhealthcontract.Cycle){
		"blank branch":    func(c *operationalhealthcontract.Cycle) { c.Branch = " " },
		"blank worktree":  func(c *operationalhealthcontract.Cycle) { c.WorktreePath = "" },
		"zero generation": func(c *operationalhealthcontract.Cycle) { c.Generation = 0 },
	} {
		cycle := base
		mutate(&cycle)
		if executionIdentityComplete(cycle) {
			t.Errorf("%s: incomplete execution identity accepted", name)
		}
	}
}

func TestLeaseHolderIndexCompleteRequiresEveryField(t *testing.T) {
	complete := operationalhealthcontract.LeaseHolderIndex{Key: "k", LifecycleID: "io", Generation: 1, Host: "codex", SessionID: "s"}
	if !leaseHolderIndexComplete(complete) {
		t.Fatal("complete index rejected")
	}
	for name, mutate := range map[string]func(*operationalhealthcontract.LeaseHolderIndex){
		"zero generation": func(i *operationalhealthcontract.LeaseHolderIndex) { i.Generation = 0 },
		"empty key":       func(i *operationalhealthcontract.LeaseHolderIndex) { i.Key = " " },
		"empty lifecycle": func(i *operationalhealthcontract.LeaseHolderIndex) { i.LifecycleID = "" },
		"unknown host":    func(i *operationalhealthcontract.LeaseHolderIndex) { i.Host = "gemini" },
		"empty session":   func(i *operationalhealthcontract.LeaseHolderIndex) { i.SessionID = "" },
	} {
		index := complete
		mutate(&index)
		if leaseHolderIndexComplete(index) {
			t.Errorf("%s: incomplete index accepted", name)
		}
	}
}
