package operationalhealth

import (
	"slices"
	"testing"
	"time"

	operationalhealthcontract "issueops/internal/contract/operationalhealth"
)

func TestEvaluateCycleAuthorityPreservationAndLeaselessBoundaries(t *testing.T) {
	active := healthyDirectSnapshot().Cycles[0]
	leaseless := operationalhealthcontract.Cycle{ID: "io-plan", Repo: "/repo", Branch: "69-plan", Phase: "plan"}
	preserve := func(ids ...string) Options { return Options{PreserveCycleIDs: ids} }

	for _, test := range []struct {
		name  string
		cycle operationalhealthcontract.Cycle
		opts  Options
		want  operationalhealthcontract.CycleAuthority
	}{
		{"missing repo", withCycle(leaseless, func(c *operationalhealthcontract.Cycle) { c.Repo = " " }), Options{}, AuthorityUnknown},
		{"missing id", withCycle(leaseless, func(c *operationalhealthcontract.Cycle) { c.ID = "" }), Options{}, AuthorityUnknown},
		{"unsupported phase", withCycle(leaseless, func(c *operationalhealthcontract.Cycle) { c.Phase = "merge" }), Options{}, AuthorityUnknown},
		{"leaseless done phase", withCycle(leaseless, func(c *operationalhealthcontract.Cycle) { c.Phase = "done" }), Options{}, AuthorityUnknown},
		{"leaseless with execution projection", withCycle(leaseless, func(c *operationalhealthcontract.Cycle) { c.Generation = 2 }), Options{}, AuthorityUnknown},
		{"leaseless preserved by id", leaseless, preserve("io-plan"), AuthorityPreserved},
		{"leaseless preserved but branchless", withCycle(leaseless, func(c *operationalhealthcontract.Cycle) { c.Branch = "" }), preserve("io-plan"), AuthorityUnknown},
		{"active live process preserved by id", active, preserve("io-v1"), AuthorityPreserved},
		{"active dead process cannot be preserved", withCycle(active, func(c *operationalhealthcontract.Cycle) {
			c.HolderProcessStatus = operationalhealthcontract.ProcessStatusDead
		}), preserve("io-v1"), AuthorityUnknown},
		{"claimable preserved by id", withCycle(active, func(c *operationalhealthcontract.Cycle) { c.LeaseStatus = "claimable"; clearCycleHolder(c) }), preserve("io-v1"), AuthorityPreserved},
		{"claimable with a holder is incomplete", withCycle(active, func(c *operationalhealthcontract.Cycle) { c.LeaseStatus = "claimable" }), preserve("io-v1"), AuthorityUnknown},
		{"revoking without holder", withCycle(active, func(c *operationalhealthcontract.Cycle) { c.LeaseStatus = "revoking"; clearCycleHolder(c) }), Options{}, AuthorityUnknown},
		{"completion on a non-done cycle", withCycle(active, func(c *operationalhealthcontract.Cycle) { c.CompletionPresent = true }), Options{}, AuthorityUnknown},
		{"direct mode with Orca identity", withCycle(active, func(c *operationalhealthcontract.Cycle) { c.TaskID = "task" }), Options{}, AuthorityUnknown},
		{"zero generation", withCycle(active, func(c *operationalhealthcontract.Cycle) { c.Generation = 0 }), Options{}, AuthorityUnknown},
		{"unsupported native host", withCycle(active, func(c *operationalhealthcontract.Cycle) { c.HolderHost = "vim" }), Options{}, AuthorityUnknown},
		{"released done but holder retained", withCycle(active, func(c *operationalhealthcontract.Cycle) {
			c.Phase, c.LeaseStatus, c.CompletionPresent = "done", "released", true
		}), Options{}, AuthorityUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := EvaluateCycleAuthority(test.cycle, test.opts); got != test.want {
				t.Fatalf("authority = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSeverityForFindingEscalatesOnlyUnknownInventory(t *testing.T) {
	if got := SeverityForFinding(operationalhealthcontract.FindingInventoryUnknown); got != "error" {
		t.Fatalf("unknown inventory severity = %q", got)
	}
	for _, code := range []string{operationalhealthcontract.FindingDeadOwner, operationalhealthcontract.FindingMessageResidue, "anything-else"} {
		if got := SeverityForFinding(code); got != "warning" {
			t.Errorf("severity(%s) = %q", code, got)
		}
	}
}

func TestIssueCreateIntentNeedsReconciliationByStatus(t *testing.T) {
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	fresh := now.Add(-time.Minute).Format(time.RFC3339Nano)
	for _, status := range []string{"invoked_unknown", "url_observed", "verification_failed", "receipt_failed"} {
		if !IssueCreateIntentNeedsReconciliationAt(status, fresh, "", now) {
			t.Errorf("%s must always need reconciliation", status)
		}
	}
	for _, status := range []string{"completed", "", "unknown"} {
		if IssueCreateIntentNeedsReconciliationAt(status, "invalid", "", now) {
			t.Errorf("%q is not a reconcilable status", status)
		}
	}
	// updated_at is blank, so started_at is the clock that ages the pending intent.
	if IssueCreateIntentNeedsReconciliationAt("pending", " ", fresh, now) {
		t.Error("recent started_at keeps a pending intent healthy")
	}
	stale := now.Add(-time.Hour).Format(time.RFC3339Nano)
	if !IssueCreateIntentNeedsReconciliationAt("pending", "", stale, now) {
		t.Error("stale started_at must need reconciliation")
	}
	if !IssueCreateIntentNeedsReconciliationAt("pending", "", "", now) {
		t.Error("pending intent with no timestamp cannot prove freshness")
	}
	if IssueCreateIntentNeedsReconciliationAt("pending", fresh, stale, now) {
		t.Error("updated_at takes precedence over started_at")
	}
}

func findingIDs(result operationalhealthcontract.Result, code, kind string) []string {
	var ids []string
	for _, finding := range result.Findings {
		if finding.Code == code && finding.ResourceKind == kind {
			ids = append(ids, finding.ResourceID)
		}
	}
	return ids
}

func TestClassifyRejectsInvalidPreservationRequests(t *testing.T) {
	snapshot := healthyDirectSnapshot()
	snapshot.Terminals = []operationalhealthcontract.OrcaTerminal{{RuntimeID: "", Handle: "user-shell"}}
	result := Classify(snapshot, Options{
		Now:                     time.Now(),
		PreserveCycleIDs:        []string{"io-v1", "io-v1", " ", "io-ghost"},
		PreserveTerminalHandles: []string{"user-shell", "user-shell", "", "missing-term"},
	})
	unknown := operationalhealthcontract.FindingInventoryUnknown
	if got := findingIDs(result, unknown, "preserve_cycle"); !slices.Equal(got, []string{"", "io-v1"}) {
		t.Errorf("invalid preserved cycles = %q, want blank and duplicate", got)
	}
	if got := findingIDs(result, unknown, "preserve_terminal"); !slices.Equal(got, []string{"", "user-shell"}) {
		t.Errorf("invalid preserved terminals = %q, want blank and duplicate", got)
	}
	if got := findingIDs(result, unknown, "cycle"); len(got) != 1 || got[0] != "io-ghost" {
		t.Errorf("preserved cycle that does not exist must be flagged once, got %v", got)
	}
	if got := findingIDs(result, unknown, "terminal"); len(got) != 1 || got[0] != "missing-term" {
		t.Errorf("preserved terminal that does not exist must be flagged once, got %v", got)
	}
}

func TestClassifyMessageObservationOutcomes(t *testing.T) {
	for _, test := range []struct {
		name     string
		messages operationalhealthcontract.MessagePresence
		profile  string
		code     string
	}{
		{"negative count", operationalhealthcontract.MessagePresence{Count: -1}, operationalhealthcontract.ProfileInteractive, operationalhealthcontract.FindingInventoryUnknown},
		{"rows present", operationalhealthcontract.MessagePresence{Count: 3}, "", operationalhealthcontract.FindingMessageResidue},
		{"not empty but zero count", operationalhealthcontract.MessagePresence{Empty: false, CompleteAbsence: true}, "", operationalhealthcontract.FindingMessageResidue},
		{"empty but absence unproven", operationalhealthcontract.MessagePresence{Empty: true}, "", operationalhealthcontract.FindingInventoryUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := healthyDirectSnapshot()
			snapshot.Messages = test.messages
			result := Classify(snapshot, Options{Now: time.Now(), Profile: test.profile})
			if !hasFinding(result, test.code, "message") || len(result.Findings) != 1 {
				t.Fatalf("findings = %+v, want exactly one %s message finding", result.Findings, test.code)
			}
		})
	}
}

func TestClassifyFlagsCanonicalSourceDefects(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Snapshot)
		kind   string
	}{
		{"missing identity", func(s *Snapshot) { s.SourceHead = "" }, "source"},
		{"dirty checkout", func(s *Snapshot) { s.SourceClean = false }, "source"},
		{"canonical worktree on another branch", func(s *Snapshot) { s.GitWorktrees[0].Branch = "dev" }, "source_worktree"},
		{"canonical worktree dirty", func(s *Snapshot) { s.GitWorktrees[0].Clean = false }, "source_worktree"},
		{"canonical worktree head drift", func(s *Snapshot) { s.GitWorktrees[0].Head = "ffff" }, "source_worktree"},
		{"local ref drift", func(s *Snapshot) { s.LocalRefs[0].OID = "ffff" }, "branch"},
		{"remote ref duplicated", func(s *Snapshot) { s.RemoteRefs = append(s.RemoteRefs, s.RemoteRefs[0]) }, "branch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := healthyDirectSnapshot()
			test.mutate(&snapshot)
			result := Classify(snapshot, Options{Now: time.Now()})
			if !hasFinding(result, operationalhealthcontract.FindingInventoryUnknown, test.kind) {
				t.Fatalf("want unknown-inventory %s finding, got %+v", test.kind, result.Findings)
			}
		})
	}
}

