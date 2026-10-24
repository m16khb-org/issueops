package issueopspreparation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestDirectPreparationPreviewAndConfirmTrace(t *testing.T) {
	tests := []struct {
		name      string
		confirm   bool
		wantTrace []string
	}{
		{name: "preview", wantTrace: []string{"load", "workspace", "root", "direct.prepare", "clock"}},
		{name: "confirm", confirm: true, wantTrace: []string{"load", "workspace", "root", "direct.access", "direct.prepare", "clock", "clock", "evidence.materialize", "commit"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newDirectServiceFixture()
			result, err := fixture.service.Prepare(context.Background(), directCommand(test.confirm))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(fixture.trace, test.wantTrace) {
				t.Fatalf("trace=%v want=%v", fixture.trace, test.wantTrace)
			}
			if !result.OK || result.Preview != !test.confirm {
				t.Fatalf("result=%+v", result)
			}
			if test.confirm {
				if fixture.repository.commit == nil || fixture.repository.commit.LinkedAt == fixture.repository.commit.ClaimedAt || result.Execution == nil {
					t.Fatalf("commit=%+v result=%+v", fixture.repository.commit, result)
				}
			} else if fixture.repository.commit != nil {
				t.Fatalf("preview committed %+v", fixture.repository.commit)
			}
		})
	}
}

func TestNormalizeActorAcceptsOmo(t *testing.T) {
	actor, err := normalizeActor(leasecontract.Actor{
		Host:      " OMO ",
		SessionID: " session ",
		SessionProcess: &leasecontract.ProcessReceipt{
			PID: 42, StartedAt: " 2026-08-12T00:00:00Z ", Executable: " omo ",
		},
	})
	if err != nil {
		t.Fatalf("Omo preparation actor must normalize: %v", err)
	}
	if actor.Host != "omo" || actor.SessionID != "session" || actor.SessionProcess == nil ||
		actor.SessionProcess.StartedAt != "2026-08-12T00:00:00Z" || actor.SessionProcess.Executable != "omo" {
		t.Fatalf("normalized Omo actor=%+v", actor)
	}
}

func TestDirectPreparationFailureStopsBeforeLaterEffects(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*directServiceFixture)
		wantTrace []string
	}{
		{name: "access denial", configure: func(f *directServiceFixture) {
			f.direct.access = preparationcontract.AccessResult{Allowed: false, RelaunchCommand: "codex --add-dir /repo.worktrees/199"}
		}, wantTrace: []string{"load", "workspace", "root", "direct.access"}},
		{name: "provision failure", configure: func(f *directServiceFixture) {
			f.direct.prepareErr = errors.New("provision failed")
		}, wantTrace: []string{"load", "workspace", "root", "direct.access", "direct.prepare"}},
		{name: "artifact failure", configure: func(f *directServiceFixture) {
			f.evidence.materializeErr = errors.New("artifact failed")
		}, wantTrace: []string{"load", "workspace", "root", "direct.access", "direct.prepare", "clock", "clock", "evidence.materialize"}},
		{name: "persistence failure", configure: func(f *directServiceFixture) {
			f.repository.commitErr = errors.New("persist failed")
		}, wantTrace: []string{"load", "workspace", "root", "direct.access", "direct.prepare", "clock", "clock", "evidence.materialize", "commit"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newDirectServiceFixture()
			test.configure(fixture)
			result, err := fixture.service.Prepare(context.Background(), directCommand(true))
			if err == nil || result.OK {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !reflect.DeepEqual(fixture.trace, test.wantTrace) {
				t.Fatalf("trace=%v want=%v", fixture.trace, test.wantTrace)
			}
		})
	}
}

func TestDirectPreparationRejectsIssueURLWithoutNumberBeforeProvisioning(t *testing.T) {
	fixture := newDirectServiceFixture()
	fixture.repository.snapshot.ArtifactDir = ""
	result, err := fixture.service.Prepare(context.Background(), directCommand(true))
	if err == nil || result.OK || !strings.Contains(err.Error(), "no issue number") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if traceIndex(fixture.trace, "direct.prepare") >= 0 || fixture.repository.commit != nil {
		t.Fatalf("issue URL without number reached provisioning: %v", fixture.trace)
	}
}

