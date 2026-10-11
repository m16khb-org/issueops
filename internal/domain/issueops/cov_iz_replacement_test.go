package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func covIZActor(agent string) model.NativeActor {
	return model.NativeActor{
		Host: "codex", SessionID: "sess'1", AgentID: agent,
		SessionProcess: &model.NativeProcessReceipt{PID: 42, StartedAt: "2026-01-01T00:00:00Z", Executable: "/bin/codex"},
	}
}

func TestValidateReplacementGenerationBindsToCurrentLease(t *testing.T) {
	record := model.IssueOpsRecord{Execution: &model.Execution{Lease: model.WriteLease{Generation: 3}}}
	if err := ValidateReplacementGeneration(model.IssueOpsRecord{}, 3, false); err == nil || !strings.Contains(err.Error(), "not prepared") {
		t.Fatalf("no execution: %v", err)
	}
	if err := ValidateReplacementGeneration(record, 3, false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReplacementGeneration(record, 0, true); err != nil {
		t.Fatalf("discovery without a generation rejected: %v", err)
	}
	if err := ValidateReplacementGeneration(record, 0, false); err == nil || !strings.Contains(err.Error(), "current=3 expected=0") {
		t.Fatalf("mutation without a generation: %v", err)
	}
	for _, discovery := range []bool{false, true} {
		if err := ValidateReplacementGeneration(record, 2, discovery); err == nil || !strings.Contains(err.Error(), "stale lease generation: current=3 expected=2") {
			t.Fatalf("discovery=%t stale generation: %v", discovery, err)
		}
	}
}

func TestValidateReplacementPreviewAllowsOnlyOwnedOrFreeLeases(t *testing.T) {
	for _, status := range []model.LeaseStatus{model.LeaseStatusActive, model.LeaseStatusReleased, model.LeaseStatusClaimable} {
		if err := ValidateReplacementPreview(status); err != nil {
			t.Fatalf("%s: %v", status, err)
		}
	}
	if err := ValidateReplacementPreview(model.LeaseStatusRevoking); err == nil || !strings.Contains(err.Error(), "unavailable from revoking") {
		t.Fatalf("revoking: %v", err)
	}
	if err := ValidateReplacementPreview(""); err == nil {
		t.Fatal("empty status accepted")
	}
}

func TestReplacementRevokeRequiresActiveLeaseAndReasonThenAdvancesGeneration(t *testing.T) {
	holder := covIZActor("")
	active := model.WriteLease{Generation: 3, Status: model.LeaseStatusActive, Holder: &holder}
	if err := ValidateReplacementRevoke(active, "owner died"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReplacementRevoke(active, "  "); err == nil {
		t.Fatal("blank reason accepted")
	}
	for _, status := range []model.LeaseStatus{model.LeaseStatusClaimable, model.LeaseStatusReleased, model.LeaseStatusRevoking} {
		if err := ValidateReplacementRevoke(model.WriteLease{Status: status}, "reason"); err == nil {
			t.Fatalf("%s lease revoked", status)
		}
	}
	revoked := RevokeReplacement(active, "  owner died ", "now")
	if revoked.Generation != 4 || revoked.Status != model.LeaseStatusRevoking || revoked.ReplacedAt != "now" || revoked.ReplacementReason != "owner died" || revoked.Holder != &holder {
		t.Fatalf("revoked=%+v", revoked)
	}
	if active.Generation != 3 || active.Status != model.LeaseStatusActive || active.ReplacementReason != "" {
		t.Fatal("revoke mutated the original lease")
	}
}

func TestFinalizeReplacementReleasesWhenWorkspaceIsAbsent(t *testing.T) {
	holder := covIZActor("")
	revoking := model.WriteLease{Generation: 5, Status: model.LeaseStatusRevoking, Holder: &holder, ClaimTokenSHA256: "stale-token", ReplacementReason: "dead"}
	claimable := FinalizeReplacement(revoking, false, "new-token")
	if claimable.Status != model.LeaseStatusClaimable || claimable.Holder != nil || claimable.ClaimTokenSHA256 != "new-token" || claimable.Generation != 5 || claimable.ReplacementReason != "dead" {
		t.Fatalf("claimable=%+v", claimable)
	}
	released := FinalizeReplacement(revoking, true, "new-token")
	if released.Status != model.LeaseStatusReleased || released.Holder != nil || released.ClaimTokenSHA256 != "" {
		t.Fatalf("a workspace-less lease must not hold a claim token: %+v", released)
	}
	if revoking.Holder == nil || revoking.ClaimTokenSHA256 != "stale-token" {
		t.Fatal("finalize mutated the original lease")
	}
}

func TestSealReplacementOwnerStampsArtifactIdentity(t *testing.T) {
	SealReplacementOwner(nil, 9, model.ReplacementArtifacts{}) // must not panic
	binding := &model.OrcaBinding{RuntimeID: "rt", OwnerHost: "codex", IssueBodySHA256: "old"}
	SealReplacementOwner(binding, 9, model.ReplacementArtifacts{IssueBodySHA256: "i", ContextPacketSHA256: "c", OwnerPromptSHA256: "p", ContextPacketPath: "/ignored"})
	want := model.OrcaBinding{
		RuntimeID: "rt", OwnerHost: "codex", LeaseGeneration: 9, ArtifactIdentityVersion: model.OrcaArtifactIdentityVersion,
		IssueBodySHA256: "i", ContextPacketSHA256: "c", OwnerPromptSHA256: "p",
	}
	if *binding != want {
		t.Fatalf("binding=%+v want=%+v", *binding, want)
	}
}

func TestCompletedReplacementBaseRequiresStampedGenerationAndBaseBranch(t *testing.T) {
	completed := func(status model.LeaseStatus, generation uint64) model.IssueOpsRecord {
		return model.IssueOpsRecord{
			Execution:     &model.Execution{Lease: model.WriteLease{Status: status}, Completion: &model.ExecutionCompletion{Generation: generation}},
			BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "main"},
		}
	}
	for name, record := range map[string]model.IssueOpsRecord{
		"no execution":  {},
		"no completion": {Execution: &model.Execution{Lease: model.WriteLease{Status: model.LeaseStatusReleased}}},
		"active lease":  completed(model.LeaseStatusActive, 3),
		"revoking":      completed(model.LeaseStatusRevoking, 3),
	} {
		if is, err := CompletedReplacementBase(record, 0); is || err != nil {
			t.Fatalf("%s: %t %v", name, is, err)
		}
	}
	for _, status := range []model.LeaseStatus{model.LeaseStatusReleased, model.LeaseStatusClaimable} {
		if is, err := CompletedReplacementBase(completed(status, 3), 0); !is || err != nil {
			t.Fatalf("%s: %t %v", status, is, err)
		}
	}
	if is, err := CompletedReplacementBase(completed(model.LeaseStatusReleased, 3), 3); !is || err != nil {
		t.Fatalf("matching selection: %t %v", is, err)
	}
	if _, err := CompletedReplacementBase(completed(model.LeaseStatusReleased, 0), 0); err == nil || !strings.Contains(err.Error(), "invalid or missing stamped completion generation") {
		t.Fatalf("unstamped completion: %v", err)
	}
	if _, err := CompletedReplacementBase(completed(model.LeaseStatusReleased, 3), 2); err == nil || !strings.Contains(err.Error(), "conflicts with stamped completion generation 3") {
		t.Fatalf("conflicting selection: %v", err)
	}
	noBase := completed(model.LeaseStatusReleased, 3)
	noBase.BranchPrepare.BaseBranch = " "
	if _, err := CompletedReplacementBase(noBase, 0); err == nil || !strings.Contains(err.Error(), "branch_prepare.base_branch") {
		t.Fatalf("blank base: %v", err)
	}
	noBase.BranchPrepare = nil
	if _, err := CompletedReplacementBase(noBase, 0); err == nil {
		t.Fatal("missing branch prepare accepted")
	}
}

func TestReplacementCommandsQuoteOperatorInputAndBindIdentity(t *testing.T) {
	if got := ReplacementClaimCommand("io'1", 2, "/ignored"); got != `issueops execution claim --id 'io'\''1' --generation 2 --claim-current-token` {
		t.Fatalf("claim=%s", got)
	}
	if got := ReplacementResumeCommand("io-1", 7); got != "issueops execution resume --id 'io-1' --expected-generation 7 --confirm" {
		t.Fatalf("resume=%s", got)
	}
	if got := quoteReplacementArg("a'b c"); got != `'a'\''b c'` {
		t.Fatalf("quote=%s", got)
	}

	actor := covIZActor("agent 1")
	reseed := ReplacementReseedCommand("io-1", 3, 2, "fp", actor, "/work tree")
	want := `issueops execution replace --id 'io-1' --expected-generation 3 --completion-generation 2 --reseed --inventory-fingerprint fp --host 'codex' --session-id 'sess'\''1' --agent-id 'agent 1' --session-pid 42 --session-started-at '2026-01-01T00:00:00Z' --session-executable '/bin/codex' --cwd '/work tree' --confirm`
	if reseed != want {
		t.Fatalf("reseed=%s", reseed)
	}
	plain := ReplacementReseedCommand("io-1", 3, 0, "fp", covIZActor(""), "/w")
	if strings.Contains(plain, "--completion-generation") || strings.Contains(plain, "--agent-id") {
		t.Fatalf("optional flags rendered: %s", plain)
	}

	revoke := ReplacementRevokeCommand("io-1", 3, "fp", actor, "/w")
	wantRevoke := `issueops execution replace --id 'io-1' --expected-generation 3 --revoke --reason <TEXT> --inventory-fingerprint fp --host 'codex' --session-id 'sess'\''1' --agent-id 'agent 1' --session-pid 42 --session-started-at '2026-01-01T00:00:00Z' --session-executable '/bin/codex' --cwd '/w' --confirm`
	if revoke != wantRevoke {
		t.Fatalf("revoke=%s", revoke)
	}
	actor.AgentID, actor.SessionProcess = "", nil
	noProcess := ReplacementRevokeCommand("io-1", 3, "fp", actor, "/w")
	if strings.Contains(noProcess, "--session-pid") || strings.Contains(noProcess, "--agent-id") || !strings.HasSuffix(noProcess, "--cwd '/w' --confirm") {
		t.Fatalf("revoke without process: %s", noProcess)
	}
}

func covIZOrcaExecution(status model.LeaseStatus, holder *model.NativeActor, sealedRuntime string) model.IssueOpsRecord {
	return model.IssueOpsRecord{Execution: &model.Execution{
		Mode: model.ExecutionModeOrca, Orca: &model.OrcaBinding{RuntimeID: sealedRuntime},
		Lease: model.WriteLease{Status: status, Holder: holder},
	}}
}

func TestAllowReplacementRuntimeRolloverRequiresDeadHolderOrNoHolder(t *testing.T) {
	holder := covIZActor("")
	for _, tc := range []struct {
		name    string
		status  model.LeaseStatus
		holder  *model.NativeActor
		process string
		want    bool
	}{
		{"released without holder", model.LeaseStatusReleased, nil, "live", true},
		{"claimable without holder", model.LeaseStatusClaimable, nil, "", true},
		{"active without holder", model.LeaseStatusActive, nil, "dead", false},
		{"active dead holder", model.LeaseStatusActive, &holder, "dead", true},
		{"revoking dead holder", model.LeaseStatusRevoking, &holder, "dead", true},
		{"active live holder", model.LeaseStatusActive, &holder, "live", false},
		{"active unknown holder", model.LeaseStatusActive, &holder, "unknown", false},
		{"released dead holder", model.LeaseStatusReleased, &holder, "dead", false},
	} {
		if got := AllowReplacementRuntimeRollover(covIZOrcaExecution(tc.status, tc.holder, "rt"), tc.process); got != tc.want {
			t.Fatalf("%s: got %t", tc.name, got)
		}
	}
}

func TestDeadReplacementOwnerRuntimeRolloverNeedsEveryDeadOwnerFact(t *testing.T) {
	holder := covIZActor("")
	record := covIZOrcaExecution(model.LeaseStatusActive, &holder, " rt-old ")
	moved := ReplacementRuntimeFacts{RuntimeID: "rt-new"}
	if !DeadReplacementOwnerRuntimeRollover(record, "dead", moved) {
		t.Fatal("dead owner on a rolled-over runtime rejected")
	}
	revoking := covIZOrcaExecution(model.LeaseStatusRevoking, &holder, "rt-old")
	if !DeadReplacementOwnerRuntimeRollover(revoking, "dead", moved) {
		t.Fatal("revoking lease rejected")
	}
	for name, tc := range map[string]struct {
		record  model.IssueOpsRecord
		process string
		facts   ReplacementRuntimeFacts
	}{
		"no execution": {model.IssueOpsRecord{}, "dead", moved},
		"direct mode": {func() model.IssueOpsRecord {
			r := record
			e := *r.Execution
			e.Mode = model.ExecutionModeDirect
			r.Execution = &e
			return r
		}(), "dead", moved},
		"no orca binding":   {func() model.IssueOpsRecord { r := record; e := *r.Execution; e.Orca = nil; r.Execution = &e; return r }(), "dead", moved},
		"live holder":       {record, "live", moved},
		"same runtime":      {record, "dead", ReplacementRuntimeFacts{RuntimeID: "rt-old"}},
		"unobserved":        {record, "dead", ReplacementRuntimeFacts{RuntimeID: " "}},
		"terminal id kept":  {record, "dead", ReplacementRuntimeFacts{RuntimeID: "rt-new", TerminalID: "t1"}},
		"terminal live":     {record, "dead", ReplacementRuntimeFacts{RuntimeID: "rt-new", TerminalLive: true}},
		"released lease":    {covIZOrcaExecution(model.LeaseStatusReleased, &holder, "rt-old"), "dead", moved},
		"holder no process": {covIZOrcaExecution(model.LeaseStatusActive, &model.NativeActor{Host: "codex"}, "rt-old"), "dead", moved},
		"no holder":         {covIZOrcaExecution(model.LeaseStatusActive, nil, "rt-old"), "dead", moved},
	} {
		if DeadReplacementOwnerRuntimeRollover(tc.record, tc.process, tc.facts) {
			t.Fatalf("%s: rollover allowed", name)
		}
	}
}

func TestValidateReplacementRuntimeRolloverRequiresQuiescentOwnerAcrossRuntimes(t *testing.T) {
	holder := covIZActor("")
	settled := ReplacementRuntimeFacts{RuntimeID: "rt-new", TaskStatus: "completed", DispatchStatus: "completed"}
	free := covIZOrcaExecution(model.LeaseStatusReleased, nil, "rt-old")

	if err := ValidateReplacementRuntimeRollover(model.IssueOpsRecord{}, "dead", settled); err != nil {
		t.Fatalf("no execution: %v", err)
	}
	if err := ValidateReplacementRuntimeRollover(free, "", ReplacementRuntimeFacts{RuntimeID: "rt-old", TaskLive: true}); err != nil {
		t.Fatalf("same runtime is not a rollover: %v", err)
	}
	if err := ValidateReplacementRuntimeRollover(free, "", ReplacementRuntimeFacts{TaskLive: true}); err != nil {
		t.Fatalf("unobserved runtime is not a rollover: %v", err)
	}
	if err := ValidateReplacementRuntimeRollover(free, "", settled); err != nil {
		t.Fatalf("quiescent holderless lease rejected: %v", err)
	}
	for _, task := range []string{"completed", "failed"} {
		for _, dispatch := range []string{"completed", "failed", "circuit_broken"} {
			facts := ReplacementRuntimeFacts{RuntimeID: "rt-new", TaskStatus: task, DispatchStatus: dispatch}
			if err := ValidateReplacementRuntimeRollover(free, "", facts); err != nil {
				t.Fatalf("task=%s dispatch=%s: %v", task, dispatch, err)
			}
		}
	}
	for name, facts := range map[string]ReplacementRuntimeFacts{
		"task live":         {RuntimeID: "rt-new", TaskLive: true, TaskStatus: "completed", DispatchStatus: "completed"},
		"terminal live":     {RuntimeID: "rt-new", TerminalLive: true, TaskStatus: "completed", DispatchStatus: "completed"},
		"terminal retained": {RuntimeID: "rt-new", TerminalID: "pty", TaskStatus: "completed", DispatchStatus: "completed"},
		"task running":      {RuntimeID: "rt-new", TaskStatus: "running", DispatchStatus: "completed"},
		"dispatch pending":  {RuntimeID: "rt-new", TaskStatus: "completed", DispatchStatus: "pending"},
		"empty statuses":    {RuntimeID: "rt-new"},
	} {
		err := ValidateReplacementRuntimeRollover(free, "", facts)
		if err == nil || !strings.Contains(err.Error(), "not quiescent") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	held := covIZOrcaExecution(model.LeaseStatusActive, &holder, "rt-old")
	if err := ValidateReplacementRuntimeRollover(held, "live", settled); err == nil || !strings.Contains(err.Error(), "task_status=completed dispatch_status=completed") {
		t.Fatalf("live holder across runtimes: %v", err)
	}
	if err := ValidateReplacementRuntimeRollover(held, "dead", settled); err != nil {
		t.Fatalf("dead holder across runtimes: %v", err)
	}
}

func TestReplacementSelfHolderMatchesHostSessionAndAgentCaseInsensitivelyOnHost(t *testing.T) {
	holder := covIZActor("a1")
	lease := model.WriteLease{Holder: &holder}
	requester := covIZActor("a1")
	requester.Host = "CODEX"
	if !ReplacementSelfHolder(lease, requester) {
		t.Fatal("same holder not recognized")
	}
	for name, mutate := range map[string]func(*model.NativeActor){
		"host":    func(a *model.NativeActor) { a.Host = "claude" },
		"session": func(a *model.NativeActor) { a.SessionID = "SESS'1" },
		"agent":   func(a *model.NativeActor) { a.AgentID = "a2" },
	} {
		other := covIZActor("a1")
		mutate(&other)
		if ReplacementSelfHolder(lease, other) {
			t.Fatalf("different %s treated as self", name)
		}
	}
	if ReplacementSelfHolder(model.WriteLease{}, requester) {
		t.Fatal("holderless lease matched")
	}
	noProcess := holder
	noProcess.SessionProcess = nil
	if ReplacementSelfHolder(model.WriteLease{Holder: &noProcess}, requester) {
		t.Fatal("holder without a process receipt matched")
	}
}

func TestReplacementPreconditionsRefuseUnsafeFinalization(t *testing.T) {
	if err := ValidateReplacementSelfRevoke("io-1", 4, "dead"); err != nil {
		t.Fatalf("non-live holder: %v", err)
	}
	if err := ValidateReplacementSelfRevoke(" io-1 ", 4, "live"); err == nil || !strings.Contains(err.Error(), "execution release --id io-1 --generation 4") {
		t.Fatalf("self revoke must point at release: %v", err)
	}
	if err := ValidateReplacementCWD(true, false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReplacementCWD(false, true); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReplacementCWD(false, false); err == nil || !strings.Contains(err.Error(), "source_root or the canonical worktree") {
		t.Fatalf("foreign cwd: %v", err)
	}
	holder := covIZActor("")
	if err := ValidateReplacementHolderProcess(&holder); err != nil {
		t.Fatal(err)
	}
	for name, h := range map[string]*model.NativeActor{"nil": nil, "no process": {Host: "codex"}} {
		if err := ValidateReplacementHolderProcess(h); err == nil || !strings.Contains(err.Error(), "old process receipt") {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestValidateReplacementQuiescenceFinalizesOnlyDeadHolders(t *testing.T) {
	receipt := model.NativeProcessReceipt{PID: 42, Executable: "/bin/codex"}
	if err := ValidateReplacementQuiescence(receipt, "dead"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReplacementQuiescence(receipt, "live"); err == nil || !strings.Contains(err.Error(), "still live: pid=42 executable=/bin/codex") {
		t.Fatalf("live: %v", err)
	}
	for _, status := range []string{"unknown", "reused", ""} {
		if err := ValidateReplacementQuiescence(receipt, status); err == nil || !strings.Contains(err.Error(), "unsafe to finalize") {
			t.Fatalf("status %q: %v", status, err)
		}
	}
}

func TestValidateReplacementOwnerQuiescenceExemptsDeadRolledOverOwners(t *testing.T) {
	holder := covIZActor("")
	record := covIZOrcaExecution(model.LeaseStatusRevoking, &holder, "rt-old")
	live := ReplacementRuntimeFacts{RuntimeID: "rt-old", TerminalLive: true, TaskLive: true, TaskStatus: "running", DispatchStatus: "pending"}
	err := ValidateReplacementOwnerQuiescence(record, "dead", live)
	if err == nil || !strings.Contains(err.Error(), "terminal_live=true task_live=true task_status=running dispatch_status=pending") {
		t.Fatalf("live terminal/task on the sealed runtime: %v", err)
	}
	if err := ValidateReplacementOwnerQuiescence(record, "dead", ReplacementRuntimeFacts{RuntimeID: "rt-old", TaskLive: true}); err == nil {
		t.Fatal("live task alone accepted")
	}
	if err := ValidateReplacementOwnerQuiescence(record, "dead", ReplacementRuntimeFacts{RuntimeID: "rt-old"}); err != nil {
		t.Fatalf("no live terminal or task: %v", err)
	}
	// a dead owner on another runtime cannot be observed there, so the sealed runtime's liveness flags no longer apply
	if err := ValidateReplacementOwnerQuiescence(record, "dead", ReplacementRuntimeFacts{RuntimeID: "rt-new", TaskLive: true}); err != nil {
		t.Fatalf("dead rolled-over owner: %v", err)
	}
	if err := ValidateReplacementOwnerQuiescence(record, "live", ReplacementRuntimeFacts{RuntimeID: "rt-new", TaskLive: true}); err == nil {
		t.Fatal("live holder with a live task accepted")
	}
}