func TestClassifyReportsDurableFailuresOnlyAsTheirOwnFinding(t *testing.T) {
	for code, mutate := range map[string]func(*operationalhealthcontract.Cycle){
		operationalhealthcontract.FindingExecutionFailure:   func(c *operationalhealthcontract.Cycle) { c.ExecutionFailurePresent = true },
		operationalhealthcontract.FindingCleanupFailure:     func(c *operationalhealthcontract.Cycle) { c.CleanupFailurePresent = true },
		operationalhealthcontract.FindingIssueCreateFailure: func(c *operationalhealthcontract.Cycle) { c.IssueCreateFailurePresent = true },
	} {
		snapshot := healthyDirectSnapshot()
		mutate(&snapshot.Cycles[0])
		result := Classify(snapshot, Options{Now: time.Now()})
		if len(result.Findings) != 1 || !hasFinding(result, code, "cycle") || result.Healthy {
			t.Errorf("%s: findings = %+v", code, result.Findings)
		}
	}
}

func TestClassifyFlagsDeadOwnerAndItsResidue(t *testing.T) {
	snapshot := healthyDirectSnapshot()
	snapshot.Cycles[0].HolderProcessStatus = operationalhealthcontract.ProcessStatusDead
	result := Classify(snapshot, Options{Now: time.Now()})
	if !hasFinding(result, operationalhealthcontract.FindingDeadOwner, "cycle") {
		t.Fatalf("dead holder must be reported: %+v", result.Findings)
	}
	if !hasFinding(result, operationalhealthcontract.FindingWorktreeResidue, "git_worktree") {
		t.Fatalf("a dead cycle no longer owns its worktree: %+v", result.Findings)
	}
}

