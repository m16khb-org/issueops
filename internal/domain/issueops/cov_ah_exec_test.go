package issueops

import (
	"strings"
	"testing"

	"issueops/internal/contract/executionissue"
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
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: "/repo", Root: "/repo.wt/x", Branch: "x", BaseHead: strings.Repeat("a", 40),
			Driver: "git", LinkedAt: "2026-01-01T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 3, Status: issueopscontract.LeaseStatusActive, Holder: &holder, ClaimedAt: "2026-01-01T00:00:00Z",
		},
		Selection: &issueopscontract.ExecutionSelection{
			RequestedMode: "direct", ResolvedMode: "direct", ExplicitDirectReason: "no orca",
			ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "2026-01-01T00:00:00Z",
		},
	}
}

func covExecOrca() issueopscontract.Execution {
	e := covExecDirect()
	e.Mode = issueopscontract.ExecutionModeOrca
	e.Workspace.Driver = "orca"
	e.Selection = &issueopscontract.ExecutionSelection{
		RequestedMode: "auto", ResolvedMode: "orca", ProbeAttempted: true, ProbeAvailable: true, ProbeReady: true,
		ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "2026-01-01T00:00:00Z",
	}
	e.Orca = &issueopscontract.OrcaBinding{
		RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt", RunID: "run-1", LeaseGeneration: 2,
		ArtifactIdentityVersion: issueopscontract.OrcaArtifactIdentityVersion,
		IssueBodySHA256:         strings.Repeat("c", 64), ContextPacketSHA256: strings.Repeat("d", 64), OwnerPromptSHA256: strings.Repeat("e", 64),
		OwnerHost: "omp", OwnerModel: "m", TaskID: "t", DispatchID: "d",
	}
	return e
}

