package issueopsauthorization

import (
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsauthorizationcontract "issueops/internal/contract/issueopsauthorization"
)

func TestAuthorizeRequiresExactLeaseHolderProcessAndWorkspace(t *testing.T) {
	process := issueopscontract.NativeProcessReceipt{
		PID:        42,
		StartedAt:  "2026-08-11T05:00:00Z",
		Executable: "/opt/codex",
	}
	record := issueopscontract.IssueOpsRecord{
		Execution: &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/decision"},
			Lease: issueopscontract.WriteLease{
				Generation: 3,
				Status:     issueopscontract.LeaseStatusActive,
				Holder: &issueopscontract.NativeActor{
					Host:           "codex",
					SessionID:      "session",
					AgentID:        "agent",
					SessionProcess: &process,
				},
			},
		},
	}
	actor := &issueopscontract.IssueOpsActor{
		Host:                  "CODEX",
		SessionID:             "session",
		AgentID:               "agent",
		CWD:                   "/repo.worktrees/decision",
		NativeProcessAncestry: []issueopscontract.NativeProcessReceipt{process},
	}
	verified := &issueopscontract.VerifiedActor{Identity: *record.Execution.Lease.Holder, Method: issueopscontract.VerifiedByNativeAncestry}
	if needsPath, err := ValidateHolder(record, actor, verified); err != nil || !needsPath {
		t.Fatal(err)
	}
	if _, err := ValidateHolder(record, actor, nil); err == nil || !strings.Contains(err.Error(), "write lease holder") {
		t.Fatalf("unverified caller must fail closed: %v", err)
	}
	actor.NativeProcessAncestry = nil
	capability := &issueopscontract.VerifiedActor{Identity: *record.Execution.Lease.Holder, Method: issueopscontract.VerifiedByCapability}
	if needsPath, err := ValidateHolder(record, actor, capability); err != nil || !needsPath {
		t.Fatalf("capability-verified holder must not need observed ancestry: %v", err)
	}
}

// A write without an active lease names the way back: the replacement chain
// for a released generation, next for the other states.
func TestValidateHolderReleasedLeaseNamesTheWayBack(t *testing.T) {
	record := issueopsauthorizationcontract.Record{ID: "io-7", Execution: &issueopscontract.Execution{
		Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusReleased},
	}}
	_, err := ValidateHolder(record, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "status released") || !strings.Contains(err.Error(), "issueops execution replace --id io-7 --expected-generation 1 --preview --json") {
		t.Fatalf("released: err=%v", err)
	}
	record.Execution.Lease.Status = issueopscontract.LeaseStatusClaimable
	if _, err := ValidateHolder(record, nil, nil); err == nil || !strings.Contains(err.Error(), "issueops next --id io-7 --json") {
		t.Fatalf("claimable: err=%v", err)
	}
}
