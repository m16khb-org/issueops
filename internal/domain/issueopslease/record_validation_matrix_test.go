package issueopslease

import (
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func persistedActorFixture() leasecontract.Actor {
	return leasecontract.Actor{
		Host: "omp", SessionID: "session",
		SessionProcess: &leasecontract.ProcessReceipt{PID: 42, StartedAt: "2026-08-03T00:00:00Z", Executable: "/bin/omp"},
	}
}

func completionFixture(generation uint64) leasecontract.Completion {
	return leasecontract.Completion{
		Generation: generation, FinalHead: strings.Repeat("a", 40),
		VerificationReportPath: "report.md", Verification: []string{"go test"},
		RemoteArtifactURL: "https://example.test/pr/1", CompletedAt: "2026-08-03T01:00:00Z",
	}
}

func directExecutionFixture() leasecontract.Record {
	return leasecontract.Record{Execution: &leasecontract.Execution{
		Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: strings.Repeat("a", 40), LinkedAt: "2026-08-03T00:00:00Z"},
		Lease:     leasecontract.Lease{Generation: 3, Status: "released"},
	}}
}

func TestValidatePersistedRecordAcceptsFixtures(t *testing.T) {
	for name, record := range map[string]leasecontract.Record{
		"no execution": {},
		"direct":       directExecutionFixture(),
	} {
		if err := ValidatePersistedRecord(record); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestValidatePersistedRecordRejectsInvalidExecution(t *testing.T) {
	holder := persistedActorFixture()
	tests := []struct {
		name    string
		base    func() leasecontract.Record
		mutate  func(*leasecontract.Execution)
		wantErr string
	}{
		{"blank workspace field", directExecutionFixture, func(e *leasecontract.Execution) { e.Workspace.Branch = "  " }, "workspace branch is required"},
		{"root equals source_root", directExecutionFixture, func(e *leasecontract.Execution) { e.Workspace.Root = e.Workspace.SourceRoot }, "canonical worktree must be isolated from source_root"},

		{"active lease without claimed_at", directExecutionFixture, func(e *leasecontract.Execution) {
			e.Lease = leasecontract.Lease{Generation: 3, Status: "active", Holder: &holder}
		}, "active lease requires one holder"},
		{"active lease with token hash", directExecutionFixture, func(e *leasecontract.Execution) {
			e.Lease = leasecontract.Lease{Generation: 3, Status: "active", Holder: &holder, ClaimedAt: "t", ClaimTokenSHA256: strings.Repeat("a", 64)}
		}, "active lease requires one holder"},
		{"active holder host", directExecutionFixture, func(e *leasecontract.Execution) {
			bad := holder
			bad.Host = "vim"
			e.Lease = leasecontract.Lease{Generation: 3, Status: "active", Holder: &bad, ClaimedAt: "t"}
		}, "native actor host must be"},
		{"revoking lease without holder", directExecutionFixture, func(e *leasecontract.Execution) {
			e.Lease = leasecontract.Lease{Generation: 3, Status: "revoking"}
		}, "revoking lease requires a holder"},
		{"claimable lease with holder", directExecutionFixture, func(e *leasecontract.Execution) {
			e.Lease = leasecontract.Lease{Generation: 3, Status: "claimable", Holder: &holder, ClaimTokenSHA256: strings.Repeat("a", 64)}
		}, "claimable lease requires no holder"},
		{"claimable lease with short token hash", directExecutionFixture, func(e *leasecontract.Execution) {
			e.Lease = leasecontract.Lease{Generation: 3, Status: "claimable", ClaimTokenSHA256: strings.Repeat("a", 63)}
		}, "claimable lease requires no holder"},
		{"released lease with holder", directExecutionFixture, func(e *leasecontract.Execution) { e.Lease.Holder = &holder }, "released lease must not retain"},
		{"unknown lease status", directExecutionFixture, func(e *leasecontract.Execution) { e.Lease.Status = "paused" }, `unsupported lease status "paused"`},

		{"incomplete pending intent", directExecutionFixture, func(e *leasecontract.Execution) {
			e.Pending = &leasecontract.ExternalIntent{OperationID: "op", Kind: "pr", StartedAt: "t"}
		}, "pending external intent is incomplete"},
		{"failure without code", directExecutionFixture, func(e *leasecontract.Execution) {
			e.Failure = &leasecontract.FailureDetail{At: "t"}
		}, "execution failure is invalid"},
		{"oversized failure message", directExecutionFixture, func(e *leasecontract.Execution) {
			e.Failure = &leasecontract.FailureDetail{Code: "c", At: "t", Message: strings.Repeat("m", 4097)}
		}, "execution failure is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := test.base()
			test.mutate(record.Execution)
			err := ValidatePersistedRecord(record)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestValidatePersistedRecordAcceptsBoundaryExecutionDetails(t *testing.T) {
	record := directExecutionFixture()
	holder := persistedActorFixture()
	record.Execution.Lease = leasecontract.Lease{Generation: 3, Status: "active", Holder: &holder, ClaimedAt: "t"}
	record.Execution.Failure = &leasecontract.FailureDetail{Code: "c", At: "t", Message: strings.Repeat("m", 4096)}
	record.Execution.Pending = &leasecontract.ExternalIntent{OperationID: "op", Kind: "pr", Marker: "m", StartedAt: "t"}
	if err := ValidatePersistedRecord(record); err != nil {
		t.Fatalf("boundary details: %v", err)
	}
	record.Execution.Lease = leasecontract.Lease{Generation: 3, Status: "claimable", ClaimTokenSHA256: strings.Repeat("a", 64)}
	if err := ValidatePersistedRecord(record); err != nil {
		t.Fatalf("claimable: %v", err)
	}
	record.Execution.Lease = leasecontract.Lease{Generation: 3, Status: "revoking", Holder: &holder}
	if err := ValidatePersistedRecord(record); err != nil {
		t.Fatalf("revoking: %v", err)
	}
}

func TestValidatePersistedActorRequiresReuseSafeReceipt(t *testing.T) {
	for _, host := range []string{"codex", "claude", "omo", "omp"} {
		actor := persistedActorFixture()
		actor.Host = host
		if err := ValidatePersistedActor(actor); err != nil {
			t.Errorf("host %q: %v", host, err)
		}
	}
	tests := []struct {
		name   string
		mutate func(*leasecontract.Actor)
		want   string
	}{
		{"unknown host", func(a *leasecontract.Actor) { a.Host = "Codex" }, "host must be"},
		{"zero pid", func(a *leasecontract.Actor) { a.SessionProcess.PID = 0 }, "session_process receipt"},
		{"missing start time", func(a *leasecontract.Actor) { a.SessionProcess.StartedAt = "" }, "session_process receipt"},
		{"missing executable", func(a *leasecontract.Actor) { a.SessionProcess.Executable = "" }, "session_process receipt"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actor := persistedActorFixture()
			test.mutate(&actor)
			if err := ValidatePersistedActor(actor); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestValidatePersistedSidecarsChecksCompletionRelativeToLease(t *testing.T) {
	record := directExecutionFixture()
	execution := record.Execution
	execution.Completion = &leasecontract.Completion{}
	if err := ValidatePersistedSidecars(*execution); err == nil || !strings.Contains(err.Error(), "execution completion is incomplete") {
		t.Fatalf("empty completion: %v", err)
	}
	completion := completionFixture(3)
	execution.Completion = &completion
	if err := ValidatePersistedSidecars(*execution); err != nil {
		t.Fatalf("completion at the lease generation: %v", err)
	}
	completion.Generation = 4
	if err := ValidatePersistedSidecars(*execution); err == nil || !strings.Contains(err.Error(), "exceeds the lease generation") {
		t.Fatalf("completion beyond the lease generation: %v", err)
	}
	completion = completionFixture(3)
	completion.Verification = []string{"go test", " "}
	if err := ValidatePersistedSidecars(*execution); err == nil || !strings.Contains(err.Error(), "verification must be nonempty") {
		t.Fatalf("blank verification evidence: %v", err)
	}
	completion = completionFixture(3)
	completion.FinalHead = strings.Repeat("a", 41)
	if err := ValidatePersistedSidecars(*execution); err == nil || !strings.Contains(err.Error(), "execution completion is incomplete") {
		t.Fatalf("final head of 41 hex characters: %v", err)
	}
	completion.FinalHead = strings.Repeat("b", 64)
	if err := ValidatePersistedSidecars(*execution); err != nil {
		t.Fatalf("64-character final head: %v", err)
	}
}

func TestValidatePersistedSidecarsChecksCompletionHistory(t *testing.T) {
	entry := func(generation uint64) leasecontract.CompletionHistoryEntry {
		return leasecontract.CompletionHistoryEntry{Generation: generation, Completion: completionFixture(generation), Reason: "reopened", ReopenedAt: "t"}
	}
	tests := []struct {
		name   string
		mutate func(*leasecontract.CompletionHistoryEntry)
		want   string
	}{
		{"zero generation", func(e *leasecontract.CompletionHistoryEntry) { e.Generation = 0 }, "history entry is incomplete"},
		{"blank reason", func(e *leasecontract.CompletionHistoryEntry) { e.Reason = " " }, "history entry is incomplete"},
		{"missing reopened_at", func(e *leasecontract.CompletionHistoryEntry) { e.ReopenedAt = "" }, "history entry is incomplete"},
		{"not preceding lease", func(e *leasecontract.CompletionHistoryEntry) {
			e.Generation = 3
			e.Completion.Generation = 3
		}, "must precede the lease generation"},
		{"invalid completion", func(e *leasecontract.CompletionHistoryEntry) { e.Completion.RemoteArtifactURL = "" }, "execution completion history: execution completion is incomplete"},
		{"generation conflicts with completion", func(e *leasecontract.CompletionHistoryEntry) { e.Completion.Generation = 1 }, "conflicts with its completion"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := directExecutionFixture()
			history := entry(2)
			test.mutate(&history)
			record.Execution.CompletionHistory = []leasecontract.CompletionHistoryEntry{history}
			err := ValidatePersistedRecord(record)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
	record := directExecutionFixture()
	record.Execution.CompletionHistory = []leasecontract.CompletionHistoryEntry{entry(1), entry(2)}
	if err := ValidatePersistedRecord(record); err != nil {
		t.Fatalf("valid history: %v", err)
	}
}

func TestValidatePersistedSidecarsChecksSyncBaseResolution(t *testing.T) {
	fixture := func() leasecontract.Execution {
		record := directExecutionFixture()
		execution := *record.Execution
		completion := completionFixture(3)
		execution.Completion = &completion
		execution.SyncBaseResolution = &leasecontract.SyncBaseResolution{
			Generation: 3, CompletionGeneration: 3, BaseOID: strings.Repeat("c", 40),
			Actor: persistedActorFixture(), ConflictFiles: []string{"a.go", "dir/b.go"}, StartedAt: "t",
		}
		return execution
	}
	if err := ValidatePersistedSidecars(fixture()); err != nil {
		t.Fatalf("valid resolution: %v", err)
	}
	const invalid = "execution sync-base resolution is invalid"
	tests := []struct {
		name   string
		mutate func(*leasecontract.Execution)
		want   string
	}{
		{"lease still held", func(e *leasecontract.Execution) {
			e.Lease.Status = "claimable"
			e.Lease.ClaimTokenSHA256 = strings.Repeat("a", 64)
		}, invalid},
		{"no completion", func(e *leasecontract.Execution) { e.Completion = nil }, invalid},
		{"resolution generation differs from lease", func(e *leasecontract.Execution) { e.SyncBaseResolution.Generation = 2 }, invalid},
		{"completion generation differs", func(e *leasecontract.Execution) { e.SyncBaseResolution.CompletionGeneration = 2 }, invalid},
		{"short base oid", func(e *leasecontract.Execution) { e.SyncBaseResolution.BaseOID = "abc" }, invalid},
		{"missing started_at", func(e *leasecontract.Execution) { e.SyncBaseResolution.StartedAt = " " }, invalid},
		{"no conflict files", func(e *leasecontract.Execution) { e.SyncBaseResolution.ConflictFiles = nil }, invalid},
		{"actor", func(e *leasecontract.Execution) { e.SyncBaseResolution.Actor.Host = "vim" }, "native actor host must be"},
		{"empty path", func(e *leasecontract.Execution) { e.SyncBaseResolution.ConflictFiles = []string{""} }, "conflict path is invalid"},
		{"absolute path", func(e *leasecontract.Execution) { e.SyncBaseResolution.ConflictFiles = []string{"/etc/passwd"} }, "conflict path is invalid"},
		{"unclean path", func(e *leasecontract.Execution) { e.SyncBaseResolution.ConflictFiles = []string{"a/../b.go"} }, "conflict path is invalid"},
		{"dot path", func(e *leasecontract.Execution) { e.SyncBaseResolution.ConflictFiles = []string{"."} }, "conflict path is invalid"},
		{"parent escape", func(e *leasecontract.Execution) { e.SyncBaseResolution.ConflictFiles = []string{"../x"} }, "conflict path is invalid"},
		{"bare parent", func(e *leasecontract.Execution) { e.SyncBaseResolution.ConflictFiles = []string{".."} }, "conflict path is invalid"},
		{"duplicate path", func(e *leasecontract.Execution) { e.SyncBaseResolution.ConflictFiles = []string{"a.go", "a.go"} }, "conflict path is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execution := fixture()
			test.mutate(&execution)
			err := ValidatePersistedSidecars(execution)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestValidatePersistedSidecarsChecksSyncBaseEvents(t *testing.T) {
	event := func() leasecontract.SyncBaseEvent {
		return leasecontract.SyncBaseEvent{Mode: "apply", BaseBranch: "main", BaseOID: strings.Repeat("a", 40), MergeCommit: strings.Repeat("b", 64), ConflictFiles: 0, Actor: "omp", At: "t"}
	}
	tests := []struct {
		name   string
		mutate func(*leasecontract.SyncBaseEvent)
		want   string
	}{
		{"mode", func(e *leasecontract.SyncBaseEvent) { e.Mode = "preview" }, "mode must be apply or finalize"},
		{"base oid", func(e *leasecontract.SyncBaseEvent) { e.BaseOID = strings.Repeat("a", 39) }, "full base and merge commit OIDs"},
		{"merge commit", func(e *leasecontract.SyncBaseEvent) { e.MergeCommit = strings.Repeat("z", 40) }, "full base and merge commit OIDs"},
		{"blank branch", func(e *leasecontract.SyncBaseEvent) { e.BaseBranch = " " }, "event is incomplete"},
		{"blank actor", func(e *leasecontract.SyncBaseEvent) { e.Actor = "" }, "event is incomplete"},
		{"blank timestamp", func(e *leasecontract.SyncBaseEvent) { e.At = "" }, "event is incomplete"},
		{"negative conflict count", func(e *leasecontract.SyncBaseEvent) { e.ConflictFiles = -1 }, "must not be negative"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execution := *directExecutionFixture().Execution
			bad := event()
			test.mutate(&bad)
			execution.SyncBaseEvents = []leasecontract.SyncBaseEvent{event(), bad}
			err := ValidatePersistedSidecars(execution)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
	execution := *directExecutionFixture().Execution
	finalize := event()
	finalize.Mode, finalize.ConflictFiles = "finalize", 3
	execution.SyncBaseEvents = []leasecontract.SyncBaseEvent{event(), finalize}
	if err := ValidatePersistedSidecars(execution); err != nil {
		t.Fatalf("valid events: %v", err)
	}
}
