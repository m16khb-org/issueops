package issueopsreplacement

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type replacementFixture struct {
	record      model.IssueOpsRecord
	saved       *model.IssueOpsRecord
	previous    *model.NativeActor
	events      []string
	locked      bool
	absent      bool
	resealError error
	saveError   error
}

func (f *replacementFixture) Load(string) (model.IssueOpsRecord, error) {
	f.events = append(f.events, "load")
	record := f.record
	execution := *record.Execution
	record.Execution = &execution
	return record, nil
}
func (f *replacementFixture) WithinLock(_ context.Context, _ string, fn func() error) error {
	f.events = append(f.events, "lock")
	f.locked = true
	defer func() { f.locked = false; f.events = append(f.events, "unlock") }()
	return fn()
}
func (f *replacementFixture) Persist(record model.IssueOpsRecord, previous *model.NativeActor) (model.IssueOpsRecord, error) {
	if !f.locked {
		panic("write outside lock")
	}
	f.events = append(f.events, "persist")
	if f.saveError != nil {
		return model.IssueOpsRecord{}, f.saveError
	}
	f.saved = &record
	f.previous = previous
	return record, nil
}
func (f *replacementFixture) WorkspaceSnapshot(model.Workspace) (string, error) {
	f.events = append(f.events, "snapshot")
	return "snapshot", nil
}
func (f *replacementFixture) SamePath(a, b string) bool { return a == b }
func (f *replacementFixture) WorkspaceProcesses(string, map[int]bool) ([]model.ReplacementWorkspaceProcess, error) {
	f.events = append(f.events, "workspace-processes")
	return nil, nil
}
func (f *replacementFixture) Cleanup(model.IssueOpsRecord) error {
	f.events = append(f.events, "cleanup")
	return nil
}
func (f *replacementFixture) WorkspaceAbsent(string) bool {
	f.events = append(f.events, "absent")
	return f.absent
}
func (f *replacementFixture) CreateToken(model.IssueOpsRecord) (string, string, error) {
	f.events = append(f.events, "token")
	return "digest", "/workspace/claim.token", nil
}
func (f *replacementFixture) Reseal(context.Context, model.IssueOpsRecord) (model.ReplacementArtifacts, error) {
	f.events = append(f.events, "reseal")
	return model.ReplacementArtifacts{}, f.resealError
}

type deadProcesses struct{}

func (deadProcesses) Inspect(receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
	return "dead", receipt, nil
}
func (deadProcesses) AncestryPIDs(pid int) map[int]bool  { return map[int]bool{pid: true} }
func (deadProcesses) HasAncestor(int, map[int]bool) bool { return false }