func TestValidateExecutionAcceptsWellFormedRecords(t *testing.T) {
	if err := ValidateExecution(covExecDirect()); err != nil {
		t.Fatalf("direct: %v", err)
	}
	if err := ValidateExecution(covExecOrca()); err != nil {
		t.Fatalf("orca: %v", err)
	}
	// auto 선택이 direct로 폴백하는 경우: probe 실패 코드가 fallback_code와 정확히 같아야 한다.
	e := covExecDirect()
	e.Selection = &issueopscontract.ExecutionSelection{
		RequestedMode: "auto", ResolvedMode: "direct", ProbeAttempted: true, ProbeCode: "orca_down", FallbackCode: "orca_down",
		ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "t",
	}
	if err := ValidateExecution(e); err != nil {
		t.Fatalf("auto fallback: %v", err)
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
		{"bad mode", func(e *issueopscontract.Execution) { e.Mode = "x" }, "execution mode must be direct or orca"},
		{"missing workspace field", func(e *issueopscontract.Execution) { e.Workspace.Branch = " " }, "workspace branch is required"},
		{"workspace not isolated", func(e *issueopscontract.Execution) { e.Workspace.Root = e.Workspace.SourceRoot }, "isolated from source_root"},
		{"direct driver", func(e *issueopscontract.Execution) { e.Workspace.Driver = "orca" }, "direct execution workspace driver must be git"},
		{"orca driver", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Workspace.Driver = "git" }, "Orca execution workspace driver must be orca"},
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
		{"missing selection", func(e *issueopscontract.Execution) { e.Selection = nil }, "selection receipt is required"},
		{"bad requested mode", func(e *issueopscontract.Execution) { e.Selection.RequestedMode = "fast" }, "requested_mode must be auto, direct, or orca"},
		{"resolved mismatch", func(e *issueopscontract.Execution) { e.Selection.ResolvedMode = "orca" }, "resolved_mode must equal execution mode"},
		{"available without attempted", func(e *issueopscontract.Execution) { e.Selection.ProbeAvailable = true }, "probe_available requires probe_attempted"},
		{"ready without available", func(e *issueopscontract.Execution) { e.Selection.ProbeAttempted = true; e.Selection.ProbeReady = true }, "probe_ready requires probe_available"},
		{"code without attempt", func(e *issueopscontract.Execution) { e.Selection.ProbeCode = "c" }, "unattempted selection probe must not contain probe_code"},
		{"auto without probe", func(e *issueopscontract.Execution) { e.Selection.RequestedMode = "auto" }, "require a readiness probe"},
		{"explicit mode mismatch", func(e *issueopscontract.Execution) {
			*e = covExecOrca()
			e.Selection.RequestedMode = "direct"
			e.Selection.ProbeAttempted, e.Selection.ProbeAvailable, e.Selection.ProbeReady = false, false, false
		}, "explicit selection mode must equal execution mode"},
		{"explicit orca on direct", func(e *issueopscontract.Execution) {
			e.Selection.RequestedMode = "orca"
			e.Selection.ProbeAttempted = true
		}, "explicit selection mode must equal execution mode"},
		{"auto direct without fallback", func(e *issueopscontract.Execution) {
			e.Selection = &issueopscontract.ExecutionSelection{
				RequestedMode: "auto", ResolvedMode: "direct", ProbeAttempted: true, ProbeCode: "c",
				ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "t",
			}
		}, "auto direct selection requires the exact probe failure fallback_code"},
		{"auto direct fallback differs", func(e *issueopscontract.Execution) {
			e.Selection = &issueopscontract.ExecutionSelection{
				RequestedMode: "auto", ResolvedMode: "direct", ProbeAttempted: true, ProbeCode: "c", FallbackCode: "d",
				ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "t",
			}
		}, "auto direct selection requires the exact probe failure fallback_code"},
		{"orca with fallback", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Selection.FallbackCode = "x" }, "Orca selection must not contain fallback_code"},
		{"non-direct with direct reason", func(e *issueopscontract.Execution) {
			*e = covExecOrca()
			e.Selection.ExplicitDirectReason = "why"
		}, "non-direct selection must not contain explicit_direct_reason"},
		{"direct without reason", func(e *issueopscontract.Execution) { e.Selection.ExplicitDirectReason = "" }, "explicit direct selection requires a reason and no probe"},
		{"direct with fallback", func(e *issueopscontract.Execution) { e.Selection.FallbackCode = "x" }, "explicit direct selection must not contain fallback_code"},
		{"bad fingerprint", func(e *issueopscontract.Execution) { e.Selection.ReadinessFingerprint = "abc" }, "selection receipt requires fingerprint and selected_at"},
		{"missing selected_at", func(e *issueopscontract.Execution) { e.Selection.SelectedAt = " " }, "selection receipt requires fingerprint and selected_at"},
		{"direct with orca binding", func(e *issueopscontract.Execution) { o := covExecOrca().Orca; e.Orca = o }, "direct execution must not contain an Orca binding"},
		{"orca missing field", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Orca.TaskID = "" }, "Orca binding task_id is required"},
		{"orca bad host", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Orca.OwnerHost = "vim" }, "Orca owner_host must be"},
		{"orca empty run id", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Orca.RunID = "" }, "run_id must be one canonical"},
		{"orca padded run id", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Orca.RunID = " run" }, "run_id must be one canonical"},
		{"orca long run id", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Orca.RunID = strings.Repeat("r", 1025) }, "run_id must be one canonical"},
		{"orca artifact version", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Orca.ArtifactIdentityVersion = 9 }, "unsupported Orca artifact identity version 9"},
		{"orca missing digest", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Orca.ContextPacketSHA256 = "" }, "requires a complete sealed artifact identity"},
		{"orca bad digest", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Orca.OwnerPromptSHA256 = "zz" }, "must contain SHA-256 digests"},
		{"orca zero lease generation", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Orca.LeaseGeneration = 0 }, "lease_generation is required"},
		{"orca future lease generation", func(e *issueopscontract.Execution) { *e = covExecOrca(); e.Orca.LeaseGeneration = 4 }, "lease_generation exceeds the lease generation"},
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

func covExecSnapshotRecord() issueopscontract.IssueOpsRecord {
	const url = "https://gitlab.com/acme/repo/-/issues/42"
	return issueopscontract.IssueOpsRecord{
		IssueURL:      url,
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{Provider: "gitlab", IssueURL: url},
	}
}

func covExecSnapshotRequest() issueopscontract.ExecutionActionRequest {
	return issueopscontract.ExecutionActionRequest{
		Action: issueopscontract.ExecutionActionPrepare,
		IssueSnapshot: &executionissue.ExecutionIssueSnapshotEvidence{
			Provider: "gitlab", Source: "glab_mcp", WebURL: "https://gitlab.com/acme/repo/-/issues/42", Body: "body", State: "opened",
		},
	}
}

