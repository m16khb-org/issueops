package issueopspreparation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func loadIntent(t *testing.T, raw string) Intent {
	t.Helper()
	var intent Intent
	if err := json.Unmarshal([]byte(raw), &intent); err != nil {
		t.Fatal(err)
	}
	return intent
}

func TestInvocationErrorWrapsCause(t *testing.T) {
	cause := errors.New("orca refused")
	err := &InvocationError{State: InvocationUnknown, Cause: cause}
	if err.Error() != "orca refused" || !errors.Is(err, cause) {
		t.Fatalf("InvocationError = %q, unwrap match %v", err.Error(), errors.Is(err, cause))
	}
	if wrapped, ok := errors.AsType[*InvocationError](errors.Join(err)); !ok || wrapped.State != InvocationUnknown {
		t.Fatalf("errors.AsType lost state: %v", wrapped)
	}
}

func TestIntentErrorOmitsBlankDetail(t *testing.T) {
	if got := (&IntentError{Code: "intent_marker_invalid", Detail: " \t"}).Error(); got != "intent_marker_invalid" {
		t.Fatalf("blank detail rendered %q", got)
	}
	if got := (&IntentError{Code: "c", Detail: "  d  "}).Error(); got != "c: d" {
		t.Fatalf("detail not trimmed: %q", got)
	}
}

func TestPlannerGateKeysPreservesOrderAndEmptiness(t *testing.T) {
	got := PlannerGateKeys([]PlannerGate{{Key: "design_review", Command: "a"}, {Key: "intent", Command: "b"}})
	if !slices.Equal(got, []string{"design_review", "intent"}) {
		t.Fatalf("keys = %v", got)
	}
	if got := PlannerGateKeys(nil); got == nil || len(got) != 0 {
		t.Fatalf("nil gates must give an empty non-nil slice, got %#v", got)
	}
}

func TestIntentCodecRejectsMalformedJSON(t *testing.T) {
	codec := IntentCodec{}
	if _, err := codec.Decode(prepareOperationID, []byte(`{"schema_version":`)); err == nil || !strings.Contains(err.Error(), "decode Orca external intent payload") {
		t.Fatalf("Decode malformed = %v", err)
	}
	if _, err := codec.DecodeSelfIdentified([]byte(`not json`)); err == nil || !strings.Contains(err.Error(), "decode Orca external intent payload") {
		t.Fatalf("DecodeSelfIdentified malformed = %v", err)
	}
}

func TestIntentCodecEncodeRefusesInvalidIntent(t *testing.T) {
	intent := loadIntent(t, prepareIntentJSON)
	intent.Generation = 0
	raw, err := (IntentCodec{}).Encode(intent)
	if err == nil || raw != nil {
		t.Fatalf("Encode of an invalid intent = %q, %v", raw, err)
	}
}