func fixtureService() (Service, *replacementFixture, model.ExecutionReplaceRequest) {
	process := model.NativeProcessReceipt{PID: 42, StartedAt: "start", Executable: "/host"}
	actor := model.NativeActor{Host: "codex", SessionID: "requester", SessionProcess: &process, ProcessAncestry: []model.NativeProcessReceipt{process}}
	oldProcess := model.NativeProcessReceipt{PID: 41, StartedAt: "old", Executable: "/old-host"}
	holder := model.NativeActor{Host: "codex", SessionID: "previous", SessionProcess: &oldProcess}
	fixture := &replacementFixture{record: model.IssueOpsRecord{ID: "io-test", Execution: &model.Execution{Mode: model.ExecutionModeDirect, Workspace: model.Workspace{Root: "/workspace", SourceRoot: "/source"}, Lease: model.WriteLease{Generation: 7, Status: model.LeaseStatusActive, Holder: &holder}}}}
	service := Service{Records: fixture, Workspace: fixture, Artifacts: fixture, PID: 43, ObserveProcesses: func() port.ReplacementProcessSnapshot { return deadProcesses{} }, InspectProcess: func(p model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) { return "live", p, nil }, Now: func() string { return "now" }}
	request := model.ExecutionReplaceRequest{ID: "io-test", Action: model.ExecutionReplacePreview, Actor: actor, CWD: "/workspace", ExpectedGeneration: 7, Confirm: true, Reason: " recovery "}
	return service, fixture, request
}
func TestRevokePersistsNewGenerationWithPreviousHolderInsideLock(t *testing.T) {
	service, fixture, req := fixtureService()
	preview, err := service.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	fixture.events = nil
	req.Action = model.ExecutionReplaceRevoke
	req.InventoryFingerprint = preview.InventoryFingerprint
	result, err := service.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	lease := result.Execution.Lease
	if fixture.saved == nil || fixture.previous == nil || fixture.previous.SessionID != "previous" || lease.Generation != 8 || lease.Status != model.LeaseStatusRevoking || lease.ReplacementReason != "recovery" || lease.ReplacedAt != "now" {
		t.Fatalf("result=%+v saved=%+v previous=%+v", result, fixture.saved, fixture.previous)
	}
	if want := []string{"lock", "load", "snapshot", "persist", "unlock"}; !reflect.DeepEqual(fixture.events, want) {
		t.Fatalf("events=%v want=%v", fixture.events, want)
	}
}
func TestReplacementRefusalsNeverPersist(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*replacementFixture, *model.ExecutionReplaceRequest)
		want   string
	}{
		{"confirm", func(_ *replacementFixture, r *model.ExecutionReplaceRequest) { r.Confirm = false }, "requires confirm"},
		{"generation", func(_ *replacementFixture, r *model.ExecutionReplaceRequest) { r.ExpectedGeneration = 6 }, "stale lease generation"},
		{"zero generation", func(_ *replacementFixture, r *model.ExecutionReplaceRequest) { r.ExpectedGeneration = 0 }, "stale lease generation"},
		{"pending", func(f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Pending = &model.ExternalIntent{}
		}, "pending external intent"},
		{"cwd", func(_ *replacementFixture, r *model.ExecutionReplaceRequest) { r.CWD = "/outside" }, "cwd must"},
		{"reason", func(_ *replacementFixture, r *model.ExecutionReplaceRequest) { r.Reason = " " }, "active lease and a reason"},
		{"inventory", func(_ *replacementFixture, r *model.ExecutionReplaceRequest) { r.InventoryFingerprint = "changed" }, "stale replacement inventory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, f, req := fixtureService()
			req.Action = model.ExecutionReplaceRevoke
			tc.change(f, &req)
			result, err := service.Run(context.Background(), req)
			if err == nil || !strings.Contains(err.Error(), tc.want) || result.OK || f.saved != nil {
				t.Fatalf("result=%+v err=%v saved=%+v", result, err, f.saved)
			}
		})
	}
}
func TestFinalizeReleasesAbsentWorkspaceWithoutCreatingToken(t *testing.T) {
	service, f, req := fixtureService()
	f.record.Execution.Lease.Status = model.LeaseStatusRevoking
	f.absent = true
	req.Action = model.ExecutionReplaceFinalizePreview
	preview, err := service.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	f.events = nil
	req.Action = model.ExecutionReplaceFinalize
	req.QuiescenceFingerprint = preview.QuiescenceFingerprint
	result, err := service.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Execution.Lease.Status != model.LeaseStatusReleased || result.Execution.Lease.Holder != nil || result.Execution.Lease.ClaimTokenSHA256 != "" || result.ClaimTokenPath != "" || result.NextCommand != "" || f.saved == nil {
		t.Fatalf("result=%+v", result)
	}
	want := []string{"lock", "load", "workspace-processes", "snapshot", "cleanup", "absent", "persist", "unlock"}
	if !reflect.DeepEqual(f.events, want) {
		t.Fatalf("events=%v want=%v", f.events, want)
	}
}
func TestFinalizeDoesNotPublishClaimableAuthorityBeforeResealAndSave(t *testing.T) {
	for _, stage := range []string{"success", "reseal", "save", "stale"} {
		t.Run(stage, func(t *testing.T) {
			service, f, req := fixtureService()
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			req.Action = model.ExecutionReplaceFinalizePreview
			preview, err := service.Run(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			req.Action = model.ExecutionReplaceFinalize
			req.QuiescenceFingerprint = preview.QuiescenceFingerprint
			f.events = nil
			switch stage {
			case "reseal":
				f.resealError = errors.New("reseal failure")
			case "save":
				f.saveError = errors.New("save failure")
			case "stale":
				req.QuiescenceFingerprint = "changed"
			}
			result, err := service.Run(context.Background(), req)
			if stage == "success" {
				if err != nil || !result.OK || f.saved == nil || result.Execution.Lease.Status != model.LeaseStatusClaimable || result.Execution.Lease.ClaimTokenSHA256 != "digest" || result.Execution.Lease.Holder != nil || !strings.Contains(result.NextCommand, "--claim-current-token") {
					t.Fatalf("result=%+v err=%v", result, err)
				}
				if got := strings.Join(f.events, ","); !strings.Contains(got, "token,reseal,persist") {
					t.Fatalf("events=%s", got)
				}
			} else {
				if err == nil || result.OK || f.saved != nil || f.record.Execution.Lease.Status != model.LeaseStatusRevoking {
					t.Fatalf("failed operation published authority: result=%+v err=%v saved=%+v", result, err, f.saved)
				}
				count := 0
				for _, event := range f.events {
					if event == "cleanup" {
						count++
					}
				}
				want := 2
				if stage == "stale" {
					want = 0
				}
				if count != want {
					t.Fatalf("events=%v cleanup=%d want=%d", f.events, count, want)
				}
			}
		})
	}
}

type occupiedWorkspace struct {
	*replacementFixture
	processes []model.ReplacementWorkspaceProcess
}

func (w occupiedWorkspace) WorkspaceProcesses(string, map[int]bool) ([]model.ReplacementWorkspaceProcess, error) {
	return w.processes, nil
}

type requesterProcesses struct{ deadProcesses }

func (requesterProcesses) HasAncestor(pid int, owners map[int]bool) bool {
	return pid == 200 && owners[42]
}
func TestQuiescenceExcludesRequesterChildButRejectsOtherSession(t *testing.T) {
	service, f, req := fixtureService()
	f.record.Execution.Lease.Status = model.LeaseStatusRevoking
	req.Action = model.ExecutionReplaceFinalizePreview
	service.ObserveProcesses = func() port.ReplacementProcessSnapshot { return requesterProcesses{} }
	child := model.ReplacementWorkspaceProcess{PID: 200, Command: "requester-child"}
	external := model.ReplacementWorkspaceProcess{PID: 300, Command: "external-session"}
	service.Workspace = occupiedWorkspace{replacementFixture: f, processes: []model.ReplacementWorkspaceProcess{child}}
	result, err := service.Run(context.Background(), req)
	if err != nil || !result.OK || result.QuiescenceFingerprint == "" {
		t.Fatalf("requester's child blocked quiescence: result=%+v err=%v", result, err)
	}
	service.Workspace = occupiedWorkspace{replacementFixture: f, processes: []model.ReplacementWorkspaceProcess{child, external}}
	result, err = service.Run(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "pid=300 command=external-session") || result.OK || f.saved != nil {
		t.Fatalf("external session did not block: result=%+v err=%v", result, err)
	}
}
