package issueops

import (
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func validSyncBaseNativeActor() issueopscontract.NativeActor {
	return issueopscontract.NativeActor{Host: "codex", SessionID: "session-1", SessionProcess: &issueopscontract.NativeProcessReceipt{PID: 42, StartedAt: "2026-08-25T00:00:00Z", Executable: "/usr/local/bin/codex"}}
}

func fullOID() string { return strings.Repeat("a", 40) }

func TestValidateWriteLeaseStatusMatrix(t *testing.T) {
	holder := func() *issueopscontract.NativeActor {
		actor := validSyncBaseNativeActor()
		return &actor
	}
	tests := []struct {
		name    string
		lease   issueopscontract.WriteLease
		wantErr string
	}{
		{"zero generation", issueopscontract.WriteLease{Generation: 0, Status: issueopscontract.LeaseStatusClaimable}, "generation must start at 1"},
		{"unsupported status", issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatus("frozen")}, "unsupported lease status"},
		{"claimable with holder", issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusClaimable, Holder: holder()}, "claimable lease requires no holder"},
		{"claimable without token", issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusClaimable}, "claimable lease requires no holder and one token hash"},
		{"active without holder", issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusActive}, "active lease requires one holder"},
		{"active keeps token", issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusActive, Holder: holder(), ClaimTokenSHA256: strings.Repeat("b", 64), ClaimedAt: "t"}, "no token hash"},
		{"revoking without holder", issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusRevoking}, "revoking lease requires the fenced holder"},
		{"released retains token", issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusReleased, ClaimTokenSHA256: strings.Repeat("c", 64)}, "released lease must not retain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWriteLease(tt.lease)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}

	happy := []issueopscontract.WriteLease{
		{Generation: 1, Status: issueopscontract.LeaseStatusClaimable, ClaimTokenSHA256: strings.Repeat("b", 64)},
		{Generation: 1, Status: issueopscontract.LeaseStatusActive, Holder: holder(), ClaimedAt: "t"},
		{Generation: 1, Status: issueopscontract.LeaseStatusRevoking, Holder: holder()},
		{Generation: 1, Status: issueopscontract.LeaseStatusReleased},
	}
	for _, lease := range happy {
		if err := validateWriteLease(lease); err != nil {
			t.Fatalf("valid lease %+v rejected: %v", lease, err)
		}
	}
}

func TestValidateExecutionSyncBaseResolutionBindsReleasedCompletion(t *testing.T) {
	execution := issueopscontract.Execution{
		Lease:      issueopscontract.WriteLease{Generation: 3, Status: issueopscontract.LeaseStatusReleased},
		Completion: &issueopscontract.ExecutionCompletion{Generation: 3},
	}
	valid := issueopscontract.ExecutionSyncBaseResolution{
		Generation:           3,
		CompletionGeneration: 3,
		BaseOID:              fullOID(),
		StartedAt:            "2026-08-25T00:00:00Z",
		ConflictFiles:        []string{"internal/a.go", "internal/b.go"},
		Actor:                validSyncBaseNativeActor(),
	}
	if err := validateExecutionSyncBaseResolution(execution, valid); err != nil {
		t.Fatalf("valid resolution rejected: %v", err)
	}

	invalid := []struct {
		name    string
		mutate  func(*issueopscontract.ExecutionSyncBaseResolution)
		wantErr string
	}{
		{"wrong generation", func(r *issueopscontract.ExecutionSyncBaseResolution) { r.Generation = 4 }, "must bind the released current completion"},
		{"wrong completion generation", func(r *issueopscontract.ExecutionSyncBaseResolution) { r.CompletionGeneration = 9 }, "must bind the released current completion"},
		{"short base oid", func(r *issueopscontract.ExecutionSyncBaseResolution) { r.BaseOID = "abc" }, "is incomplete"},
		{"no conflict files", func(r *issueopscontract.ExecutionSyncBaseResolution) { r.ConflictFiles = nil }, "is incomplete"},
		{"duplicate conflict file", func(r *issueopscontract.ExecutionSyncBaseResolution) { r.ConflictFiles = []string{"a.go", "a.go"} }, "conflict path is invalid"},
		{"absolute conflict file", func(r *issueopscontract.ExecutionSyncBaseResolution) { r.ConflictFiles = []string{"/etc/passwd"} }, "conflict path is invalid"},
		{"escaping conflict file", func(r *issueopscontract.ExecutionSyncBaseResolution) { r.ConflictFiles = []string{"../secret"} }, "conflict path is invalid"},
		{"unclean conflict path", func(r *issueopscontract.ExecutionSyncBaseResolution) { r.ConflictFiles = []string{"./a.go"} }, "conflict path is invalid"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			resolution := valid
			tt.mutate(&resolution)
			err := validateExecutionSyncBaseResolution(execution, resolution)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}

	unbound := execution
	unbound.Lease.Status = issueopscontract.LeaseStatusActive
	if err := validateExecutionSyncBaseResolution(unbound, valid); err == nil ||
		!strings.Contains(err.Error(), "must bind the released current completion") {
		t.Fatal("resolution on non-released lease must be rejected")
	}
}

func TestValidateExecutionSyncBaseEventContract(t *testing.T) {
	valid := issueopscontract.ExecutionSyncBaseEvent{
		Mode:          issueopscontract.ExecutionSyncBaseEventApply,
		BaseOID:       fullOID(),
		MergeCommit:   strings.Repeat("b", 40),
		BaseBranch:    "main",
		Actor:         "codex",
		At:            "2026-08-25T00:00:00Z",
		ConflictFiles: 2,
	}
	if err := validateExecutionSyncBaseEvent(valid); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}

	invalid := []struct {
		name    string
		mutate  func(*issueopscontract.ExecutionSyncBaseEvent)
		wantErr string
	}{
		{"unknown mode", func(e *issueopscontract.ExecutionSyncBaseEvent) { e.Mode = "revert" }, "mode must be apply or finalize"},
		{"short merge commit", func(e *issueopscontract.ExecutionSyncBaseEvent) { e.MergeCommit = "zz" }, "full base and merge commit"},
		{"empty branch", func(e *issueopscontract.ExecutionSyncBaseEvent) { e.BaseBranch = " " }, "event is incomplete"},
		{"negative conflicts", func(e *issueopscontract.ExecutionSyncBaseEvent) { e.ConflictFiles = -1 }, "must not be negative"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			event := valid
			tt.mutate(&event)
			err := validateExecutionSyncBaseEvent(event)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
	finalize := valid
	finalize.Mode = issueopscontract.ExecutionSyncBaseEventFinalize
	if err := validateExecutionSyncBaseEvent(finalize); err != nil {
		t.Fatalf("finalize mode rejected: %v", err)
	}
}
