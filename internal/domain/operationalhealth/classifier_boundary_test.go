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

func TestClassifyCycleOrcaIdentityMismatchRules(t *testing.T) {
	const summary = "cycle Orca runtime or repository identity does not match"
	tests := []struct {
		name   string
		mutate func(*operationalhealthcontract.Cycle)
		want   bool
	}{
		{"runtime mismatch", func(c *operationalhealthcontract.Cycle) { c.OrcaRuntimeID = "other" }, true},
		{"repo id mismatch in observed repo", func(c *operationalhealthcontract.Cycle) { c.OrcaRepoID = "other" }, true},
		{"repo id mismatch in another repo", func(c *operationalhealthcontract.Cycle) { c.Repo, c.OrcaRepoID = "/elsewhere", "other" }, false},
		{"no Orca identity at all", func(c *operationalhealthcontract.Cycle) { c.OrcaRuntimeID, c.OrcaRepoID = "", "" }, false},
		{"only repo id present and mismatched", func(c *operationalhealthcontract.Cycle) { c.OrcaRuntimeID, c.OrcaRepoID = "", "other" }, true},
		{"only runtime present and mismatched", func(c *operationalhealthcontract.Cycle) { c.OrcaRuntimeID, c.OrcaRepoID = "other", "" }, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := indexedHealthSnapshot(1)
			test.mutate(&snapshot.Cycles[0])
			result := Classify(snapshot, Options{Now: time.Now()})
			if got := hasFindingSummary(result, operationalhealthcontract.FindingInventoryUnknown, "cycle", summary); got != test.want {
				t.Fatalf("identity mismatch finding = %v, want %v: %+v", got, test.want, result.Findings)
			}
		})
	}
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
			snapshot := healthyDirectSnapshot()
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

func TestExecutionIdentityCompleteAcceptsOnlyKnownOrcaOwnerHosts(t *testing.T) {
	base := indexedHealthSnapshot(1).Cycles[0]
	for _, host := range []string{"codex", "claude", "omo", "omp"} {
		cycle := base
		cycle.OrcaOwnerHost = host
		if !executionIdentityComplete(cycle) {
			t.Errorf("owner host %q rejected", host)
		}
	}
	for _, host := range []string{"", "gemini", "CODEX"} {
		cycle := base
		cycle.OrcaOwnerHost = host
		if executionIdentityComplete(cycle) {
			t.Errorf("owner host %q accepted", host)
		}
	}
}

// preserved(비-live) 사이클은 dispatch/task 상태나 terminal 연결 상태가 달라도 불일치로 보지 않지만, live 사이클은 보고한다.
func TestClassifyLiveOnlyOrcaStatusAndConnectivityChecks(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		kind    string
		summary string
	}{
		{"dispatch status", func(s *Snapshot) { s.Dispatches[0].Status = "completed" }, "dispatch", "cycle dispatch identity"},
		{"terminal disconnected", func(s *Snapshot) { s.Terminals[0].Connected = false }, "terminal", "cycle terminal identity or liveness"},
		{"terminal not writable", func(s *Snapshot) { s.Terminals[0].Writable = false }, "terminal", "cycle terminal identity or liveness"},
		{"task status", func(s *Snapshot) { s.Tasks[0].Status = "completed" }, "task", "cycle task identity or status"},
	}
	for _, test := range tests {
		t.Run(test.name+"/live", func(t *testing.T) {
			snapshot := indexedHealthSnapshot(1)
			test.mutate(&snapshot)
			result := Classify(snapshot, Options{Now: time.Now()})
			if !hasFindingSummary(result, operationalhealthcontract.FindingInventoryUnknown, test.kind, test.summary) {
				t.Fatalf("live cycle must report %s: %+v", test.kind, result.Findings)
			}
		})
		t.Run(test.name+"/preserved", func(t *testing.T) {
			snapshot := indexedHealthSnapshot(1)
			test.mutate(&snapshot)
			result := Classify(snapshot, Options{Now: time.Now(), PreserveCycleIDs: []string{snapshot.Cycles[0].ID}})
			if hasFindingSummary(result, operationalhealthcontract.FindingInventoryUnknown, test.kind, test.summary) {
				t.Fatalf("preserved cycle must not report %s: %+v", test.kind, result.Findings)
			}
		})
	}
}

func TestClassifyCycleGitWorktreeCheckSkippedForNonLiveWithoutPath(t *testing.T) {
	const summary = "cycle identity does not match exactly one Git worktree"
	t.Run("non-live with path is validated", func(t *testing.T) {
		snapshot := healthyDirectSnapshot()
		snapshot.GitWorktrees = snapshot.GitWorktrees[:1]
		result := Classify(snapshot, Options{Now: time.Now(), PreserveCycleIDs: []string{snapshot.Cycles[0].ID}})
		if !hasFindingSummary(result, operationalhealthcontract.FindingInventoryUnknown, "git_worktree", summary) {
			t.Fatalf("missing Git worktree must be reported: %+v", result.Findings)
		}
	})
	t.Run("non-live without path is skipped", func(t *testing.T) {
		snapshot := healthyDirectSnapshot()
		clearCycleHolder(&snapshot.Cycles[0])
		snapshot.Cycles[0].LeaseStatus, snapshot.Cycles[0].ExecutionMode = "", ""
		snapshot.Cycles[0].Generation, snapshot.Cycles[0].WorktreePath = 0, ""
		result := Classify(snapshot, Options{Now: time.Now(), PreserveCycleIDs: []string{snapshot.Cycles[0].ID}})
		if hasFindingSummary(result, operationalhealthcontract.FindingInventoryUnknown, "git_worktree", summary) {
			t.Fatalf("pathless preserved cycle must not be matched to a worktree: %+v", result.Findings)
		}
	})
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