// An agent holding the parent's lease prepared a child: the preview passed and
// the confirm failed at commit with a worktree already provisioned.
func TestDirectPreparationRefusesAnActorHoldingAnotherLeaseBeforeProvisioning(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		fixture := newDirectServiceFixture()
		fixture.repository.holderErr = errors.New("this session already holds the execution lease of io-parent")
		result, err := fixture.service.Prepare(context.Background(), directCommand(confirm))
		if err == nil || result.OK || !strings.Contains(err.Error(), "io-parent") {
			t.Fatalf("confirm=%v result=%+v err=%v", confirm, result, err)
		}
		if traceIndex(fixture.trace, "direct.prepare") >= 0 || fixture.repository.commit != nil {
			t.Fatalf("confirm=%v reached provisioning: %v", confirm, fixture.trace)
		}
	}
}

func TestPreparationDecisionShortCircuitsDirectEffects(t *testing.T) {
	tests := []struct {
		name      string
		execution *leasecontract.Execution
		rootErr   error
		wantTrace []string
		wantCode  string
		wantNext  bool
	}{
		{name: "idempotent", execution: applicationExecution("active", false), wantTrace: []string{"load"}, wantCode: "existing"},
		{name: "pending", execution: applicationExecution("released", true), wantTrace: []string{"load"}, wantCode: "pending_reconcile", wantNext: true},
		{name: "writerless", execution: applicationExecution("released", false), wantTrace: []string{"load"}, wantCode: "writerless", wantNext: true},
		{name: "root collision", rootErr: errors.New("canonical root belongs to io-other"), wantTrace: []string{"load", "workspace", "root"}, wantCode: "root_conflict"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newDirectServiceFixture()
			fixture.repository.snapshot.Record.Execution = test.execution
			fixture.repository.rootErr = test.rootErr
			result, err := fixture.service.Prepare(context.Background(), directCommand(true))
			if test.wantCode == "existing" {
				if err != nil || !result.OK {
					t.Fatalf("result=%+v err=%v", result, err)
				}
			} else if err == nil || result.OK {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !reflect.DeepEqual(fixture.trace, test.wantTrace) {
				t.Fatalf("trace=%v result=%+v", fixture.trace, result)
			}
			if test.wantNext && result.NextCommand == "" {
				t.Fatal("recovery decision omitted next_command")
			}
		})
	}
}

type directServiceFixture struct {
	trace      []string
	repository *applicationRepositoryFake
	direct     *applicationDirectFake
	evidence   *applicationEvidenceFake
	service    *Service
}

func newDirectServiceFixture() *directServiceFixture {
	fixture := &directServiceFixture{}
	record := leasecontract.Record{ID: "io-prepare", Repo: "/repo", Branch: "199-prepare"}
	fixture.repository = &applicationRepositoryFake{trace: &fixture.trace, snapshot: preparationcontract.Snapshot{Record: record, RecordRaw: []byte("raw"), ArtifactDir: ".issueops/issues/199/artifact"}}
	fixture.direct = &applicationDirectFake{trace: &fixture.trace, access: preparationcontract.AccessResult{Allowed: true}, receipt: preparationcontract.WorkspaceReceipt{SourceRoot: "/repo", Root: "/repo.worktrees/199-prepare", Branch: "199-prepare", BaseHead: "base", Exists: true}}
	fixture.evidence = &applicationEvidenceFake{trace: &fixture.trace, workspace: preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: record.Repo, Root: "/repo.worktrees/199-prepare", Branch: record.Branch, BaseBranch: "117-parent", BaseHead: "base", Confirm: true}}
	fixture.service = NewService(fixture.repository, &applicationClockFake{trace: &fixture.trace}, fixture.direct, fixture.evidence)
	return fixture
}

