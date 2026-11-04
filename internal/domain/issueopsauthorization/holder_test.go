package issueopsauthorization

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestMutationHolderRequiresCurrentLeaseAndVerifiedProcess(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 1, StartedAt: "start", Executable: "/bin/codex"}
	holder := model.NativeActor{Host: "codex", SessionID: "session", AgentID: "agent", SessionProcess: &process}
	record := model.Record{Execution: &model.Execution{Workspace: model.Workspace{SourceRoot: "/repo", Root: "/repo.worktrees/run", Branch: "run", BaseHead: strings.Repeat("a", 40), LinkedAt: "then"}, Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &holder, ClaimedAt: "then"}}}
	actor := model.Actor{Host: " CODEX ", SessionID: " session ", AgentID: " agent ", NativeProcessAncestry: []model.NativeProcessReceipt{process}}
	verified := model.VerifiedActor{Identity: holder, Method: model.VerifiedByNativeAncestry}
	if needs, err := ValidateHolder(model.Record{}, nil, nil); err != nil || needs {
		t.Fatalf("unprepared planning: needs=%v err=%v", needs, err)
	}
	if needs, err := ValidateHolder(record, &actor, &verified); err != nil || !needs {
		t.Fatalf("holder: needs=%v err=%v", needs, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*model.Actor, *model.VerifiedActor) *model.VerifiedActor
		detail string
	}{
		{"no actor flags", func(a *model.Actor, v *model.VerifiedActor) *model.VerifiedActor {
			*a = model.Actor{}
			return v
		}, "no actor flags were passed"},
		{"foreign host", func(a *model.Actor, v *model.VerifiedActor) *model.VerifiedActor {
			a.Host = "claude"
			return v
		}, `caller host "claude", holder host "codex"`},
		{"foreign session", func(a *model.Actor, v *model.VerifiedActor) *model.VerifiedActor {
			a.SessionID = "other"
			return v
		}, `caller session id "other", holder session id "session"`},
		{"foreign agent", func(a *model.Actor, v *model.VerifiedActor) *model.VerifiedActor {
			a.AgentID = ""
			return v
		}, `caller agent id "", holder agent id "agent"`},
		{"unverified caller", func(*model.Actor, *model.VerifiedActor) *model.VerifiedActor { return nil }, "verified session process"},
		{"verified other session", func(_ *model.Actor, v *model.VerifiedActor) *model.VerifiedActor {
			v.Identity.SessionID = "other"
			return v
		}, "verified session process"},
		{"reused pid", func(_ *model.Actor, v *model.VerifiedActor) *model.VerifiedActor {
			v.Identity.SessionProcess = &model.NativeProcessReceipt{PID: 1, StartedAt: "other", Executable: "/bin/codex"}
			return v
		}, "verified session process"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, v := actor, verified
			_, err := ValidateHolder(record, &a, tc.change(&a, &v))
			// The refusal names the mismatch and the recovery command.
			if err == nil || !strings.Contains(err.Error(), tc.detail) || !strings.Contains(err.Error(), "execution whoami --json") {
				t.Fatalf("err=%v, want %q and the whoami recovery step", err, tc.detail)
			}
		})
	}
	if _, err := ValidateHolder(record, nil, &verified); err == nil {
		t.Fatal("missing actor accepted")
	}
	record.Execution.Lease.Status = model.LeaseStatusReleased
	record.Execution.Lease.Holder = nil
	if _, err := ValidateHolder(record, &actor, &verified); err == nil || !strings.Contains(err.Error(), "no active write lease") {
		t.Fatalf("released error=%v", err)
	}
	if err := ValidateCWD(false, "/repo", "/repo.worktrees/1"); err == nil || !strings.Contains(err.Error(), `rerun with --cwd "/repo.worktrees/1" (got "/repo")`) {
		t.Fatalf("foreign cwd error=%v", err)
	}
	if err := ValidateCWD(true, "/repo.worktrees/1", "/repo.worktrees/1"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateHolderRefusalBranches(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 1, StartedAt: "start", Executable: "/bin/codex"}
	other := model.NativeProcessReceipt{PID: 2, StartedAt: "start", Executable: "/bin/codex"}
	holder := model.NativeActor{Host: "codex", SessionID: "session", AgentID: "agent", SessionProcess: &process}
	lease := func(status model.LeaseStatus, holder *model.NativeActor) model.Record {
		return model.Record{ID: "7", Execution: &model.Execution{Lease: model.WriteLease{Generation: 3, Status: status, Holder: holder}}}
	}
	verifiedWith := func(change func(*model.NativeActor)) *model.VerifiedActor {
		identity := holder
		change(&identity)
		return &model.VerifiedActor{Identity: identity, Method: model.VerifiedByNativeAncestry}
	}
	same := func(*model.NativeActor) {}
	actor := func(host, session, agent string) *model.Actor {
		return &model.Actor{Host: host, SessionID: session, AgentID: agent}
	}
	noProcessHolder := holder
	noProcessHolder.SessionProcess = nil

	for _, tc := range []struct {
		name     string
		record   model.Record
		actor    *model.Actor
		verified *model.VerifiedActor
		want     string
	}{
		{"active without holder", lease(model.LeaseStatusActive, nil), actor("codex", "session", "agent"), verifiedWith(same),
			"generation 3 has no active write lease (status active); run `issueops next --id 7 --json` and follow its next_command"},
		{"released", lease(model.LeaseStatusReleased, &holder), actor("codex", "session", "agent"), verifiedWith(same),
			"(status released); to write again, take a new generation: `issueops execution replace --id 7 --expected-generation 3 --preview --json`"},
		{"other status with holder", lease("", &holder), actor("codex", "session", "agent"), verifiedWith(same),
			"run `issueops next --id 7 --json`"},
		{"blank host and session", lease(model.LeaseStatusActive, &holder), actor(" ", "  ", "agent"), verifiedWith(same), "no actor flags were passed"},
		{"host only", lease(model.LeaseStatusActive, &holder), actor("codex", "", "agent"), verifiedWith(same), `caller session id "", holder session id "session"`},
		{"session only", lease(model.LeaseStatusActive, &holder), actor("", "session", "agent"), verifiedWith(same), `caller host "", holder host "codex"`},
		{"holder without process", lease(model.LeaseStatusActive, &noProcessHolder), actor("codex", "session", "agent"), verifiedWith(same), "verified session process"},
		{"verified without process", lease(model.LeaseStatusActive, &holder), actor("codex", "session", "agent"), verifiedWith(func(n *model.NativeActor) { n.SessionProcess = nil }), "verified session process"},
		{"verified other host", lease(model.LeaseStatusActive, &holder), actor("codex", "session", "agent"), verifiedWith(func(n *model.NativeActor) { n.Host = "claude" }), "verified session process"},
		{"verified other agent", lease(model.LeaseStatusActive, &holder), actor("codex", "session", "agent"), verifiedWith(func(n *model.NativeActor) { n.AgentID = "x" }), "verified session process"},
		{"verified other process", lease(model.LeaseStatusActive, &holder), actor("codex", "session", "agent"), verifiedWith(func(n *model.NativeActor) { n.SessionProcess = &other }), "verified session process"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			needs, err := ValidateHolder(tc.record, tc.actor, tc.verified)
			if needs || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("needs=%v err=%v, want refusal containing %q", needs, err, tc.want)
			}
		})
	}
	if needs, err := ValidateHolder(lease(model.LeaseStatusActive, &holder), actor("CODEX", "session", "agent"), verifiedWith(func(n *model.NativeActor) { n.Host = "CODEX" })); err != nil || !needs {
		t.Fatalf("case-insensitive host: needs=%v err=%v", needs, err)
	}
}
