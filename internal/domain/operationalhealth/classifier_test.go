package operationalhealth

import (
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	"testing"
	"time"
)

func TestEvaluateCycleAuthorityUsesExecutionLease(t *testing.T) {
	base := operationalhealthcontract.Cycle{
		ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: "implement",
		LeaseStatus: "active", Generation: 1,
		WorktreePath: "/repo.worktrees/69-v1", HolderHost: "codex", HolderSessionID: "session",
		HolderPID: 123, HolderStartedAt: "2026-07-22T00:00:00Z", HolderExecutable: "/opt/codex",
		HolderProcessStatus: operationalhealthcontract.ProcessStatusLive,
	}
	tests := []struct {
		name  string
		cycle operationalhealthcontract.Cycle
		want  operationalhealthcontract.CycleAuthority
	}{
		{name: "planning", cycle: operationalhealthcontract.Cycle{ID: "io-plan", Repo: "/repo", Branch: "69-plan", Phase: "plan"}, want: AuthorityPreserved},
		{name: "active", cycle: base, want: AuthorityLive},
		{name: "active dead process", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) {
			c.HolderProcessStatus = operationalhealthcontract.ProcessStatusDead
		}), want: operationalhealthcontract.AuthorityDead},
		{name: "active reused pid", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) {
			c.HolderProcessStatus = operationalhealthcontract.ProcessStatusIdentityMismatch
		}), want: operationalhealthcontract.AuthorityDead},
		{name: "active probe unknown", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) {
			c.HolderProcessStatus = operationalhealthcontract.ProcessStatusUnknown
		}), want: AuthorityUnknown},
		{name: "active missing process", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) { c.HolderPID = 0 }), want: AuthorityUnknown},
		{name: "claimable", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) { c.LeaseStatus = "claimable"; clearCycleHolder(c) }), want: AuthorityPreserved},
		{name: "revoking", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) { c.LeaseStatus = "revoking" }), want: AuthorityPreserved},
		{name: "released", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) { c.LeaseStatus = "released"; clearCycleHolder(c) }), want: AuthorityPreserved},
		{name: "done", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) {
			c.Phase = "done"
			c.LeaseStatus = "released"
			c.CompletionPresent = true
			clearCycleHolder(c)
		}), want: operationalhealthcontract.AuthorityDead},
		{name: "done active lease", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) { c.Phase = "done"; c.CompletionPresent = true }), want: AuthorityUnknown},
		{name: "done missing completion", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) {
			c.Phase = "done"
			c.LeaseStatus = "released"
			clearCycleHolder(c)
		}), want: AuthorityUnknown},
		{name: "invalid lease state", cycle: withCycle(base, func(c *operationalhealthcontract.Cycle) { c.LeaseStatus = "owner_active" }), want: AuthorityUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EvaluateCycleAuthority(test.cycle, Options{Now: time.Now()}); got != test.want {
				t.Fatalf("authority = %q, want %q", got, test.want)
			}
		})
	}
}

func TestClassifyRequiresExactLeaseHolderIndex(t *testing.T) {
	base := healthySnapshot()
	if result := Classify(base, Options{Now: time.Now()}); !result.Healthy {
		t.Fatalf("matched v1 inventory should be healthy: %#v", result.Findings)
	}

	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{name: "missing", mutate: func(snapshot *Snapshot) { snapshot.LeaseHolderIndexes = nil }},
		{name: "orphan", mutate: func(snapshot *Snapshot) {
			snapshot.LeaseHolderIndexes = append(snapshot.LeaseHolderIndexes, operationalhealthcontract.LeaseHolderIndex{Key: "orphan", LifecycleID: "io-orphan", Generation: 1, Host: "codex", SessionID: "other"})
		}},
		{name: "stale generation", mutate: func(snapshot *Snapshot) { snapshot.LeaseHolderIndexes[0].Generation++ }},
		{name: "wrong actor", mutate: func(snapshot *Snapshot) { snapshot.LeaseHolderIndexes[0].SessionID = "other" }},
		{name: "released retains index", mutate: func(snapshot *Snapshot) {
			snapshot.Cycles[0].LeaseStatus = "released"
			clearCycleHolder(&snapshot.Cycles[0])
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := base
			snapshot.Cycles = append([]operationalhealthcontract.Cycle(nil), base.Cycles...)
			snapshot.LeaseHolderIndexes = append([]operationalhealthcontract.LeaseHolderIndex(nil), base.LeaseHolderIndexes...)
			test.mutate(&snapshot)
			result := Classify(snapshot, Options{Now: time.Now()})
			if !hasFinding(result, operationalhealthcontract.FindingInventoryUnknown, "lease_holder") {
				t.Fatalf("findings = %#v, want lease-holder inventory failure", result.Findings)
			}
		})
	}
}

func TestClassifyAcceptsOmoNativeHolderAndIndex(t *testing.T) {
	snapshot := healthySnapshot()
	snapshot.Cycles[0].HolderHost = "omo"
	snapshot.Cycles[0].HolderExecutable = "omo"
	snapshot.LeaseHolderIndexes[0].Host = "omo"
	result := Classify(snapshot, Options{Now: time.Now()})
	if !result.Healthy {
		t.Fatalf("Omo native holder inventory should be healthy: %#v", result.Findings)
	}
}