func directCommand(confirm bool) preparationcontract.Command {
	return preparationcontract.Command{
		ID: "io-prepare", CWD: "/repo", Confirm: confirm,
		Actor: leasecontract.Actor{Host: "codex", SessionID: "session", SessionProcess: &leasecontract.ProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/codex"}},
	}
}

func applicationExecution(status string, pending bool) *leasecontract.Execution {
	execution := &leasecontract.Execution{Workspace: leasecontract.Workspace{SourceRoot: "/repo", Root: "/repo.worktrees/199-prepare", Branch: "199-prepare", BaseHead: "base", LinkedAt: "linked"}, Lease: leasecontract.Lease{Generation: 1, Status: status}}
	if status == "active" {
		execution.Lease.Holder = &leasecontract.Actor{Host: "codex", SessionID: "session", SessionProcess: &leasecontract.ProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/codex"}}
	}
	if pending {
		execution.Pending = &leasecontract.ExternalIntent{OperationID: "operation", Kind: "remote_pr_create", Marker: "marker", StartedAt: "started"}
	}
	return execution
}

type applicationRepositoryFake struct {
	trace     *[]string
	snapshot  preparationcontract.Snapshot
	rootErr   error
	holderErr error
	commitErr error
	commit    *DirectCommit
}

func (fake *applicationRepositoryFake) Load(context.Context, string) (preparationcontract.Snapshot, error) {
	*fake.trace = append(*fake.trace, "load")
	return fake.snapshot.Clone(), nil
}

func (fake *applicationRepositoryFake) EnsureRootUnclaimed(context.Context, string, string) error {
	*fake.trace = append(*fake.trace, "root")
	return fake.rootErr
}

func (fake *applicationRepositoryFake) EnsureActorHoldsNoLease(context.Context, leasecontract.Actor) error {
	return fake.holderErr
}

func (fake *applicationRepositoryFake) CommitDirect(_ context.Context, commit DirectCommit) (preparationcontract.Result, error) {
	*fake.trace = append(*fake.trace, "commit")
	fake.commit = &commit
	if fake.commitErr != nil {
		return preparationcontract.Result{ID: commit.Command.ID}, fake.commitErr
	}
	execution := &leasecontract.Execution{Workspace: leasecontract.Workspace{SourceRoot: commit.Workspace.SourceRoot, Root: commit.Workspace.Root, Branch: commit.Workspace.Branch, BaseHead: commit.Workspace.BaseHead, ParentWorktree: commit.Workspace.ParentWorktree, LinkedAt: commit.LinkedAt}, Lease: leasecontract.Lease{Generation: 1, Status: "active", Holder: &commit.Command.Actor, ClaimedAt: commit.ClaimedAt}}
	return preparationcontract.Result{OK: true, ID: commit.Command.ID, Workspace: execution.Workspace, Execution: execution}, nil
}

type applicationClockFake struct {
	trace *[]string
	next  int
}

func (fake *applicationClockFake) Now() time.Time {
	*fake.trace = append(*fake.trace, "clock")
	fake.next++
	return time.Date(2026, time.August, 2, 2, fake.next, 0, 0, time.UTC)
}

type applicationDirectFake struct {
	trace      *[]string
	access     preparationcontract.AccessResult
	accessErr  error
	receipt    preparationcontract.WorkspaceReceipt
	prepareErr error
}

func (fake *applicationDirectFake) ProbeAccess(context.Context, preparationcontract.WorkspaceRequest, string) (preparationcontract.AccessResult, error) {
	*fake.trace = append(*fake.trace, "direct.access")
	return fake.access, fake.accessErr
}

func (fake *applicationDirectFake) Prepare(context.Context, preparationcontract.WorkspaceRequest) (preparationcontract.WorkspaceReceipt, error) {
	*fake.trace = append(*fake.trace, "direct.prepare")
	return fake.receipt, fake.prepareErr
}

type applicationEvidenceFake struct {
	trace          *[]string
	workspace      preparationcontract.WorkspaceRequest
	workspaceErr   error
	materializeErr error
}

func (fake *applicationEvidenceFake) Workspace(_ preparationcontract.Snapshot, confirm bool) (preparationcontract.WorkspaceRequest, error) {
	*fake.trace = append(*fake.trace, "workspace")
	result := fake.workspace
	result.Confirm = confirm
	return result, fake.workspaceErr
}
func (fake *applicationEvidenceFake) MaterializeDirect(context.Context, preparationcontract.Snapshot, preparationcontract.WorkspaceReceipt) error {
	*fake.trace = append(*fake.trace, "evidence.materialize")
	return fake.materializeErr
}

func traceIndex(trace []string, name string) int {
	for index, entry := range trace {
		if entry == name {
			return index
		}
	}
	return -1
}
