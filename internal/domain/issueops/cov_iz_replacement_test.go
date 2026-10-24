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
	if got := ReplacementClaimCommand("io'1", 2); got != `issueops execution claim --id 'io'\''1' --generation 2 --claim-current-token` {
		t.Fatalf("claim=%s", got)
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
	receipt := model.NativeProcessReceipt{PID: 42, StartedAt: "2026-10-01T00:00:00Z", Executable: "/bin/codex"}
	none := model.NativeProcessReceipt{}
	if err := ValidateReplacementQuiescence(receipt, "dead", none); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReplacementQuiescence(receipt, "live", receipt); err == nil || !strings.Contains(err.Error(), "still live: pid=42 executable=/bin/codex") {
		t.Fatalf("live: %v", err)
	}
	for _, status := range []string{"unknown", "reused", ""} {
		if err := ValidateReplacementQuiescence(receipt, status, none); err == nil || !strings.Contains(err.Error(), "unsafe to finalize") {
			t.Fatalf("status %q: %v", status, err)
		}
	}
	// A reused PID (another start time) proves the recorded process is gone.
	reused := model.NativeProcessReceipt{PID: 42, StartedAt: "2026-10-24T09:00:00Z", Executable: "/usr/sbin/cron"}
	if err := ValidateReplacementQuiescence(receipt, "identity-mismatch", reused); err != nil {
		t.Fatalf("reused pid: %v", err)
	}
	// Same start, other executable (an exec in place); another start with the
	// same executable (a TZ or clock difference on a live holder); or an
	// unreadable start stays unsafe.
	for _, observed := range []model.NativeProcessReceipt{
		{PID: 42, StartedAt: receipt.StartedAt, Executable: "/bin/other"},
		{PID: 42, StartedAt: "2026-10-01T09:00:00Z", Executable: "/bin/codex"},
		{PID: 42, Executable: "/bin/other"},
		{PID: 43, StartedAt: "2026-10-24T09:00:00Z", Executable: "/bin/codex"},
	} {
		if err := ValidateReplacementQuiescence(receipt, "identity-mismatch", observed); err == nil || !strings.Contains(err.Error(), "unsafe to finalize") {
			t.Fatalf("observed %+v: %v", observed, err)
		}
	}
}
