package issueopspreparation

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestPrepareFailsClosedWithoutRequiredCollaborators(t *testing.T) {
	cases := []struct {
		name   string
		build  func(f *directServiceFixture) *Service
		want   string
		absent string
	}{
		{"repository", func(f *directServiceFixture) *Service {
			return NewService(nil, &applicationClockFake{trace: &f.trace}, f.direct, f.evidence)
		}, "repository is unavailable", "load"},
		{"evidence", func(f *directServiceFixture) *Service {
			return NewService(f.repository, &applicationClockFake{trace: &f.trace}, f.direct, nil)
		}, "evidence is unavailable", "root"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newDirectServiceFixture()
			result, err := tc.build(fixture).Prepare(context.Background(), directCommand(false))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
			if result.OK || result.ID != "io-prepare" {
				t.Fatalf("failed result must carry the id and not be OK: %+v", result)
			}
			if slices.Contains(fixture.trace, tc.absent) {
				t.Fatalf("trace %v must not reach %q", fixture.trace, tc.absent)
			}
		})
	}
}

func TestPrepareStopsBeforeRootClaimWhenWorkspaceEvidenceFails(t *testing.T) {
	fixture := newDirectServiceFixture()
	failing := &workspaceFailingEvidence{applicationEvidenceFake: fixture.evidence, err: errors.New("no canonical root")}
	service := NewService(fixture.repository, &applicationClockFake{trace: &fixture.trace}, fixture.direct, failing)
	result, err := service.Prepare(context.Background(), directCommand(false))
	if !errors.Is(err, failing.err) || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !slices.Equal(fixture.trace, []string{"load"}) {
		t.Fatalf("trace=%v", fixture.trace)
	}
}

type workspaceFailingEvidence struct {
	*applicationEvidenceFake
	err error
}

func (f *workspaceFailingEvidence) Workspace(preparationcontract.Snapshot, bool) (preparationcontract.WorkspaceRequest, error) {
	return preparationcontract.WorkspaceRequest{}, f.err
}

func TestNormalizeActorTrimsAndRequiresPIDReuseSafeReceipt(t *testing.T) {
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
	service := NewService(fixture.repository, &applicationClockFake{trace: &fixture.trace}, nil, fixture.evidence)
	_, err := service.Prepare(context.Background(), directCommand(false))
	if err == nil || !strings.Contains(err.Error(), "direct Git worktree provisioner is unavailable") {
		t.Fatalf("err=%v", err)
	}

	fixture = newDirectServiceFixture()
	service = NewService(fixture.repository, nil, fixture.direct, fixture.evidence)
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

func TestPrepareDirectPreviewConfirmCommandCarriesActorAndCWD(t *testing.T) {
	fixture := newDirectServiceFixture()
	command := directCommand(false)
	command.Actor.AgentID = "agent-7"
	result, err := fixture.service.Prepare(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"issueops execution prepare --id 'io-prepare'", "--host 'codex'", "--agent-id 'agent-7'", "--session-id 'session'", "--session-pid 42", "--cwd '/repo'", "--confirm --json"} {
		if !strings.Contains(result.NextCommand, want) {
			t.Fatalf("confirm command %q lacks %q", result.NextCommand, want)
		}
	}
}

func TestPrepareDirectPreviewConfirmCommandOmitsUnsetValues(t *testing.T) {
	fixture := newDirectServiceFixture()
	command := directCommand(false)
	command.CWD = ""
	result, err := fixture.service.Prepare(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--agent-id", "--cwd"} {
		if strings.Contains(result.NextCommand, flag) {
			t.Fatalf("unset value must not produce %s: %s", flag, result.NextCommand)
		}
	}
}

func TestPrepareExistingExecutionReportsRecoveryCommands(t *testing.T) {
	cases := []struct {
		name     string
		status   string
		pending  bool
		confirm  bool
		wantOK   bool
		wantErr  string
		wantNext string
	}{
		{"existing active", "active", false, true, true, "", ""},
		{"pending", "released", true, false, false, "pending external intent", "issueops execution reconcile --id io-prepare --preview ACTOR_FLAGS"},
		{"claimable", "claimable", false, true, false, "generation 1 is claimable and has no writer", "issueops execution claim --id 'io-prepare' --generation 1 --claim-current-token"},
		{"released", "released", false, true, false, "was released and has no writer", "issueops execution replace --id 'io-prepare' --expected-generation 1 --preview"},
		{"revoking", "revoking", false, true, false, "is revoking and has no writer", "issueops execution replace --id 'io-prepare' --expected-generation 1 --finalize-preview"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newDirectServiceFixture()
			fixture.repository.snapshot.Record.Execution = applicationExecution(tc.status, tc.pending)
			result, err := fixture.service.Prepare(context.Background(), directCommand(tc.confirm))
			if tc.wantErr == "" {
				if err != nil || !result.OK {
					t.Fatalf("result=%+v err=%v", result, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) || result.OK {
				t.Fatalf("result=%+v err=%v want %q", result, err, tc.wantErr)
			}
			if result.NextCommand != tc.wantNext {
				t.Fatalf("next=%q want=%q", result.NextCommand, tc.wantNext)
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
	record := leasecontract.Record{ID: "io-1", Execution: applicationExecution("active", false)}
	if got := writerlessNextCommand(preparationcontract.Snapshot{Record: record}); got != "" {
		t.Fatalf("active lease has no recovery command, got %q", got)
	}
	if err := writerlessError(record, ""); err == nil || err.Error() != "IssueOps execution has no writer" {
		t.Fatalf("err=%v", err)
	}
}

func TestPreparedResultClonesExecution(t *testing.T) {
	record := leasecontract.Record{ID: "io-1", Execution: applicationExecution("active", false)}
	result := preparedResult(record)
	if !result.OK || result.ID != "io-1" || result.Workspace.Root != record.Execution.Workspace.Root {
		t.Fatalf("result=%+v", result)
	}
	result.Execution.Workspace.Root = "mutated"
	if record.Execution.Workspace.Root == "mutated" {
		t.Fatal("result execution must be a clone")
	}
	if cloneExecution(nil) != nil {
		t.Fatal("nil execution must clone to nil")
	}
}
