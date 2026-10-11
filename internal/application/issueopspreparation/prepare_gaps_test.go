package issueopspreparation

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

func orcaConfirmFor(t *testing.T, mutate func(*preparationcontract.Command)) preparationcontract.Command {
	t.Helper()
	command := orcaCommand(false, preparationcontract.ModeOrca)
	mutate(&command)
	command.Confirm = true
	decision, err := preparationdomain.Decide(preparationdomain.DecisionInput{Command: command, Orca: preparationdomain.OrcaReadiness{Available: true, Ready: true, Provider: "github", Issue: 199}})
	if err != nil {
		t.Fatal(err)
	}
	command.ExpectedReadinessFingerprint = decision.ReadinessFingerprint
	return command
}

func TestPrepareFailsClosedWithoutRequiredCollaborators(t *testing.T) {
	cases := []struct {
		name    string
		build   func(f *orcaApplicationFixture) *Service
		command func(t *testing.T) preparationcontract.Command
		want    string
		absent  string
	}{
		{"repository", func(f *orcaApplicationFixture) *Service {
			return NewService(nil, &applicationClockFake{trace: &f.trace}, &orcaOperationIDFake{trace: &f.trace}, nil, f.gateway, f.evidence, f.models)
		}, func(*testing.T) preparationcontract.Command { return orcaCommand(false, preparationcontract.ModeOrca) }, "repository is unavailable", "load"},
		{"evidence", func(f *orcaApplicationFixture) *Service {
			return NewService(f.repository, &applicationClockFake{trace: &f.trace}, &orcaOperationIDFake{trace: &f.trace}, nil, f.gateway, nil, f.models)
		}, func(*testing.T) preparationcontract.Command { return orcaCommand(false, preparationcontract.ModeOrca) }, "evidence is unavailable", "root"},
		{"operation id generator", func(f *orcaApplicationFixture) *Service {
			return NewService(f.repository, &applicationClockFake{trace: &f.trace}, nil, nil, f.gateway, f.evidence, f.models)
		}, func(t *testing.T) preparationcontract.Command {
			return orcaConfirmFor(t, func(*preparationcontract.Command) {})
		}, "operation ID generator is unavailable", "begin"},
		{"clock", func(f *orcaApplicationFixture) *Service {
			return NewService(f.repository, nil, &orcaOperationIDFake{trace: &f.trace}, nil, f.gateway, f.evidence, f.models)
		}, func(t *testing.T) preparationcontract.Command {
			return orcaConfirmFor(t, func(*preparationcontract.Command) {})
		}, "clock is unavailable", "begin"},
		{"agent model resolver", func(f *orcaApplicationFixture) *Service {
			return NewService(f.repository, &applicationClockFake{trace: &f.trace}, &orcaOperationIDFake{trace: &f.trace}, nil, f.gateway, f.evidence, nil)
		}, func(*testing.T) preparationcontract.Command { return orcaCommand(false, preparationcontract.ModeOrca) }, "agent model resolver is unavailable", "root"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOrcaApplicationFixture()
			result, err := tc.build(fixture).Prepare(context.Background(), tc.command(t))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
			if result.OK || result.ID != "io-orca" {
				t.Fatalf("failed result must carry the id and not be OK: %+v", result)
			}
			if slices.Contains(fixture.trace, tc.absent) {
				t.Fatalf("trace %v must not reach %q", fixture.trace, tc.absent)
			}
		})
	}
}

func TestPrepareRejectsInvalidModeBeforeAnySideEffect(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	command := orcaCommand(false, "bogus")
	_, err := fixture.service.Prepare(context.Background(), command)
	if err == nil || !strings.Contains(err.Error(), "execution mode must be auto, direct, or orca") {
		t.Fatalf("err=%v", err)
	}
	assertOrcaTrace(t, fixture.trace, []string{"load"})
}

