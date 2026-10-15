package issueops

import (
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func covExecActor() issueopscontract.NativeActor {
	return issueopscontract.NativeActor{
		Host: "codex", SessionID: "s1",
		SessionProcess: &issueopscontract.NativeProcessReceipt{PID: 10, StartedAt: "2026-01-01T00:00:00Z", Executable: "/bin/codex"},
	}
}

func covExecCompletion(generation uint64) issueopscontract.ExecutionCompletion {
	return issueopscontract.ExecutionCompletion{
		Generation: generation, FinalHead: strings.Repeat("a", 40), VerificationReportPath: "report.md",
		Verification: []string{"go test"}, RemoteArtifactURL: "https://example.test/pr/1", CompletedAt: "2026-01-01T00:00:00Z",
	}
}

func covExecDirect() issueopscontract.Execution {
	holder := covExecActor()
	return issueopscontract.Execution{
		Workspace: issueopscontract.Workspace{
			SourceRoot: "/repo", Root: "/repo.wt/x", Branch: "x", BaseHead: strings.Repeat("a", 40),
			LinkedAt: "2026-01-01T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 3, Status: issueopscontract.LeaseStatusActive, Holder: &holder, ClaimedAt: "2026-01-01T00:00:00Z",
		},
	}
}

func TestValidateExecutionAcceptsWellFormedRecords(t *testing.T) {
	if err := ValidateExecution(covExecDirect()); err != nil {
		t.Fatalf("direct: %v", err)
	}
}

func TestValidateExecutionRejectsMalformedRecords(t *testing.T) {
	released := func(e *issueopscontract.Execution) {
		e.Lease = issueopscontract.WriteLease{Generation: 3, Status: issueopscontract.LeaseStatusReleased}
	}
	resolution := func() *issueopscontract.ExecutionSyncBaseResolution {
		return &issueopscontract.ExecutionSyncBaseResolution{
			Generation: 3, CompletionGeneration: 3, BaseOID: strings.Repeat("a", 40), Actor: covExecActor(),
			ConflictFiles: []string{"a/b.go"}, StartedAt: "t",
		}
	}
	withCompletion := func(e *issueopscontract.Execution) {
		released(e)
		c := covExecCompletion(3)
		e.Completion = &c
	}
	cases := []struct {
		name   string
		mutate func(*issueopscontract.Execution)
		want   string
	}{
		{"missing workspace field", func(e *issueopscontract.Execution) { e.Workspace.Branch = " " }, "workspace branch is required"},
		{"workspace not isolated", func(e *issueopscontract.Execution) { e.Workspace.Root = e.Workspace.SourceRoot }, "isolated from source_root"},
		{"zero generation", func(e *issueopscontract.Execution) { e.Lease.Generation = 0 }, "lease generation must start at 1"},
		{"unknown lease status", func(e *issueopscontract.Execution) { e.Lease.Status = "weird" }, `unsupported lease status "weird"`},
		{"claimable with holder", func(e *issueopscontract.Execution) {
			e.Lease.Status = issueopscontract.LeaseStatusClaimable
			e.Lease.ClaimTokenSHA256 = strings.Repeat("a", 64)
		}, "claimable lease requires no holder"},
		{"claimable without token", func(e *issueopscontract.Execution) {
			e.Lease = issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusClaimable}
		}, "claimable lease requires no holder"},
		{"active without claimed_at", func(e *issueopscontract.Execution) { e.Lease.ClaimedAt = "" }, "active lease requires one holder"},
		{"active invalid holder", func(e *issueopscontract.Execution) { e.Lease.Holder.Host = "vim" }, "native actor host"},
		{"revoking without holder", func(e *issueopscontract.Execution) {
			e.Lease.Status = issueopscontract.LeaseStatusRevoking
			e.Lease.Holder = nil
		}, "revoking lease requires the fenced holder"},
		{"revoking invalid holder", func(e *issueopscontract.Execution) {
			e.Lease.Status = issueopscontract.LeaseStatusRevoking
			e.Lease.Holder.SessionID = ""
		}, "session_id is required"},
		{"released with token", func(e *issueopscontract.Execution) {
			released(e)
			e.Lease.ClaimTokenSHA256 = "x"
		}, "released lease must not retain"},
		{"pending incomplete", func(e *issueopscontract.Execution) {
			e.Pending = &issueopscontract.ExternalIntent{OperationID: "o", Kind: "k", Marker: "m"}
		}, "pending external intent is incomplete"},
		{"completion incomplete", func(e *issueopscontract.Execution) {
			c := covExecCompletion(1)
			c.FinalHead = "short"
			e.Completion = &c
		}, "execution completion is incomplete"},
		{"completion blank verification", func(e *issueopscontract.Execution) {
			c := covExecCompletion(1)
			c.Verification = []string{"ok", " "}
			e.Completion = &c
		}, "verification must be nonempty"},
		{"completion future generation", func(e *issueopscontract.Execution) { c := covExecCompletion(4); e.Completion = &c }, "completion generation exceeds the lease generation"},
		{"history incomplete", func(e *issueopscontract.Execution) {
			e.CompletionHistory = []issueopscontract.ExecutionCompletionHistory{{Generation: 1, Completion: covExecCompletion(1), ReopenedAt: "t"}}
		}, "completion history entry is incomplete"},
		{"history not preceding", func(e *issueopscontract.Execution) {
			e.CompletionHistory = []issueopscontract.ExecutionCompletionHistory{{Generation: 3, Completion: covExecCompletion(3), Reason: "r", ReopenedAt: "t"}}
		}, "must precede the lease generation"},
		{"history invalid completion", func(e *issueopscontract.Execution) {
			c := covExecCompletion(1)
			c.CompletedAt = ""
			e.CompletionHistory = []issueopscontract.ExecutionCompletionHistory{{Generation: 1, Completion: c, Reason: "r", ReopenedAt: "t"}}
		}, "execution completion history: execution completion is incomplete"},
		{"history generation conflict", func(e *issueopscontract.Execution) {
			e.CompletionHistory = []issueopscontract.ExecutionCompletionHistory{{Generation: 1, Completion: covExecCompletion(2), Reason: "r", ReopenedAt: "t"}}
		}, "generation conflicts with its completion"},
		{"failure without code", func(e *issueopscontract.Execution) { e.Failure = &issueopscontract.ExecutionFailure{At: "t"} }, "execution failure is invalid"},
		{"failure without at", func(e *issueopscontract.Execution) { e.Failure = &issueopscontract.ExecutionFailure{Code: "c"} }, "execution failure is invalid"},
		{"failure message too long", func(e *issueopscontract.Execution) {
			e.Failure = &issueopscontract.ExecutionFailure{Code: "c", At: "t", Message: strings.Repeat("m", 4097)}
		}, "execution failure is invalid"},
		{"resolution on active lease", func(e *issueopscontract.Execution) { e.SyncBaseResolution = resolution() }, "must bind the released current completion"},
		{"resolution generation mismatch", func(e *issueopscontract.Execution) {
			withCompletion(e)
			r := resolution()
			r.Generation = 2
			e.SyncBaseResolution = r
		}, "must bind the released current completion"},
		{"resolution completion mismatch", func(e *issueopscontract.Execution) {
			withCompletion(e)
			r := resolution()
			r.CompletionGeneration = 2
			e.SyncBaseResolution = r
		}, "must bind the released current completion"},
		{"resolution short oid", func(e *issueopscontract.Execution) {
			withCompletion(e)
			r := resolution()
			r.BaseOID = "abc"
			e.SyncBaseResolution = r
		}, "sync-base resolution is incomplete"},
		{"resolution no conflict files", func(e *issueopscontract.Execution) {
			withCompletion(e)
			r := resolution()
			r.ConflictFiles = nil
			e.SyncBaseResolution = r
		}, "sync-base resolution is incomplete"},
		{"resolution bad actor", func(e *issueopscontract.Execution) {
			withCompletion(e)
			r := resolution()
			r.Actor.Host = "vim"
			e.SyncBaseResolution = r
		}, "native actor host"},
		{"resolution escaping path", func(e *issueopscontract.Execution) {
			withCompletion(e)
			r := resolution()
			r.ConflictFiles = []string{"../x"}
			e.SyncBaseResolution = r
		}, "conflict path is invalid"},
		{"resolution absolute path", func(e *issueopscontract.Execution) {
			withCompletion(e)
			r := resolution()
			r.ConflictFiles = []string{"/etc/passwd"}
			e.SyncBaseResolution = r
		}, "conflict path is invalid"},
		{"resolution unclean path", func(e *issueopscontract.Execution) {
			withCompletion(e)
			r := resolution()
			r.ConflictFiles = []string{"a//b"}
			e.SyncBaseResolution = r
		}, "conflict path is invalid"},
		{"resolution dot path", func(e *issueopscontract.Execution) {
			withCompletion(e)
			r := resolution()
			r.ConflictFiles = []string{"."}
			e.SyncBaseResolution = r
		}, "conflict path is invalid"},
		{"resolution duplicate path", func(e *issueopscontract.Execution) {
			withCompletion(e)
			r := resolution()
			r.ConflictFiles = []string{"a.go", "a.go"}
			e.SyncBaseResolution = r
		}, "conflict path is invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := covExecDirect()
			tc.mutate(&e)
			err := ValidateExecution(e)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateExecutionAcceptsCompletionHistoryAndSyncBaseResolution(t *testing.T) {
	e := covExecDirect()
	e.Lease = issueopscontract.WriteLease{Generation: 3, Status: issueopscontract.LeaseStatusReleased}
	c := covExecCompletion(3)
	e.Completion = &c
	e.CompletionHistory = []issueopscontract.ExecutionCompletionHistory{{Generation: 2, Completion: covExecCompletion(2), Reason: "reopened", ReopenedAt: "t"}}
	e.SyncBaseResolution = &issueopscontract.ExecutionSyncBaseResolution{
		Generation: 3, CompletionGeneration: 3, BaseOID: strings.Repeat("f", 64), Actor: covExecActor(),
		ConflictFiles: []string{"a/b.go", "c.go"}, StartedAt: "t",
	}
	e.Failure = &issueopscontract.ExecutionFailure{Code: "c", At: "t", Message: strings.Repeat("m", 4096)}
	e.Pending = &issueopscontract.ExternalIntent{OperationID: "o", Kind: "k", Marker: "m", StartedAt: "t"}
	if err := ValidateExecution(e); err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}
}

func TestValidateExecutionSyncBaseEvents(t *testing.T) {
	valid := func() issueopscontract.ExecutionSyncBaseEvent {
		return issueopscontract.ExecutionSyncBaseEvent{
			Mode: issueopscontract.ExecutionSyncBaseEventApply, BaseBranch: "main", BaseOID: strings.Repeat("a", 40),
			MergeCommit: strings.Repeat("b", 64), Actor: "codex", At: "t",
		}
	}
	e := covExecDirect()
	e.SyncBaseEvents = []issueopscontract.ExecutionSyncBaseEvent{valid()}
	if err := ValidateExecution(e); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	cases := map[string]struct {
		mutate func(*issueopscontract.ExecutionSyncBaseEvent)
		want   string
	}{
		"mode":         {func(v *issueopscontract.ExecutionSyncBaseEvent) { v.Mode = "abort" }, "mode must be apply or finalize"},
		"base oid":     {func(v *issueopscontract.ExecutionSyncBaseEvent) { v.BaseOID = "abc" }, "full base and merge commit OIDs"},
		"merge oid":    {func(v *issueopscontract.ExecutionSyncBaseEvent) { v.MergeCommit = strings.Repeat("g", 40) }, "full base and merge commit OIDs"},
		"base branch":  {func(v *issueopscontract.ExecutionSyncBaseEvent) { v.BaseBranch = " " }, "event is incomplete"},
		"actor":        {func(v *issueopscontract.ExecutionSyncBaseEvent) { v.Actor = "" }, "event is incomplete"},
		"at":           {func(v *issueopscontract.ExecutionSyncBaseEvent) { v.At = "" }, "event is incomplete"},
		"negative cnt": {func(v *issueopscontract.ExecutionSyncBaseEvent) { v.ConflictFiles = -1 }, "must not be negative"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ev := valid()
			tc.mutate(&ev)
			e := covExecDirect()
			e.SyncBaseEvents = []issueopscontract.ExecutionSyncBaseEvent{valid(), ev}
			err := ValidateExecution(e)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestGateLedgerRelativePath(t *testing.T) {
	cases := []struct{ root, file, want string }{
		{"/repo", "/repo/.issueops/gates/a.md", ".issueops/gates/a.md"},
		{"/repo/", "/repo/x.md", "x.md"},
		{"/repo", "/repo", "/repo"},
	}
	for _, tc := range cases {
		if got := GateLedgerRelativePath(tc.root, tc.file); got != tc.want {
			t.Fatalf("GateLedgerRelativePath(%q,%q)=%q want %q", tc.root, tc.file, got, tc.want)
		}
	}
}

func TestScopeGateLedgersKeepsNonNumericAndEmptyIssueFolders(t *testing.T) {
	files := []string{
		"/r/.issueops/issues//gates.md",
		"/r/.issueops/issues/12a/gates.md",
		"/r/.issueops/issues/99/gates.md",
	}
	judged, skipped := ScopeGateLedgers("/r", files, "5")
	if len(judged) != 2 || judged[0] != files[0] || judged[1] != files[1] {
		t.Fatalf("empty and non-numeric folders must be judged: %v", judged)
	}
	if len(skipped) != 1 || skipped[0] != files[2] {
		t.Fatalf("other issue's numeric folder must be skipped: %v", skipped)
	}
}