func TestClassifyFlagsUnownedNonMainBranchRefs(t *testing.T) {
	snapshot := healthyDirectSnapshot()
	head := snapshot.SourceHead
	snapshot.LocalRefs = append(snapshot.LocalRefs, operationalhealthcontract.GitRef{Name: "refs/heads/old", Branch: "old", OID: head, Location: "local"})
	snapshot.RemoteRefs = append(snapshot.RemoteRefs, operationalhealthcontract.GitRef{Name: "refs/remotes/origin/69-v1", Branch: "69-v1", OID: head, Location: "remote"})
	result := Classify(snapshot, Options{Now: time.Now()})
	var summaries []string
	for _, finding := range result.Findings {
		if finding.Code == operationalhealthcontract.FindingNonMainBranchResidue {
			summaries = append(summaries, finding.Summary)
		}
	}
	if len(summaries) != 1 {
		t.Fatalf("want one branch residue finding, got %+v", result.Findings)
	}
	want := "1 non-canonical refs have no live or invocation-preserved owner: local:refs/heads/old@" + head
	if summaries[0] != want {
		t.Fatalf("summary = %q, want %q (the active cycle's own branch must not be residue)", summaries[0], want)
	}
}

func TestClassifyRequiresCycleGitWorktreeToMatchBranch(t *testing.T) {
	snapshot := healthyDirectSnapshot()
	snapshot.GitWorktrees[1].Branch = "other"
	result := Classify(snapshot, Options{Now: time.Now()})
	if !hasFinding(result, operationalhealthcontract.FindingInventoryUnknown, "git_worktree") {
		t.Fatalf("branch mismatch must be unknown inventory: %+v", result.Findings)
	}
}