func TestPrepareOwnerDefaultsErrorWrapsResolverFailure(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	fixture.models.ownerErr = errors.New("settings file broken")
	_, err := fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca))
	if err == nil || !strings.Contains(err.Error(), "resolve owner model") || !errors.Is(err, fixture.models.ownerErr) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrepareStopsBeforeProbeWhenWorkspaceEvidenceFails(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	failing := &workspaceFailingEvidence{orcaApplicationEvidenceFake: fixture.evidence, err: errors.New("no canonical root")}
	service := NewService(fixture.repository, &applicationClockFake{trace: &fixture.trace}, &orcaOperationIDFake{trace: &fixture.trace}, nil, fixture.gateway, failing, fixture.models)
	result, err := service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca))
	if !errors.Is(err, failing.err) || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertOrcaTrace(t, fixture.trace, []string{"load"})
}

type workspaceFailingEvidence struct {
	*orcaApplicationEvidenceFake
	err error
}

func (f *workspaceFailingEvidence) Workspace(preparationcontract.Snapshot, bool) (preparationcontract.WorkspaceRequest, error) {
	return preparationcontract.WorkspaceRequest{}, f.err
}

func TestPrepareOrcaRefusesToLaunchWithoutPlannerOwnedRecords(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	fixture.repository.snapshot.Record.Intent = nil
	fixture.repository.snapshot.Record.DesignReview = nil
	result, err := fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca))
	if err == nil || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, want := range []string{"intent_contract", "design_review", "issueops intent record --id io-orca", "issueops design review --id io-orca"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "devils_advocate_review") {
		t.Fatalf("satisfied gate must not be listed: %v", err)
	}
	if slices.Contains(fixture.trace, "orca.probe") {
		t.Fatalf("probe must not run before planner gates pass: %v", fixture.trace)
	}
}

func TestPrepareOrcaRejectsOwnerHostOutsideSupportedSet(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	command := orcaCommand(false, preparationcontract.ModeOrca)
	command.OwnerHost = "gemini"
	_, err := fixture.service.Prepare(context.Background(), command)
	if err == nil || !strings.Contains(err.Error(), "owner_host must be codex, claude, omo, or omp") {
		t.Fatalf("err=%v", err)
	}
	if slices.Contains(fixture.trace, "orca.probe") {
		t.Fatalf("unsupported host must not be probed: %v", fixture.trace)
	}
}

func TestPrepareOrcaRequiresVerifiedIssueIdentity(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	fixture.repository.snapshot.Record.BranchPrepare = nil
	_, err := fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca))
	if err == nil || !strings.Contains(err.Error(), "verified branch issue identity") {
		t.Fatalf("err=%v", err)
	}
	if slices.Contains(fixture.trace, "orca.probe") {
		t.Fatalf("probe must not run without issue identity: %v", fixture.trace)
	}
}

func TestPrepareProbeTransportErrorDeniesOrcaButAutoFallsBack(t *testing.T) {
	probeErr := errors.New("socket closed")

	explicit := newOrcaApplicationFixture()
	explicit.gateway.probe = preparationcontract.ProbeResult{Available: true, Ready: true}
	explicit.gateway.probeErr = probeErr
	result, err := explicit.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca))
	if !errors.Is(err, probeErr) || result.OK || result.RequestedMode != preparationcontract.ModeOrca {
		t.Fatalf("explicit orca result=%+v err=%v", result, err)
	}

	auto := newOrcaApplicationFixture()
	auto.gateway.probe = preparationcontract.ProbeResult{Available: true, Ready: true}
	auto.gateway.probeErr = probeErr
	result, err = auto.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeAuto))
	if err != nil {
		t.Fatal(err)
	}
	if result.ResolvedMode != preparationcontract.ModeDirect || result.FallbackCode != "orca_probe_failed" || result.ProbeReady {
		t.Fatalf("a probe error must not count as ready: %+v", result)
	}
}

func TestPrepareOrcaMissingGatewayFallsBackOnlyForAuto(t *testing.T) {
	build := func() (*orcaApplicationFixture, *Service) {
		f := newOrcaApplicationFixture()
		direct := &applicationDirectFake{trace: &f.trace, access: preparationcontract.AccessResult{Allowed: true}, receipt: preparationcontract.WorkspaceReceipt{SourceRoot: "/repo", Root: "/repo.worktrees/199-orca", Branch: "199-orca", BaseHead: "base", Driver: "git"}}
		return f, NewService(f.repository, &applicationClockFake{trace: &f.trace}, &orcaOperationIDFake{trace: &f.trace}, direct, nil, f.evidence, f.models)
	}
	_, service := build()
	if _, err := service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca)); err == nil || !strings.Contains(err.Error(), "Orca provisioner is unavailable") {
		t.Fatalf("explicit orca err=%v", err)
	}
	_, service = build()
	result, err := service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeAuto))
	if err != nil || result.ResolvedMode != preparationcontract.ModeDirect || result.FallbackCode != "orca_adapter_unavailable" {
		t.Fatalf("auto result=%+v err=%v", result, err)
	}
}