func TestIntentCodecEnvelopeAndProbeRejections(t *testing.T) {
	link := filepath.Join(t.TempDir(), "link")
	source := t.TempDir()
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	const invalid = "Orca external intent payload is invalid"
	for _, tc := range []struct {
		name   string
		mutate func(*Intent)
	}{
		{"schema version", func(i *Intent) { i.SchemaVersion = 99 }},
		{"empty lifecycle", func(i *Intent) { i.LifecycleID = ""; i.Workspace.LifecycleID = "" }},
		{"workspace lifecycle mismatch", func(i *Intent) { i.Workspace.LifecycleID = "other" }},
		{"missing started_at", func(i *Intent) { i.StartedAt = "" }},
		{"probe marker mismatch", func(i *Intent) { i.Probe.Marker += " " }},
		{"probe repo differs from source root", func(i *Intent) { i.Probe.Repo = t.TempDir() }},
		{"unsupported host", func(i *Intent) { i.Probe.Host = "gemini" }},
		{"blank model", func(i *Intent) { i.Probe.Model = "  " }},
		{"invocation state", func(i *Intent) { i.InvocationState = "invoked" }},
		{"negative attempts", func(i *Intent) { i.InvocationAttempts = -1 }},
		{"short body digest", func(i *Intent) { i.IssueBodySHA256 = "abc" }},
		{"uppercase body digest", func(i *Intent) { i.IssueBodySHA256 = strings.ToUpper(digestA) }},
		{"bad orca request id", func(i *Intent) { i.OrcaRequestID = "not-a-uuid" }},
		{"bad orca prompt request id", func(i *Intent) { i.OrcaPromptRequestID = "1234" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			intent := loadIntent(t, prepareIntentJSON)
			tc.mutate(&intent)
			if err := (IntentCodec{}).Validate(intent, intent.OperationID); err == nil || err.Error() != invalid {
				t.Fatalf("err = %v, want %q", err, invalid)
			}
		})
	}

	intent := loadIntent(t, prepareIntentJSON)
	intent.OrcaRequestID = "123e4567-E89B-12d3-a456-426614174000"
	intent.OrcaPromptRequestID = "123e4567-e89b-12d3-a456-426614174001"
	intent.InvocationState = InvocationUnknown
	intent.InvocationAttempts = MaxInvocationAttempts
	if err := (IntentCodec{}).Validate(intent, intent.OperationID); err != nil {
		t.Fatalf("valid UUIDs, unknown state, and max attempts rejected: %v", err)
	}
	// A symlinked probe repo resolves to the same source root.
	intent = loadIntent(t, prepareIntentJSON)
	intent.Workspace.SourceRoot = source
	intent.Probe.Repo = link
	if err := (IntentCodec{}).Validate(intent, intent.OperationID); err != nil {
		t.Fatalf("symlinked repo must match its target: %v", err)
	}
}