func TestValidateExecutionSnapshotEvidence(t *testing.T) {
	if err := ValidateExecutionSnapshotEvidence(covExecSnapshotRequest(), covExecSnapshotRecord()); err != nil {
		t.Fatalf("valid evidence rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*issueopscontract.ExecutionActionRequest, *issueopscontract.IssueOpsRecord)
		want   string
	}{
		{"record gate", func(q *issueopscontract.ExecutionActionRequest, _ *issueopscontract.IssueOpsRecord) {
			q.Action = issueopscontract.ExecutionActionReconcile
			q.Confirm = true
		}, "only when confirm resumes a pending worktree_create"},
		{"no branch prepare", func(_ *issueopscontract.ExecutionActionRequest, r *issueopscontract.IssueOpsRecord) {
			r.BranchPrepare = nil
		}, "provider does not match the linked IssueOps provider"},
		{"record provider", func(_ *issueopscontract.ExecutionActionRequest, r *issueopscontract.IssueOpsRecord) {
			r.BranchPrepare.Provider = "github"
		}, "provider does not match the linked IssueOps provider"},
		{"inconsistent identity", func(_ *issueopscontract.ExecutionActionRequest, r *issueopscontract.IssueOpsRecord) {
			r.BranchPrepare.IssueURL = "https://gitlab.com/acme/repo/-/issues/43"
		}, "linked IssueOps GitLab identity is inconsistent"},
		{"evidence provider", func(q *issueopscontract.ExecutionActionRequest, _ *issueopscontract.IssueOpsRecord) {
			q.IssueSnapshot.Provider = "github"
		}, "issue_snapshot provider must be gitlab"},
		{"evidence source", func(q *issueopscontract.ExecutionActionRequest, _ *issueopscontract.IssueOpsRecord) {
			q.IssueSnapshot.Source = "curl"
		}, "issue_snapshot source must be glab_mcp"},
		{"evidence url", func(q *issueopscontract.ExecutionActionRequest, _ *issueopscontract.IssueOpsRecord) {
			q.IssueSnapshot.WebURL = "https://gitlab.com/acme/repo/-/issues/7"
		}, "invalid issue_snapshot: GitLab issue snapshot URL does not match"},
		{"evidence state", func(q *issueopscontract.ExecutionActionRequest, _ *issueopscontract.IssueOpsRecord) {
			q.IssueSnapshot.State = "merged"
		}, "invalid issue_snapshot: GitLab issue snapshot state"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, rec := covExecSnapshotRequest(), covExecSnapshotRecord()
			tc.mutate(&req, &rec)
			err := ValidateExecutionSnapshotEvidence(req, rec)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateExecutionSnapshotActionRejectsReplaceWithoutSeedingAction(t *testing.T) {
	for _, ra := range []string{"", "other"} {
		req := issueopscontract.ExecutionActionRequest{Action: issueopscontract.ExecutionActionReplace, ReplaceAction: ra}
		if err := ValidateExecutionSnapshotAction(req); err == nil {
			t.Fatalf("replace_action %q must be rejected", ra)
		}
	}
	req := issueopscontract.ExecutionActionRequest{Action: issueopscontract.ExecutionActionReplace, ReplaceAction: issueopscontract.ExecutionReplaceReseed}
	if err := ValidateExecutionSnapshotAction(req); err != nil {
		t.Fatalf("reseed must pass: %v", err)
	}
	req = issueopscontract.ExecutionActionRequest{Action: issueopscontract.ExecutionActionReconcile, Confirm: true, Preview: true}
	if err := ValidateExecutionSnapshotAction(req); err == nil {
		t.Fatal("confirm+preview reconcile must be rejected")
	}
}

func TestModeSwitchRecordFingerprintTracksRecordContent(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{WorktreePath: "/w", PlanPath: "p.md"}
	first, err := ModeSwitchRecordFingerprint(record)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 || !validSHA256(first) {
		t.Fatalf("fingerprint must be a sha256 hex digest: %q", first)
	}
	again, _ := ModeSwitchRecordFingerprint(record)
	if again != first {
		t.Fatal("fingerprint must be deterministic")
	}
	record.PlanPath = "q.md"
	changed, _ := ModeSwitchRecordFingerprint(record)
	if changed == first {
		t.Fatal("fingerprint must change when the record changes")
	}
	reset, _ := ModeSwitchRecordFingerprint(ResetExecutionForModeSwitch(record))
	if reset == changed {
		t.Fatal("reset record must fingerprint differently")
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