func TestPrepareOrcaOwnerEvidenceIdentityMustMatchProbe(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	fixture.evidence.owner.Provider, fixture.evidence.owner.Issue = "github", 7
	_, err := fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca))
	if err == nil || !strings.Contains(err.Error(), "owner issue identity changed") {
		t.Fatalf("issue mismatch err=%v", err)
	}

	fixture = newOrcaApplicationFixture()
	fixture.evidence.owner.Provider, fixture.evidence.owner.Issue = "gitlab", 199
	if _, err = fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca)); err == nil || !strings.Contains(err.Error(), "owner issue identity changed") {
		t.Fatalf("provider mismatch err=%v", err)
	}

	fixture = newOrcaApplicationFixture()
	fixture.evidence.ownerErr = errors.New("issue unreadable")
	if _, err = fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca)); !errors.Is(err, fixture.evidence.ownerErr) {
		t.Fatalf("owner read err=%v", err)
	}
}

func TestPrepareOrcaConfirmRequiresNativeActor(t *testing.T) {
	cases := map[string]struct {
		actor leasecontract.Actor
		want  string
	}{
		"host":    {leasecontract.Actor{Host: "gemini", SessionID: "s", SessionProcess: &leasecontract.ProcessReceipt{PID: 1, StartedAt: "t", Executable: "/x"}}, "native actor host"},
		"session": {leasecontract.Actor{Host: "codex", SessionProcess: &leasecontract.ProcessReceipt{PID: 1, StartedAt: "t", Executable: "/x"}}, "session_id is required"},
		"process": {leasecontract.Actor{Host: "codex", SessionID: "s"}, "session_process receipt"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newOrcaApplicationFixture()
			command := orcaConfirmFor(t, func(c *preparationcontract.Command) { c.Actor = tc.actor })
			_, err := fixture.service.Prepare(context.Background(), command)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
			if slices.Contains(fixture.trace, "begin") {
				t.Fatalf("intent must not begin for an invalid actor: %v", fixture.trace)
			}
		})
	}
}