func TestClassifyRejectsOneNativeSessionOwningTwoActiveCycles(t *testing.T) {
	snapshot := healthySnapshot()
	second := snapshot.Cycles[0]
	second.ID = "io-v2"
	second.Branch = "70-v1"
	second.WorktreePath = "/repo.worktrees/70-v1"
	snapshot.Cycles = append(snapshot.Cycles, second)
	snapshot.GitWorktrees = append(snapshot.GitWorktrees, operationalhealthcontract.GitWorktree{Path: second.WorktreePath, Branch: second.Branch, Head: snapshot.SourceHead, Clean: true})
	snapshot.LeaseHolderIndexes = append(snapshot.LeaseHolderIndexes, operationalhealthcontract.LeaseHolderIndex{Key: "second", LifecycleID: second.ID, Generation: second.Generation, Host: second.HolderHost, SessionID: second.HolderSessionID})
	result := Classify(snapshot, Options{Now: time.Now()})
	if !hasFinding(result, operationalhealthcontract.FindingInventoryUnknown, "lease_holder") {
		t.Fatalf("findings = %#v, want duplicate native-session finding", result.Findings)
	}
}

func healthySnapshot() Snapshot {
	const head = "0123456789abcdef0123456789abcdef01234567"
	cycle := operationalhealthcontract.Cycle{
		ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: "implement",
		LeaseStatus: "active", Generation: 1,
		WorktreePath: "/repo.worktrees/69-v1", HolderHost: "codex", HolderSessionID: "session",
		HolderPID: 123, HolderStartedAt: "2026-07-22T00:00:00Z", HolderExecutable: "/opt/codex",
		HolderProcessStatus: operationalhealthcontract.ProcessStatusLive,
	}
	return Snapshot{
		RepoRoot: "/repo", CanonicalBranch: "main", SourceHead: head, SourceClean: true,
		Cycles:             []operationalhealthcontract.Cycle{cycle},
		LeaseHolderIndexes: []operationalhealthcontract.LeaseHolderIndex{{Key: "current", LifecycleID: cycle.ID, Generation: cycle.Generation, Host: cycle.HolderHost, SessionID: cycle.HolderSessionID}},
		GitWorktrees: []operationalhealthcontract.GitWorktree{
			{Path: "/repo", Branch: "main", Head: head, Clean: true, Canonical: true},
			{Path: cycle.WorktreePath, Branch: cycle.Branch, Head: head, Clean: true},
		},
		LocalRefs:  []operationalhealthcontract.GitRef{{Name: "refs/heads/main", Branch: "main", OID: head, Location: "local"}},
		RemoteRefs: []operationalhealthcontract.GitRef{{Name: "refs/remotes/origin/main", Branch: "main", OID: head, Location: "remote"}},
	}
}

func hasFinding(result operationalhealthcontract.Result, code, kind string) bool {
	for _, finding := range result.Findings {
		if finding.Code == code && finding.ResourceKind == kind {
			return true
		}
	}
	return false
}

func TestKnownExecutionLeaseStatesRejectRemovedStates(t *testing.T) {
	for _, state := range []string{"", "claimable", "active", "revoking", "released"} {
		if !knownLeaseStatus(state) {
			t.Fatalf("v1 lease state %q rejected", state)
		}
	}
	for _, state := range []string{"owner_active", "ownership_dispatching", "closed"} {
		if knownLeaseStatus(state) {
			t.Fatalf("removed state %q remains accepted", state)
		}
	}
}

func clearCycleHolder(cycle *operationalhealthcontract.Cycle) {
	cycle.HolderHost = ""
	cycle.HolderSessionID = ""
	cycle.HolderAgentID = ""
	cycle.HolderPID = 0
	cycle.HolderStartedAt = ""
	cycle.HolderExecutable = ""
	cycle.HolderProcessStatus = ""
}

func withCycle(cycle operationalhealthcontract.Cycle, mutate func(*operationalhealthcontract.Cycle)) operationalhealthcontract.Cycle {
	mutate(&cycle)
	return cycle
}

// zero-clock 방어: Now를 비운 호출은 classification 신뢰 자체가 없다.
func TestClassifyRequiresExplicitClock(t *testing.T) {
	result := Classify(Snapshot{RepoRoot: "/repo"}, Options{})
	found := false
	for _, finding := range result.Findings {
		if finding.Code == operationalhealthcontract.FindingInventoryUnknown && finding.ResourceID == "now" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("zero clock must produce the clock finding: %+v", result.Findings)
	}
}

func TestClassifyPropagatesInventoryProblems(t *testing.T) {
	snapshot := Snapshot{
		RepoRoot: "/repo",
		InventoryProblems: []operationalhealthcontract.InventoryProblem{
			{Source: "ps", Code: "EIO", Detail: "probe failed"},
		},
	}
	result := Classify(snapshot, Options{Now: time.Now()})
	for _, finding := range result.Findings {
		if finding.Code == operationalhealthcontract.FindingInventoryUnknown && finding.ResourceID == "EIO" {
			return
		}
	}
	t.Fatalf("inventory problem must surface as an inventory finding: %+v", result.Findings)
}
