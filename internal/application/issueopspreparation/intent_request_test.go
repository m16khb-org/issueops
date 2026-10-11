package issueopspreparation

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

type intentFilesFake struct {
	token       string
	tokenErr    error
	artifacts   map[string]string
	packetPath  string
	promptPath  string
	readTokens  int
	readFiles   []string
	artifactErr error
}

func (f *intentFilesFake) SamePath(a, b string) bool {
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}
func (f *intentFilesFake) ArtifactPaths(preparationcontract.Record) (string, string) {
	return f.packetPath, f.promptPath
}
func (f *intentFilesFake) ReadToken(preparationcontract.Record) (string, error) {
	f.readTokens++
	return f.token, f.tokenErr
}
func (f *intentFilesFake) ReadArtifact(root, path string) ([]byte, error) {
	f.readFiles = append(f.readFiles, root+"|"+path)
	if f.artifactErr != nil {
		return nil, f.artifactErr
	}
	content, ok := f.artifacts[path]
	if !ok {
		return nil, errors.New("missing artifact " + path)
	}
	return []byte(content), nil
}

type sealedIntentFixture struct {
	record  preparationcontract.Record
	intent  preparationcontract.Intent
	files   *intentFilesFake
	builder IntentRequestBuilder
}

// newSealedTerminalIntent drives the real begin and worktree-receipt
// transitions so the record and intent satisfy the sealed-intent contract.
func newSealedTerminalIntent(t *testing.T) sealedIntentFixture {
	t.Helper()
	record := leasecontract.Record{
		ID: "io-1", IssueURL: "https://github.com/acme/repo/issues/21",
		BranchPrepare: json.RawMessage(`{"provider":"github","issue_url":"https://github.com/acme/repo/issues/21","link_verified":true}`),
	}
	begin := OrcaBegin{
		Workspace:   preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: "base"},
		Command:     preparationcontract.Command{OwnerHost: "codex", OwnerModel: "model", OwnerEffort: "high"},
		Probe:       preparationcontract.ProbeRequest{Repo: "/source", Host: "codex", Model: "model", Effort: "high", Provider: "github", Issue: 21},
		Owner:       preparationcontract.OwnerEvidence{Provider: "github", Issue: 21, BodySHA256: strings.Repeat("a", 64)},
		OperationID: "0123456789abcdef0123456789abcdef", StartedAt: "2026-08-02T00:00:00Z",
	}
	recorded, intent, err := ApplyOrcaBegin(record, begin, "artifact-dir")
	if err != nil {
		t.Fatal(err)
	}
	files := &intentFilesFake{
		token:      "claim-token",
		packetPath: "/worktree/.issueops/packet.json", promptPath: "/worktree/.issueops/prompt.md",
		artifacts: map[string]string{"/worktree/.issueops/packet.json": "packet-bytes", "/worktree/.issueops/prompt.md": "owner prompt"},
	}
	state := IntentState{
		Intent:   intent,
		Snapshot: preparationcontract.Snapshot{Record: recorded},
		OwnerArtifacts: preparationcontract.OwnerArtifacts{
			PlanPath: "plan", ClaimTokenSHA256: intentDigest([]byte(files.token)),
			OwnerPromptPath: files.promptPath, OwnerPromptSHA256: intentDigest([]byte(files.artifacts[files.promptPath])),
			ContextPacketPath: files.packetPath, ContextPacketSHA256: intentDigest([]byte(files.artifacts[files.packetPath])),
		},
	}
	receipt := preparationcontract.IntentReceipt{Workspace: &preparationcontract.OrcaWorkspaceReceipt{
		Workspace: preparationcontract.WorkspaceReceipt{SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: "base", Driver: "orca"},
		RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt",
	}}
	decision, err := ApplyOrcaReceipt(state, receipt, "artifact-dir", nil)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Intent.Stage != preparationcontract.IntentStageTerminal {
		t.Fatalf("stage=%s", decision.Intent.Stage)
	}
	return sealedIntentFixture{record: decision.Record, intent: decision.Intent, files: files, builder: IntentRequestBuilder{Files: files}}
}