func TestNormalizeActorCanonicalizesAndRequiresPIDReuseSafeReceipt(t *testing.T) {
	valid := leasecontract.Actor{Host: " Claude ", SessionID: " s1 ", AgentID: " a ", SessionProcess: &leasecontract.ProcessReceipt{PID: 9, StartedAt: " t ", Executable: " /bin/claude "}}
	got, err := normalizeActor(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "claude" || got.SessionID != "s1" || got.AgentID != "a" || got.SessionProcess.StartedAt != "t" || got.SessionProcess.Executable != "/bin/claude" {
		t.Fatalf("actor=%+v", got)
	}
	if valid.SessionProcess.StartedAt != " t " {
		t.Fatal("input receipt must not be mutated")
	}
	for name, mutate := range map[string]func(*leasecontract.ProcessReceipt){
		"pid":        func(p *leasecontract.ProcessReceipt) { p.PID = 0 },
		"started_at": func(p *leasecontract.ProcessReceipt) { p.StartedAt = "  " },
		"executable": func(p *leasecontract.ProcessReceipt) { p.Executable = "" },
	} {
		actor := valid
		process := *valid.SessionProcess
		mutate(&process)
		actor.SessionProcess = &process
		if _, err := normalizeActor(actor); err == nil || !strings.Contains(err.Error(), "PID reuse-safe") {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
}

func TestPrepareDirectFailsClosedWithoutProvisionerOrClock(t *testing.T) {
	fixture := newDirectServiceFixture()
	service := NewService(fixture.repository, &applicationClockFake{trace: &fixture.trace}, applicationOperationIDFake{}, nil, applicationOrcaFake{}, fixture.evidence, &agentModelsFake{})
	_, err := service.Prepare(context.Background(), directCommand(false))
	if err == nil || !strings.Contains(err.Error(), "direct Git worktree provisioner is unavailable") {
		t.Fatalf("err=%v", err)
	}

	fixture = newDirectServiceFixture()
	service = NewService(fixture.repository, nil, applicationOperationIDFake{}, fixture.direct, applicationOrcaFake{}, fixture.evidence, &agentModelsFake{})
	_, err = service.Prepare(context.Background(), directCommand(true))
	if err == nil || !strings.Contains(err.Error(), "clock is unavailable") {
		t.Fatalf("err=%v", err)
	}
	if fixture.repository.commit != nil || slices.Contains(fixture.trace, "evidence.materialize") {
		t.Fatalf("must not materialize or commit without a clock: %v", fixture.trace)
	}
}

func TestPrepareDirectConfirmRejectsInvalidActorAndAccessFailures(t *testing.T) {
	fixture := newDirectServiceFixture()
	command := directCommand(true)
	command.Actor.SessionProcess = nil
	if _, err := fixture.service.Prepare(context.Background(), command); err == nil || !strings.Contains(err.Error(), "session_process receipt") {
		t.Fatalf("actor err=%v", err)
	}
	if slices.Contains(fixture.trace, "direct.prepare") {
		t.Fatalf("must not provision for invalid actor: %v", fixture.trace)
	}

	fixture = newDirectServiceFixture()
	fixture.direct.accessErr = errors.New("probe exploded")
	if _, err := fixture.service.Prepare(context.Background(), directCommand(true)); !errors.Is(err, fixture.direct.accessErr) {
		t.Fatalf("access err=%v", err)
	}
	if slices.Contains(fixture.trace, "direct.prepare") {
		t.Fatalf("must not provision after access probe error: %v", fixture.trace)
	}

	fixture = newDirectServiceFixture()
	fixture.direct.access = preparationcontract.AccessResult{Allowed: false, RelaunchCommand: "codex --add-dir /base"}
	result, err := fixture.service.Prepare(context.Background(), directCommand(true))
	if err == nil || !strings.Contains(err.Error(), "relaunch with: codex --add-dir /base") || result.NextCommand != "codex --add-dir /base" || result.OK {
		t.Fatalf("denied access result=%+v err=%v", result, err)
	}
	if slices.Contains(fixture.trace, "direct.prepare") {
		t.Fatalf("must not provision when base is inaccessible: %v", fixture.trace)
	}
}

func TestPrepareDirectPreviewIncludesActorAndAgentFlagsInConfirmCommand(t *testing.T) {
	fixture := newDirectServiceFixture()
	command := directCommand(false)
	command.Actor.AgentID = "agent-7"
	result, err := fixture.service.Prepare(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--agent-id 'agent-7'", "--session-id 'session'", "--session-pid 42", "--direct-reason 'planned recovery'", "--confirm --json"} {
		if !strings.Contains(result.NextCommand, want) {
			t.Fatalf("confirm command %q lacks %q", result.NextCommand, want)
		}
	}
}

func TestPrepareExistingExecutionReportsRecoveryCommands(t *testing.T) {
	cases := []struct {
		name       string
		mode       string
		status     string
		pending    bool
		request    string
		wantOK     bool
		wantErr    string
		wantNext   string
		wantResult string
	}{
		{"existing active", "direct", "active", false, "direct", true, "", "", "direct"},
		{"pending", "orca", "released", true, "auto", false, "pending external intent", "issueops execution reconcile --id io-prepare --preview ACTOR_FLAGS", "orca"},
		{"mode mismatch", "direct", "active", false, "orca", false, "switching to orca removes the canonical worktree", "issueops execution switch-mode --id io-prepare --mode orca --json", "direct"},
		{"claimable direct", "direct", "claimable", false, "auto", false, "generation 1 is claimable and has no writer", "issueops execution claim --id 'io-prepare' --generation 1 --claim-current-token", "direct"},
		{"claimable orca without owner", "orca", "claimable", false, "auto", false, "Orca generation 1 has no current owner", "issueops execution resume --id 'io-prepare' --expected-generation 1 --confirm", "orca"},
		{"released", "direct", "released", false, "auto", false, "was released and has no writer", "issueops execution replace --id 'io-prepare' --expected-generation 1 --preview", "direct"},
		{"revoking", "direct", "revoking", false, "auto", false, "is revoking and has no writer", "issueops execution replace --id 'io-prepare' --expected-generation 1 --finalize-preview", "direct"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newDirectServiceFixture()
			fixture.repository.snapshot.Record.Execution = applicationExecution(tc.mode, tc.status, tc.pending)
			command := directCommand(false)
			command.Mode = tc.request
			command.Confirm = strings.Contains(tc.name, "claimable") || tc.name == "released" || tc.name == "revoking"
			result, err := fixture.service.Prepare(context.Background(), command)
			if tc.wantErr == "" {
				if err != nil || !result.OK {
					t.Fatalf("result=%+v err=%v", result, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) || result.OK {
				t.Fatalf("result=%+v err=%v want %q", result, err, tc.wantErr)
			}
			if result.NextCommand != tc.wantNext || result.ResolvedMode != tc.wantResult {
				t.Fatalf("next=%q resolved=%q", result.NextCommand, result.ResolvedMode)
			}
			if tc.wantNext != "" && tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantNext) {
				t.Fatalf("error must embed the recovery command: %v", err)
			}
			if slices.Contains(fixture.trace, "direct.prepare") {
				t.Fatalf("existing execution must not provision: %v", fixture.trace)
			}
		})
	}
}

func TestWriterlessNextCommandAndErrorForActiveLeaseHaveNoRecovery(t *testing.T) {
	record := leasecontract.Record{ID: "io-1", Execution: applicationExecution("direct", "active", false)}
	if got := writerlessNextCommand(preparationcontract.Snapshot{Record: record}); got != "" {
		t.Fatalf("active lease has no recovery command, got %q", got)
	}
	if err := writerlessError(record, ""); err == nil || err.Error() != "IssueOps execution has no writer" {
		t.Fatalf("err=%v", err)
	}
}

func TestPreparedResultPrefersSelectionReceiptAndClonesExecution(t *testing.T) {
	record := leasecontract.Record{ID: "io-1", Execution: applicationExecution("direct", "active", false)}
	plain := preparedResult(record, "auto", "fb")
	if plain.RequestedMode != "auto" || plain.FallbackCode != "fb" || plain.ResolvedMode != "direct" || !plain.OK {
		t.Fatalf("plain=%+v", plain)
	}
	record.Execution.Selection = &leasecontract.Selection{
		RequestedMode: "orca", ResolvedMode: "direct", FallbackCode: "orca_unready", ProbeAttempted: true, ProbeAvailable: true,
		ProbeCode: "orca_unready", ReadinessFingerprint: "fp", ExplicitDirectReason: "why",
	}
	sel := preparedResult(record, "auto", "ignored")
	if sel.RequestedMode != "orca" || sel.FallbackCode != "orca_unready" || !sel.ProbeAttempted || !sel.ProbeAvailable || sel.ProbeReady || sel.ReadinessFingerprint != "fp" || sel.ExplicitDirectReason != "why" {
		t.Fatalf("sel=%+v", sel)
	}
	sel.Execution.Workspace.Root = "mutated"
	if record.Execution.Workspace.Root == "mutated" {
		t.Fatal("result execution must be a clone")
	}
	if cloneExecution(nil) != nil {
		t.Fatal("nil execution must clone to nil")
	}
}

type orcaFaultGateway struct {
	*orcaApplicationGatewayFake
	inspectErr, invokeErr error
}

func (g *orcaFaultGateway) Inspect(ctx context.Context, request preparationcontract.IntentRequest) (preparationcontract.IntentInventory, error) {
	if g.inspectErr != nil {
		*g.trace = append(*g.trace, "inspect:"+string(request.Stage))
		return preparationcontract.IntentInventory{}, g.inspectErr
	}
	return g.orcaApplicationGatewayFake.Inspect(ctx, request)
}

func (g *orcaFaultGateway) Invoke(ctx context.Context, request preparationcontract.IntentRequest) (preparationcontract.IntentReceipt, error) {
	if g.invokeErr != nil {
		*g.trace = append(*g.trace, "invoke:"+string(request.Stage))
		return preparationcontract.IntentReceipt{}, g.invokeErr
	}
	return g.orcaApplicationGatewayFake.Invoke(ctx, request)
}

type orcaFaultRepository struct {
	*orcaApplicationRepositoryFake
	markErr, applyErr error
}

func (r *orcaFaultRepository) MarkInvoking(ctx context.Context, state IntentState) (IntentState, error) {
	if r.markErr != nil {
		*r.trace = append(*r.trace, "mark:"+string(state.Intent.Stage))
		return state, r.markErr
	}
	return r.orcaApplicationRepositoryFake.MarkInvoking(ctx, state)
}

func (r *orcaFaultRepository) ApplyReceipt(ctx context.Context, state IntentState, receipt preparationcontract.IntentReceipt) (IntentProgress, error) {
	if r.applyErr != nil {
		*r.trace = append(*r.trace, "apply:"+string(state.Intent.Stage))
		return IntentProgress{}, r.applyErr
	}
	return r.orcaApplicationRepositoryFake.ApplyReceipt(ctx, state, receipt)
}

func faultService(f *orcaApplicationFixture, repo *orcaFaultRepository, gw *orcaFaultGateway, clock Clock) *Service {
	return NewService(repo, clock, &orcaOperationIDFake{trace: &f.trace}, nil, gw, f.evidence, f.models)
}

func TestPrepareOrcaRetainsIntentAndRecordsInvocationStateOnFailure(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name        string
		setup       func(*orcaFaultRepository, *orcaFaultGateway)
		wantErr     string
		wantFailure string
		wantInvokes int
	}{
		{"inventory error", func(_ *orcaFaultRepository, g *orcaFaultGateway) { g.inspectErr = boom },
			"inventory is ambiguous; intent retained", "failure:" + preparationcontract.InvocationNotInvoked, 0},
		{"mark invoking error", func(r *orcaFaultRepository, _ *orcaFaultGateway) { r.markErr = boom }, "boom", "", 0},
		{"invoke error defaults to unknown", func(_ *orcaFaultRepository, g *orcaFaultGateway) { g.invokeErr = boom },
			"mutation was not repeated", "failure:" + preparationcontract.InvocationUnknown, 1},
		{"invoke error keeps typed proof", func(_ *orcaFaultRepository, g *orcaFaultGateway) {
			g.invokeErr = &preparationcontract.InvocationError{State: preparationcontract.InvocationNotInvoked, Cause: boom}
		}, "mutation was not repeated", "failure:" + preparationcontract.InvocationNotInvoked, 1},
		{"apply receipt error", func(r *orcaFaultRepository, _ *orcaFaultGateway) { r.applyErr = boom }, "boom", "failure:" + preparationcontract.InvocationUnknown, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newOrcaApplicationFixture()
			repo := &orcaFaultRepository{orcaApplicationRepositoryFake: fixture.repository}
			gw := &orcaFaultGateway{orcaApplicationGatewayFake: fixture.gateway}
			tc.setup(repo, gw)
			result, err := faultService(fixture, repo, gw, &applicationClockFake{trace: &fixture.trace}).Prepare(context.Background(), orcaCommand(true, preparationcontract.ModeOrca))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || result.OK {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !errors.Is(err, boom) && tc.name != "inventory error" {
				t.Fatalf("cause must be wrapped: %v", err)
			}
			if tc.wantFailure == "" {
				if countTracePrefix(fixture.trace, "failure:") != 0 {
					t.Fatalf("no failure record expected: %v", fixture.trace)
				}
			} else if countTracePrefix(fixture.trace, tc.wantFailure) != 1 {
				t.Fatalf("want exactly one %q in %v", tc.wantFailure, fixture.trace)
			}
			if got := countTracePrefix(fixture.trace, "invoke:"); got != tc.wantInvokes {
				t.Fatalf("invokes=%d want %d: %v", got, tc.wantInvokes, fixture.trace)
			}
		})
	}
}