func TestIntentCodecWorktreeStageRejectsLaterStageReceipts(t *testing.T) {
	const want = "Orca worktree intent payload contains later-stage receipts"
	for name, mutate := range map[string]func(*Intent){
		"prepared":     func(i *Intent) { i.Prepared = &OrcaWorkspaceReceipt{} },
		"launch":       func(i *Intent) { i.Launch = &LaunchIdentity{} },
		"claim token":  func(i *Intent) { i.ClaimTokenSHA256 = digestB },
		"terminal pty": func(i *Intent) { i.TerminalPTYID = "pty" },
		"run id":       func(i *Intent) { i.RunID = "run" },
		"run bound":    func(i *Intent) { i.RunBound = true },
		"task id":      func(i *Intent) { i.TaskID = "task" },
	} {
		t.Run(name, func(t *testing.T) {
			intent := loadIntent(t, prepareIntentJSON)
			mutate(&intent)
			if err := (IntentCodec{}).Validate(intent, prepareOperationID); err == nil || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}

func TestIntentCodecRejectsUnsupportedStage(t *testing.T) {
	intent := loadIntent(t, prepareIntentJSON)
	intent.Stage = "teleport"
	if err := (IntentCodec{}).Validate(intent, prepareOperationID); err == nil || !strings.Contains(err.Error(), `unsupported Orca external intent stage "teleport"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestIntentCodecOwnerStagesRequireExactReceiptProgress(t *testing.T) {
	type progress struct {
		pty, run string
		bound    bool
		task     string
	}
	for _, tc := range []struct {
		name  string
		stage IntentStage
		got   progress
		want  string // empty means valid
	}{
		{"terminal with nothing", IntentStageTerminal, progress{}, ""},
		{"terminal with pty", IntentStageTerminal, progress{pty: "p"}, "Orca terminal intent payload contains later-stage receipts"},
		{"run without pty", IntentStageRun, progress{}, "Orca Run intent payload is incomplete"},
		{"run with pty", IntentStageRun, progress{pty: "p"}, ""},
		{"run with run id too early", IntentStageRun, progress{pty: "p", run: "r"}, "Orca Run intent payload is incomplete"},
		{"bind without run", IntentStageRunBind, progress{pty: "p"}, "Orca Run bind intent payload is incomplete"},
		{"bind ready", IntentStageRunBind, progress{pty: "p", run: "r"}, ""},
		{"bind already bound", IntentStageRunBind, progress{pty: "p", run: "r", bound: true}, "Orca Run bind intent payload is incomplete"},
		{"task before bind", IntentStageTask, progress{pty: "p", run: "r"}, "Orca task intent payload is incomplete"},
		{"task ready", IntentStageTask, progress{pty: "p", run: "r", bound: true}, ""},
		{"dispatch without task", IntentStageDispatch, progress{pty: "p", run: "r", bound: true}, "Orca dispatch intent payload is incomplete"},
		{"dispatch ready", IntentStageDispatch, progress{pty: "p", run: "r", bound: true, task: "t"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			intent := loadIntent(t, resumeIntentJSON)
			intent.Stage = tc.stage
			intent.TerminalPTYID, intent.RunID, intent.RunBound, intent.TaskID = tc.got.pty, tc.got.run, tc.got.bound, tc.got.task
			err := (IntentCodec{}).Validate(intent, resumeOperationID)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("valid progress rejected: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestIntentCodecOwnerStageRequiresSealedLaunchReceipts(t *testing.T) {
	const want = "Orca owner intent payload is missing sealed launch receipts"
	for name, mutate := range map[string]func(*Intent){
		"no prepared receipt":    func(i *Intent) { i.Prepared = nil },
		"no launch":              func(i *Intent) { i.Launch = nil },
		"bad prompt digest":      func(i *Intent) { i.Launch.PromptSHA256 = "xyz" },
		"bad context digest":     func(i *Intent) { i.Launch.ContextPacketSHA256 = "" },
		"blank prompt path":      func(i *Intent) { i.Launch.PromptPath = "  " },
		"blank context path":     func(i *Intent) { i.Launch.ContextPacketPath = "" },
		"workspace branch drift": func(i *Intent) { i.Prepared.Workspace.Branch = "other" },
		"workspace root drift":   func(i *Intent) { i.Prepared.Workspace.Root = "/elsewhere" },
		"workspace base drift":   func(i *Intent) { i.Prepared.Workspace.BaseHead = "deadbeef" },
		"non-orca driver":        func(i *Intent) { i.Prepared.Workspace.Driver = "git" },
		"blank runtime id":       func(i *Intent) { i.Prepared.RuntimeID = " " },
		"blank repo id":          func(i *Intent) { i.Prepared.RepoID = "" },
		"blank worktree id":      func(i *Intent) { i.Prepared.WorktreeID = "" },
		"unexpected parent":      func(i *Intent) { i.Prepared.Workspace.ParentWorktree = "/parent" },
		"missing parent":         func(i *Intent) { i.Workspace.ParentWorktree = "/parent" },
	} {
		t.Run(name, func(t *testing.T) {
			intent := loadIntent(t, resumeIntentJSON)
			mutate(&intent)
			if err := (IntentCodec{}).Validate(intent, resumeOperationID); err == nil || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
	// The claim token digest is checked on prepare-purpose owner stages, where no resume lease cross-checks it first.
	bad := loadIntent(t, resumeIntentJSON)
	bad.Purpose, bad.Generation, bad.PriorBinding, bad.ResumeLease, bad.ClaimTokenSHA256 = PurposePrepare, 1, nil, nil, "xyz"
	if err := (IntentCodec{}).Validate(bad, resumeOperationID); err == nil || err.Error() != want {
		t.Fatalf("bad claim token digest: err = %v, want %q", err, want)
	}
	// A receipt that names the same parent as the request is accepted.
	intent := loadIntent(t, resumeIntentJSON)
	intent.Workspace.ParentWorktree = "/parent"
	intent.Prepared.Workspace.ParentWorktree = "/parent"
	if err := (IntentCodec{}).Validate(intent, resumeOperationID); err != nil {
		t.Fatalf("matching parent rejected: %v", err)
	}
}

func TestIntentCodecResumeAuthorityRejections(t *testing.T) {
	const want = "Orca resume intent payload is invalid"
	for name, mutate := range map[string]func(*Intent){
		"worktree stage cannot resume": func(i *Intent) { i.Stage = IntentStageWorktree },
		"no prior binding":             func(i *Intent) { i.PriorBinding = nil },
		"lease generation drift":       func(i *Intent) { i.ResumeLease.Generation = 3 },
		"lease not claimable":          func(i *Intent) { i.ResumeLease.Status = "active" },
		"lease already held":           func(i *Intent) { i.ResumeLease.Holder = &leasecontract.Actor{Host: "codex"} },
		"claim token differs from lease": func(i *Intent) {
			i.ClaimTokenSHA256 = digestA
		},
		"lease token not a digest": func(i *Intent) {
			i.ResumeLease.ClaimTokenSHA256, i.ClaimTokenSHA256 = "short", "short"
		},
	} {
		t.Run(name, func(t *testing.T) {
			intent := loadIntent(t, resumeIntentJSON)
			mutate(&intent)
			if err := (IntentCodec{}).Validate(intent, resumeOperationID); err == nil || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
	intent := loadIntent(t, prepareIntentJSON)
	intent.PriorBinding = &ResumeBinding{}
	if err := (IntentCodec{}).Validate(intent, prepareOperationID); err == nil || err.Error() != "Orca prepare intent payload is invalid" {
		t.Fatalf("prepare intent with prior binding = %v", err)
	}
}

func TestRenderReadinessMarkerRejectsInvalidIdentity(t *testing.T) {
	codec := IntentCodec{}
	for name, call := range map[string]func() (string, error){
		"lifecycle with space": func() (string, error) {
			return codec.RenderReadinessMarker("io bad", IssueIdentity{Provider: "github", Issue: 1})
		},
		"empty lifecycle": func() (string, error) {
			return codec.RenderReadinessMarker("", IssueIdentity{Provider: "github", Issue: 1})
		},
		"lifecycle with equals": func() (string, error) {
			return codec.RenderReadinessMarker("a=b", IssueIdentity{Provider: "github", Issue: 1})
		},
		"unknown provider": func() (string, error) {
			return codec.RenderReadinessMarker("io-1", IssueIdentity{Provider: "bitbucket", Issue: 1})
		},
		"zero issue": func() (string, error) {
			return codec.RenderReadinessMarker("io-1", IssueIdentity{Provider: "gitlab", Issue: 0})
		},
	} {
		t.Run(name, func(t *testing.T) {
			marker, err := call()
			if intentErr, ok := errors.AsType[*IntentError](err); !ok || intentErr.Code != "intent_marker_invalid" || marker != "" {
				t.Fatalf("marker=%q err=%v", marker, err)
			}
		})
	}
}

func TestRenderMarkerResumeIdentityRules(t *testing.T) {
	codec := IntentCodec{}
	valid := MarkerIdentity{Purpose: PurposeResume, LifecycleID: "io-1", Generation: 3, OperationID: resumeOperationID, Provider: "gitlab", Issue: 7}
	marker, err := codec.RenderMarker(valid)
	if err != nil || marker != "issueops-v1 resume lifecycle=io-1 generation=3 operation="+resumeOperationID+" provider=gitlab issue=7" {
		t.Fatalf("resume marker = %q, %v", marker, err)
	}
	for name, mutate := range map[string]func(*MarkerIdentity){
		"generation zero":  func(m *MarkerIdentity) { m.Generation = 0 },
		"unknown purpose":  func(m *MarkerIdentity) { m.Purpose = "repair" },
		"empty operation":  func(m *MarkerIdentity) { m.OperationID = "" },
		"negative issue":   func(m *MarkerIdentity) { m.Issue = -4 },
		"padded lifecycle": func(m *MarkerIdentity) { m.LifecycleID = " io-1" },
	} {
		identity := valid
		mutate(&identity)
		if marker, err := codec.RenderMarker(identity); err == nil {
			t.Errorf("%s: rendered %q", name, marker)
		}
	}
}

func TestParseMarkerRejectsNonCanonicalOrMalformedFields(t *testing.T) {
	op := "operation=" + prepareOperationID
	for name, marker := range map[string]string{
		"wrong prefix":             "issueops-v2 lifecycle=io-1 " + op + " provider=github issue=1",
		"resume keyword missing":   "issueops-v1 lifecycle=io-1 generation=2 " + op + " provider=github issue=1",
		"wrong first field name":   "issueops-v1 life=io-1 " + op + " provider=github issue=1",
		"wrong operation name":     "issueops-v1 lifecycle=io-1 op=x provider=github issue=1",
		"non numeric issue":        "issueops-v1 lifecycle=io-1 " + op + " provider=github issue=abc",
		"plus-signed issue":        "issueops-v1 lifecycle=io-1 " + op + " provider=github issue=+5",
		"resume generation text":   "issueops-v1 resume lifecycle=io-1 generation=two " + op + " provider=github issue=1",
		"resume generation padded": "issueops-v1 resume lifecycle=io-1 generation=02 " + op + " provider=github issue=1",
		"resume generation zero":   "issueops-v1 resume lifecycle=io-1 generation=0 " + op + " provider=github issue=1",
		"resume wrong gen field":   "issueops-v1 resume lifecycle=io-1 gen=2 " + op + " provider=github issue=1",
		"double space":             "issueops-v1  lifecycle=io-1 " + op + " provider=github issue=1",
		"empty":                    "",
	} {
		if identity, err := parseMarker(marker); err == nil {
			t.Errorf("%s: accepted %q as %+v", name, marker, identity)
		}
	}
	identity, err := parseMarker("issueops-v1 resume lifecycle=io-1 generation=12 " + op + " provider=gitlab issue=9")
	if err != nil || identity != (MarkerIdentity{Purpose: PurposeResume, LifecycleID: "io-1", OperationID: prepareOperationID, Provider: "gitlab", Generation: 12, Issue: 9}) {
		t.Fatalf("resume parse = %+v, %v", identity, err)
	}
}

func TestPreparationCloneIsolatesEveryExecutionPointer(t *testing.T) {
	holder := &leasecontract.Actor{Host: "codex", SessionID: "s"}
	execution := &leasecontract.Execution{
		Selection: &leasecontract.Selection{ResolvedMode: "orca"},
		Lease:     leasecontract.Lease{Holder: holder},
		Orca:      &leasecontract.OrcaBinding{RuntimeID: "rt"},
		Pending:   &leasecontract.ExternalIntent{OperationID: "op"},
		Failure:   &leasecontract.FailureDetail{Code: "boom"},
	}
	cloned := Result{Execution: execution}.Clone()
	cloned.Execution.Selection.ResolvedMode = "direct"
	cloned.Execution.Orca.RuntimeID = "changed"
	cloned.Execution.Pending.OperationID = "changed"
	cloned.Execution.Failure.Code = "changed"
	cloned.Execution.Lease.Holder.Host = "claude"
	if execution.Selection.ResolvedMode != "orca" || execution.Orca.RuntimeID != "rt" || execution.Pending.OperationID != "op" ||
		execution.Failure.Code != "boom" || execution.Lease.Holder.Host != "codex" {
		t.Fatalf("clone shares pointers with its source: %+v", execution)
	}
	// Absent optional parts stay absent rather than becoming empty values.
	bare := Result{Execution: &leasecontract.Execution{}}.Clone()
	e := bare.Execution
	if e.Selection != nil || e.Orca != nil || e.Pending != nil || e.Failure != nil || e.Completion != nil ||
		e.CompletionHistory != nil || e.SyncBaseResolution != nil || e.SyncBaseEvents != nil || e.Lease.Holder != nil {
		t.Fatalf("bare clone materialized optional fields: %+v", e)
	}
	if got := (Result{}).Clone(); got.Execution != nil {
		t.Fatalf("nil execution cloned into %+v", got.Execution)
	}
}

func TestPreparationCloneKeepsNilVersusEmptyStrings(t *testing.T) {
	execution := &leasecontract.Execution{
		Completion: &leasecontract.Completion{},
		CompletionHistory: []leasecontract.CompletionHistoryEntry{
			{Completion: leasecontract.Completion{Verification: []string{}}},
		},
	}
	cloned := Result{Execution: execution}.Clone().Execution
	if cloned.Completion.Verification != nil {
		t.Fatalf("nil verification became %#v", cloned.Completion.Verification)
	}
	if v := cloned.CompletionHistory[0].Completion.Verification; v == nil || len(v) != 0 {
		t.Fatalf("empty verification became %#v", v)
	}
}