func TestIntentRequestBuilderBuildHydratesSealedLaunchOnlyWhenDigestsHold(t *testing.T) {
	f := newSealedTerminalIntent(t)
	request, err := f.builder.Build(f.record, f.intent)
	if err != nil {
		t.Fatal(err)
	}
	if request.Launch == nil || request.Launch.Prompt != "owner prompt" || request.Launch.PromptSHA256 != f.intent.Launch.PromptSHA256 || request.Stage != preparationcontract.IntentStageTerminal ||
		request.Marker != f.intent.Marker || request.OperationID != f.intent.OperationID || request.Prepared == nil || request.Prepared.RuntimeID != "rt" {
		t.Fatalf("request=%+v", request)
	}
	if f.files.readFiles[0] != "/worktree|"+f.files.promptPath || f.files.readFiles[1] != "/worktree|"+f.files.packetPath {
		t.Fatalf("artifacts must be read under the workspace root: %v", f.files.readFiles)
	}

	t.Run("token drift", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.files.token = "rotated"
		if _, err := f.builder.Build(f.record, f.intent); err == nil || !strings.Contains(err.Error(), "claim token identity changed") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("token unreadable", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.files.tokenErr = errors.New("gone")
		if _, err := f.builder.Build(f.record, f.intent); err == nil || !strings.Contains(err.Error(), "claim token identity changed") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("prompt drift", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.files.artifacts[f.files.promptPath] = "tampered prompt"
		if _, err := f.builder.Build(f.record, f.intent); err == nil || !strings.Contains(err.Error(), "owner prompt identity changed") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("packet drift", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.files.artifacts[f.files.packetPath] = "tampered packet"
		if _, err := f.builder.Build(f.record, f.intent); err == nil || !strings.Contains(err.Error(), "context packet identity changed") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("prompt unreadable", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.files.artifactErr = errors.New("io")
		if _, err := f.builder.Build(f.record, f.intent); err == nil || !strings.Contains(err.Error(), "owner prompt identity changed") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("authority changed", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.record.Execution.Pending.Marker = "other"
		if _, err := f.builder.Build(f.record, f.intent); err == nil || !strings.Contains(err.Error(), "authority changed") {
			t.Fatalf("err=%v", err)
		}
		if f.files.readTokens != 0 {
			t.Fatal("artifacts must not be read before authority holds")
		}
	})
}

func TestIntentRequestBuilderBuildWorktreeStageNeedsNoLaunchFiles(t *testing.T) {
	f := newSealedTerminalIntent(t)
	record := leasecontract.Record{
		ID: "io-1", IssueURL: "https://github.com/acme/repo/issues/21",
		BranchPrepare: json.RawMessage(`{"provider":"github","issue_url":"https://github.com/acme/repo/issues/21","link_verified":true}`),
	}
	begin := OrcaBegin{
		Workspace:   preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: "base"},
		Command:     preparationcontract.Command{OwnerHost: "codex", OwnerModel: "model", OwnerEffort: "high"},
		Probe:       preparationcontract.ProbeRequest{Repo: "/source", Host: "codex", Model: "model", Effort: "high", Provider: "github", Issue: 21},
		Owner:       preparationcontract.OwnerEvidence{Provider: "github", Issue: 21, BodySHA256: strings.Repeat("a", 64)},
		OperationID: "0123456789abcdef0123456789abcdef", StartedAt: "2026-08-02T00:00:00Z",
	}
	recorded, intent, err := ApplyOrcaBegin(record, begin, "artifact-dir")
	if err != nil {
		t.Fatal(err)
	}
	request, err := f.builder.Build(recorded, intent)
	if err != nil {
		t.Fatal(err)
	}
	if request.Launch != nil || request.Stage != preparationcontract.IntentStageWorktree || request.Marker != intent.Marker {
		t.Fatalf("request=%+v", request)
	}
	if f.files.readTokens != 0 || len(f.files.readFiles) != 0 {
		t.Fatal("worktree stage must not read sealed launch files")
	}
}

func TestIntentRequestBuilderInspectUsesSealedMetadataOnly(t *testing.T) {
	f := newSealedTerminalIntent(t)
	request, err := f.builder.Inspect(f.record, f.intent)
	if err != nil {
		t.Fatal(err)
	}
	if request.Launch == nil || request.Launch.Prompt != "" || request.Launch.PromptPath != f.files.promptPath {
		t.Fatalf("launch=%+v", request.Launch)
	}
	if f.files.readTokens != 0 || len(f.files.readFiles) != 0 {
		t.Fatal("Inspect must not open token or owner artifacts")
	}

	t.Run("artifact path drift", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.files.promptPath = "/elsewhere/prompt.md"
		if _, err := f.builder.Inspect(f.record, f.intent); err == nil || !strings.Contains(err.Error(), "owner artifact path changed") {
			t.Fatalf("err=%v", err)
		}
		f = newSealedTerminalIntent(t)
		f.files.packetPath = "/elsewhere/packet.json"
		if _, err := f.builder.Inspect(f.record, f.intent); err == nil || !strings.Contains(err.Error(), "owner artifact path changed") {
			t.Fatalf("packet err=%v", err)
		}
	})
	t.Run("invalid payload", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		intent := f.intent
		intent.Marker = ""
		if _, err := f.builder.Inspect(f.record, intent); err == nil {
			t.Fatal("unsealed payload must be rejected")
		}
	})
	t.Run("workspace root drift", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.record.Execution.Workspace.Root = "/moved"
		if _, err := f.builder.Inspect(f.record, f.intent); err == nil || !strings.Contains(err.Error(), "record identity changed") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("prepared workspace drift", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.record.WorktreePath = "/moved"
		if _, err := f.builder.Inspect(f.record, f.intent); err == nil || !strings.Contains(err.Error(), "prepared workspace identity changed") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("parent worktree compared only when both present", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.record.Execution.Workspace.ParentWorktree = "/parent"
		if _, err := f.builder.Inspect(f.record, f.intent); err == nil {
			t.Fatal("record parent without sealed parent must mismatch")
		}
		f = newSealedTerminalIntent(t)
		f.record.Execution.Workspace.ParentWorktree = "  "
		if _, err := f.builder.Inspect(f.record, f.intent); err != nil {
			t.Fatalf("blank parent equals absent parent: %v", err)
		}
	})
	t.Run("missing execution", func(t *testing.T) {
		f := newSealedTerminalIntent(t)
		f.record.Execution = nil
		if _, err := f.builder.Inspect(f.record, f.intent); err == nil {
			t.Fatal("record without execution must be rejected")
		}
	})
}

func TestLaunchHydratorRevalidatesSealedLaunchBeforeMaterializingPrompt(t *testing.T) {
	f := newSealedTerminalIntent(t)
	sealedRequest, err := f.builder.Inspect(f.record, f.intent)
	if err != nil {
		t.Fatal(err)
	}
	hydrator := func(record preparationcontract.Record, readRecordErr error, intent preparationcontract.Intent, readIntentErr error) LaunchHydrator {
		return LaunchHydrator{
			ReadRecord: func(id string) (preparationcontract.Record, error) {
				if id != "io-1" {
					t.Fatalf("id=%q", id)
				}
				return record, readRecordErr
			},
			ReadIntent: func(op string) (preparationcontract.Intent, error) {
				if op != f.intent.OperationID {
					t.Fatalf("operation=%q", op)
				}
				return intent, readIntentErr
			},
			Builder: f.builder,
		}
	}

	got, err := hydrator(f.record, nil, f.intent, nil).Hydrate("io-1", sealedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if got.Launch == nil || got.Launch.Prompt != "owner prompt" {
		t.Fatalf("launch=%+v", got.Launch)
	}
	if sealedRequest.Launch.Prompt != "" {
		t.Fatal("Hydrate must not mutate the caller's launch request")
	}

	boom := errors.New("boom")
	if _, err := hydrator(f.record, boom, f.intent, nil).Hydrate("io-1", sealedRequest); !errors.Is(err, boom) {
		t.Fatalf("record err=%v", err)
	}
	noExecution := f.record
	noExecution.Execution = nil
	if _, err := hydrator(noExecution, nil, f.intent, nil).Hydrate("io-1", sealedRequest); err == nil || !strings.Contains(err.Error(), "sealed Orca intent is unavailable") {
		t.Fatalf("no execution err=%v", err)
	}
	noPending := f.record
	execution := *f.record.Execution
	execution.Pending = nil
	noPending.Execution = &execution
	if _, err := hydrator(noPending, nil, f.intent, nil).Hydrate("io-1", sealedRequest); err == nil || !strings.Contains(err.Error(), "sealed Orca intent is unavailable") {
		t.Fatalf("no pending err=%v", err)
	}
	if _, err := hydrator(f.record, nil, f.intent, boom).Hydrate("io-1", sealedRequest); !errors.Is(err, boom) {
		t.Fatalf("intent err=%v", err)
	}

	changed := sealedRequest
	launch := *sealedRequest.Launch
	launch.PromptSHA256 = strings.Repeat("0", 64)
	changed.Launch = &launch
	if _, err := hydrator(f.record, nil, f.intent, nil).Hydrate("io-1", changed); err == nil || !strings.Contains(err.Error(), "owner launch identity changed") {
		t.Fatalf("changed launch err=%v", err)
	}

	f.files.artifacts[f.files.promptPath] = "tampered"
	if _, err := hydrator(f.record, nil, f.intent, nil).Hydrate("io-1", sealedRequest); err == nil || !strings.Contains(err.Error(), "owner prompt identity changed") {
		t.Fatalf("tampered prompt err=%v", err)
	}
}

func TestValidateIntentStateChecksRawEvidenceAndRecordAuthority(t *testing.T) {
	f := newSealedTerminalIntent(t)
	state := IntentState{Snapshot: preparationcontract.Snapshot{Record: f.record, RecordRaw: []byte("r")}, Intent: f.intent, IntentRaw: []byte("i")}
	if err := ValidateIntentState(state); err != nil {
		t.Fatal(err)
	}
	noIntentRaw := state
	noIntentRaw.IntentRaw = nil
	if err := ValidateIntentState(noIntentRaw); err == nil || !strings.Contains(err.Error(), "raw CAS evidence") {
		t.Fatalf("err=%v", err)
	}
	drifted := state
	execution := *f.record.Execution
	pending := *execution.Pending
	pending.Marker = "other"
	execution.Pending = &pending
	drifted.Snapshot.Record.Execution = &execution
	if err := ValidateIntentState(drifted); err == nil || !strings.Contains(err.Error(), "authority changed") {
		t.Fatalf("err=%v", err)
	}
}

func TestIntentDigestIsHexSHA256(t *testing.T) {
	// SHA-256 of the empty input is a well-known vector.
	if got := intentDigest(nil); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("digest=%s", got)
	}
}

func TestApplyOrcaReceiptIdentifierStages(t *testing.T) {
	f := newSealedTerminalIntent(t)
	state := IntentState{Intent: f.intent, Snapshot: preparationcontract.Snapshot{Record: f.record}}

	terminal, err := ApplyOrcaReceipt(state, preparationcontract.IntentReceipt{TerminalPTYID: "  pty-1 "}, "artifact-dir", nil)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Intent.TerminalPTYID != "pty-1" || terminal.Intent.Stage != preparationcontract.IntentStageRun || terminal.Complete {
		t.Fatalf("terminal=%+v", terminal.Intent)
	}
	if f.intent.TerminalPTYID != "" || f.intent.Stage != preparationcontract.IntentStageTerminal {
		t.Fatal("input intent must not be mutated")
	}

	state = IntentState{Intent: terminal.Intent, Snapshot: preparationcontract.Snapshot{Record: terminal.Record}}
	if _, err := ApplyOrcaReceipt(state, preparationcontract.IntentReceipt{RunID: "  "}, "artifact-dir", nil); err == nil || !strings.Contains(err.Error(), "Run candidate is incomplete") {
		t.Fatalf("run err=%v", err)
	}
	run, err := ApplyOrcaReceipt(state, preparationcontract.IntentReceipt{RunID: " run-1 "}, "artifact-dir", nil)
	if err != nil || run.Intent.RunID != "run-1" || run.Intent.Stage != preparationcontract.IntentStageRunBind {
		t.Fatalf("run=%+v err=%v", run.Intent, err)
	}

	state = IntentState{Intent: run.Intent, Snapshot: preparationcontract.Snapshot{Record: run.Record}}
	for name, receipt := range map[string]preparationcontract.IntentReceipt{
		"unbound":   {RunID: "run-1"},
		"other run": {RunID: "run-2", RunBound: true},
	} {
		if _, err := ApplyOrcaReceipt(state, receipt, "artifact-dir", nil); err == nil || !strings.Contains(err.Error(), "Run binding candidate is incomplete") {
			t.Fatalf("%s err=%v", name, err)
		}
	}
	bound, err := ApplyOrcaReceipt(state, preparationcontract.IntentReceipt{RunID: "run-1", RunBound: true}, "artifact-dir", nil)
	if err != nil || !bound.Intent.RunBound || bound.Intent.Stage != preparationcontract.IntentStageTask {
		t.Fatalf("bound=%+v err=%v", bound.Intent, err)
	}

	state = IntentState{Intent: bound.Intent, Snapshot: preparationcontract.Snapshot{Record: bound.Record}}
	if _, err := ApplyOrcaReceipt(state, preparationcontract.IntentReceipt{}, "artifact-dir", nil); err == nil || !strings.Contains(err.Error(), "task candidate is incomplete") {
		t.Fatalf("task err=%v", err)
	}
	task, err := ApplyOrcaReceipt(state, preparationcontract.IntentReceipt{TaskID: " task-1 "}, "artifact-dir", nil)
	if err != nil || task.Intent.TaskID != "task-1" || task.Intent.Stage != preparationcontract.IntentStageDispatch || task.Record.Execution.Pending.Kind != "dispatch" {
		t.Fatalf("task=%+v err=%v", task.Intent, err)
	}
}

func TestApplyOrcaReceiptDispatchCompletesPreparation(t *testing.T) {
	f := newSealedTerminalIntent(t)
	intent := f.intent
	intent.Stage = preparationcontract.IntentStageDispatch
	intent.TerminalPTYID, intent.RunID, intent.RunBound, intent.TaskID = "pty", "run", true, "task"
	state := IntentState{
		Intent: intent, Snapshot: preparationcontract.Snapshot{Record: f.record},
		Owner:          preparationcontract.OwnerEvidence{Source: "github"},
		OwnerArtifacts: preparationcontract.OwnerArtifacts{ClaimTokenPath: "/claim", ContextPacketPath: "/packet", ContextPacketSHA256: "cp", OwnerPromptPath: "/prompt", OwnerPromptSHA256: "op"},
	}
	receipt := preparationcontract.IntentReceipt{
		DispatchID: " d-1 ", RequestID: " req-1 ",
		PromptReceipt: &preparationcontract.PromptReceipt{RequestID: " 6f1f2f3e-0000-4000-8000-000000000001 "},
	}

	if _, err := ApplyOrcaReceipt(state, receipt, "artifact-dir", func() error { return errors.New("not delivered") }); err == nil || !strings.Contains(err.Error(), "dispatch candidate is incomplete: not delivered") {
		t.Fatalf("delivery err=%v", err)
	}
	missingPrepared := state
	missingPrepared.Intent.Prepared = nil
	if _, err := ApplyOrcaReceipt(missingPrepared, receipt, "artifact-dir", func() error { return nil }); err == nil || !strings.Contains(err.Error(), "prepared workspace receipt is missing") {
		t.Fatalf("prepared err=%v", err)
	}
	missingLaunch := state
	missingLaunch.Intent.Launch = nil
	if _, err := ApplyOrcaReceipt(missingLaunch, receipt, "artifact-dir", func() error { return nil }); err == nil || !strings.Contains(err.Error(), "sealed owner artifact identity is missing") {
		t.Fatalf("launch err=%v", err)
	}

	decision, err := ApplyOrcaReceipt(state, receipt, "artifact-dir", func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	execution := decision.Record.Execution
	if !decision.Complete || execution.Pending != nil || execution.Failure != nil || execution.Lease.Status != "claimable" || execution.Lease.Generation != 1 || execution.Lease.ClaimTokenSHA256 != intent.ClaimTokenSHA256 {
		t.Fatalf("execution=%+v", execution)
	}
	binding := execution.Orca
	if binding == nil || binding.RuntimeID != "rt" || binding.WorktreeID != "wt" || binding.RunID != "run" || binding.TaskID != "task" || binding.DispatchID != "d-1" || binding.TerminalPTYID != "pty" ||
		binding.OwnerHost != "codex" || binding.OwnerModel != "model" || binding.IssueBodySHA256 != intent.IssueBodySHA256 || binding.OwnerPromptSHA256 != intent.Launch.PromptSHA256 {
		t.Fatalf("binding=%+v", binding)
	}
	if decision.Intent.OrcaRequestID != "req-1" || decision.Intent.OrcaPromptRequestID != "6f1f2f3e-0000-4000-8000-000000000001" {
		t.Fatalf("request ids=%q/%q", decision.Intent.OrcaRequestID, decision.Intent.OrcaPromptRequestID)
	}
	res := decision.Result
	if !res.OK || res.ResolvedMode != preparationcontract.ModeOrca || res.ClaimTokenPath != "/claim" || res.OwnerPromptSHA256 != "op" || res.IssueSnapshotSource != "github" || res.Execution != execution {
		t.Fatalf("result=%+v", res)
	}
	if f.record.Execution.Orca != nil || f.record.Execution.Pending == nil {
		t.Fatal("input record must not be mutated")
	}
}

func TestApplyOrcaReceiptRejectsUnsupportedStageAndMissingArtifactDir(t *testing.T) {
	f := newSealedTerminalIntent(t)
	state := IntentState{Intent: f.intent, Snapshot: preparationcontract.Snapshot{Record: f.record}}
	if _, err := ApplyOrcaReceipt(state, preparationcontract.IntentReceipt{TerminalPTYID: "p"}, "  ", nil); err == nil || !strings.Contains(err.Error(), "no issue number") {
		t.Fatalf("artifact dir err=%v", err)
	}
	state.Intent.Stage = "bogus"
	if _, err := ApplyOrcaReceipt(state, preparationcontract.IntentReceipt{}, "artifact-dir", nil); err == nil || !strings.Contains(err.Error(), "unsupported Orca intent stage") {
		t.Fatalf("stage err=%v", err)
	}
	state.Intent.Stage = preparationcontract.IntentStageWorktree
	if _, err := ApplyOrcaReceipt(state, preparationcontract.IntentReceipt{}, "artifact-dir", nil); err == nil || !strings.Contains(err.Error(), "worktree candidate does not match") {
		t.Fatalf("worktree err=%v", err)
	}
}

func TestValidateSelectionReceiptsGateDirectCommitAndOrcaBegin(t *testing.T) {
	command := preparationcontract.Command{ID: "io", Mode: preparationcontract.ModeDirect, DirectReason: "planned recovery", Actor: leasecontract.Actor{Host: "codex"}}
	decision, err := preparationdomain.Decide(preparationdomain.DecisionInput{Command: command})
	if err != nil {
		t.Fatal(err)
	}
	good := leasecontract.Selection{RequestedMode: preparationcontract.ModeDirect, ResolvedMode: preparationcontract.ModeDirect, ReadinessFingerprint: decision.ReadinessFingerprint, SelectedAt: "t", ExplicitDirectReason: "planned recovery"}
	if err := ValidateDirectCommit(DirectCommit{Command: command, Selection: good}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOrcaBegin(OrcaBegin{Command: command, Selection: good}); err == nil || !strings.Contains(err.Error(), "does not match the chosen execution path") {
		t.Fatalf("orca begin must reject a direct selection: %v", err)
	}
	stale := good
	stale.ReadinessFingerprint = "stale"
	if err := ValidateDirectCommit(DirectCommit{Command: command, Selection: stale}); err == nil || !strings.Contains(err.Error(), "fingerprint changed") {
		t.Fatalf("stale err=%v", err)
	}
	unselected := good
	unselected.SelectedAt = " "
	if err := ValidateDirectCommit(DirectCommit{Command: command, Selection: unselected}); err == nil || !strings.Contains(err.Error(), "selected_at is required") {
		t.Fatalf("unselected err=%v", err)
	}
	wrongRequest := good
	wrongRequest.RequestedMode = preparationcontract.ModeAuto
	if err := ValidateDirectCommit(DirectCommit{Command: command, Selection: wrongRequest}); err == nil {
		t.Fatal("selection requested mode must match the command")
	}
}

func TestApplyDirectCommitRequiresArtifactDirAndKeepsInputPristine(t *testing.T) {
	record := leasecontract.Record{ID: "io-1"}
	commit := DirectCommit{Command: preparationcontract.Command{Actor: leasecontract.Actor{Host: "codex", SessionID: "s"}}, ArtifactDir: " "}
	after, _, err := ApplyDirectCommit(record, commit)
	if err == nil || !strings.Contains(err.Error(), "no issue number") || after.Execution != nil {
		t.Fatalf("after=%+v err=%v", after, err)
	}
}

func TestEnsureOrcaBeginUnprepared(t *testing.T) {
	if err := EnsureOrcaBeginUnprepared(leasecontract.Record{}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureOrcaBeginUnprepared(leasecontract.Record{Execution: &leasecontract.Execution{}}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err=%v", err)
	}
}

func TestApplyOrcaBeginRejectsMissingArtifactDirIdentityAndOwnerProfileDrift(t *testing.T) {
	record := leasecontract.Record{ID: "io-1", IssueURL: "https://github.com/acme/repo/issues/21", BranchPrepare: json.RawMessage(`{"provider":"github","issue_url":"https://github.com/acme/repo/issues/21","link_verified":true}`)}
	begin := OrcaBegin{
		Workspace:   preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: "/source", Root: "/worktree", Branch: "branch"},
		Command:     preparationcontract.Command{OwnerHost: "codex", OwnerModel: "model", OwnerEffort: "high"},
		Probe:       preparationcontract.ProbeRequest{Repo: "/source", Host: "codex", Model: "model", Effort: "high", Provider: "github", Issue: 21},
		Owner:       preparationcontract.OwnerEvidence{Provider: "github", Issue: 21, BodySHA256: strings.Repeat("a", 64)},
		OperationID: "0123456789abcdef0123456789abcdef", StartedAt: "2026-08-02T00:00:00Z",
	}
	if _, _, err := ApplyOrcaBegin(record, begin, ""); err == nil || !strings.Contains(err.Error(), "no issue number") {
		t.Fatalf("artifact dir err=%v", err)
	}
	noEvidence := record
	noEvidence.BranchPrepare = nil
	if _, _, err := ApplyOrcaBegin(noEvidence, begin, "dir"); err == nil || !strings.Contains(err.Error(), "verified branch issue identity") {
		t.Fatalf("identity err=%v", err)
	}
	probeDrift := begin
	probeDrift.Probe.Issue = 99
	if _, _, err := ApplyOrcaBegin(record, probeDrift, "dir"); err == nil || !strings.Contains(err.Error(), "owner issue identity changed") {
		t.Fatalf("probe drift err=%v", err)
	}
	for name, mutate := range map[string]func(*OrcaBegin){
		"host":   func(b *OrcaBegin) { b.Command.OwnerHost = "claude" },
		"model":  func(b *OrcaBegin) { b.Command.OwnerModel = "other" },
		"effort": func(b *OrcaBegin) { b.Command.OwnerEffort = "low" },
	} {
		drift := begin
		mutate(&drift)
		if _, _, err := ApplyOrcaBegin(record, drift, "dir"); err == nil || !strings.Contains(err.Error(), "owner profile changed") {
			t.Fatalf("%s err=%v", name, err)
		}
	}
	bad := begin
	bad.Owner.BodySHA256 = "short"
	if _, _, err := ApplyOrcaBegin(record, bad, "dir"); err == nil {
		t.Fatal("invalid issue body digest must fail intent sealing")
	}
}
