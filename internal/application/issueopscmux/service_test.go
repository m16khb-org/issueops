package issueopscmux

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	audit "issueops/internal/contract/audit"
	cmux "issueops/internal/contract/cmux"
	model "issueops/internal/contract/issueops"
	port "issueops/internal/port/cmux"
)

type testDirectory struct{ closed int }

func (*testDirectory) Path() string                      { return "/workspace" }
func (*testDirectory) SamePath(string) bool              { return true }
func (d *testDirectory) Pin() (port.DirectoryPin, error) { return d, nil }
func (*testDirectory) Matches(port.Directory) bool       { return true }
func (d *testDirectory) Close()                          { d.closed++ }

type testClient struct {
	create, send       int
	createErr, sendErr error
}

func (*testClient) Preflight(context.Context, cmux.PreflightRequest) (cmux.PreflightResult, error) {
	return cmux.PreflightResult{}, nil
}
func (c *testClient) CreateWorkspace(context.Context, cmux.CreateRequest) (cmux.CreatedWorkspace, error) {
	c.create++
	return cmux.CreatedWorkspace{WorkspaceID: "workspace", SurfaceID: "surface", CWD: "/workspace"}, c.createErr
}
func (c *testClient) Send(context.Context, cmux.SendRequest) (cmux.SendReceipt, error) {
	c.send++
	return cmux.SendReceipt{Accepted: c.sendErr == nil}, c.sendErr
}

func releasedRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{ID: "cycle", WorktreePath: "/workspace", Execution: &model.Execution{Mode: model.ExecutionModeDirect, Workspace: model.Workspace{Root: "/workspace"}, Lease: model.WriteLease{Status: model.LeaseStatusReleased, Generation: 2}}}
}
func rootEnvironment(d *testDirectory) port.RootEnvironment {
	return port.RootEnvironment{
		Read:      func(string, string) (model.IssueOpsRecord, error) { return releasedRecord(), nil },
		Directory: func(string) (port.Directory, error) { return d, nil },
		Getwd:     func() (string, error) { return "/workspace", nil }, GitTop: func(s string) (string, error) { return s, nil },
	}
}
func TestHandoffFailureStopsEffectsAndPreservesRecovery(t *testing.T) {
	lost := &port.MutationError{Ambiguous: true, Cause: errors.New("response lost")}
	for _, tc := range []struct {
		name                                         string
		stageFail                                    bool
		createErr, sendErr, receiptErr, cleanupErr   error
		wantStatus                                   string
		wantCreate, wantSend, wantAwait, wantCleanup int
		wantOK, wantError                            bool
	}{
		{name: "stage audit fails", stageFail: true, wantStatus: "rejected", wantError: true},
		{name: "create response lost", createErr: lost, wantStatus: "ambiguous", wantCreate: 1, wantError: true},
		{name: "send response lost", sendErr: lost, wantStatus: "ambiguous", wantCreate: 1, wantSend: 1, wantError: true},
		{name: "receiver unverified", receiptErr: errors.New("no receipt"), wantStatus: "input_accepted_receiver_unverified", wantCreate: 1, wantSend: 1, wantAwait: 1, wantOK: true},
		{name: "cleanup failed", cleanupErr: errors.New("cleanup denied"), wantStatus: "input_accepted_cleanup_failed", wantCreate: 1, wantSend: 1, wantAwait: 1, wantCleanup: 1, wantOK: true, wantError: true},
		{name: "receiver correlated", wantStatus: "input_accepted", wantCreate: 1, wantSend: 1, wantAwait: 1, wantCleanup: 1, wantOK: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := &testDirectory{}
			client := &testClient{createErr: tc.createErr, sendErr: tc.sendErr}
			awaited, cleaned := 0, 0
			var observations []model.IssueOpsHandoffDeliveryObservation
			service := Service{Client: client, Roots: rootEnvironment(dir), Now: func() time.Time { return time.Unix(0, 0) }, ArtifactRoot: "/artifacts",
				ReadAudit: func() ([]model.IssueOpsHandoffDeliveryObservation, error) { return nil, nil },
				Observe: func(_ model.IssueOpsRecord, o model.IssueOpsHandoffDeliveryObservation) (audit.HandoffDeliveryAuditRecord, error) {
					if tc.stageFail {
						return audit.HandoffDeliveryAuditRecord{}, errors.New("audit failed")
					}
					observations = append(observations, o)
					return audit.HandoffDeliveryAuditRecord{Observation: o}, nil
				},
				ReadPrompt: func(string, string, string) ([]byte, error) { return []byte("prompt"), nil }, SamePath: func(string, string) bool { return true }, ValidateHostExecutable: func(string) error { return nil }, ValidateHostProfile: func(string, string, string, string) error { return nil },
				Prepare: func(cmux.ArtifactRequest) (cmux.PreparedLauncher, error) {
					return cmux.PreparedLauncher{Directory: "/recovery", Command: "launch"}, nil
				},
				AwaitReceipt: func(context.Context, cmux.PreparedLauncher, cmux.BootstrapExpectation) (model.NativeProcessReceipt, error) {
					awaited++
					return model.NativeProcessReceipt{PID: 42, Executable: "/codex"}, tc.receiptErr
				},
				Cleanup: func(cmux.PreparedLauncher) error { cleaned++; return tc.cleanupErr },
			}
			result, err := service.Handoff(context.Background(), "state", model.ExecutionCmuxHandoffRequest{ID: "cycle", Generation: 2, CWD: "/workspace", PromptFile: "/workspace/prompt", MaterialSHA256: strings.Repeat("b", 64)})
			if result.Status != tc.wantStatus || result.OK != tc.wantOK || (err != nil) != tc.wantError || client.create != tc.wantCreate || client.send != tc.wantSend || awaited != tc.wantAwait || cleaned != tc.wantCleanup || dir.closed != 1 {
				t.Fatalf("result=%+v err=%v effects=%d,%d,%d,%d closed=%d", result, err, client.create, client.send, awaited, cleaned, dir.closed)
			}
			if result.NativeTurnObserved || result.OwnerClaimed {
				t.Fatalf("transport promoted authority: %+v", result)
			}
			if tc.wantSend == 1 && (tc.sendErr != nil || tc.receiptErr != nil || tc.cleanupErr != nil) && result.RecoveryArtifactDir != "/recovery" {
				t.Fatalf("lost recovery: %+v", result)
			}
			if tc.createErr != nil || tc.sendErr != nil {
				last := observations[len(observations)-1]
				if last.Ambiguous.Status != model.IssueOpsHandoffDeliveryStateObserved || last.InputAccepted.Status == model.IssueOpsHandoffDeliveryStateObserved {
					t.Fatalf("ambiguous audit=%+v", last)
				}
			}
		})
	}
}
func TestRootFenceClosesPinAfterFailedSecondRead(t *testing.T) {
	dir := &testDirectory{}
	env := rootEnvironment(dir)
	reads := 0
	failed := errors.New("read changed")
	env.Read = func(string, string) (model.IssueOpsRecord, error) {
		reads++
		if reads == 2 {
			return model.IssueOpsRecord{}, failed
		}
		return releasedRecord(), nil
	}
	_, _, err := openRootFence(env, "state", model.ExecutionCmuxHandoffRequest{Generation: 2})
	if !errors.Is(err, failed) || dir.closed != 1 || reads != 2 {
		t.Fatalf("err=%v closed=%d reads=%d", err, dir.closed, reads)
	}
}
func TestRootFenceRejectsGenerationBeforeDirectoryObservation(t *testing.T) {
	env := rootEnvironment(&testDirectory{})
	env.Directory = func(string) (port.Directory, error) {
		t.Fatal("observed directory before generation check")
		return nil, nil
	}
	if _, _, err := openRootFence(env, "state", model.ExecutionCmuxHandoffRequest{Generation: 3}); err == nil {
		t.Fatal("wrong generation accepted")
	}
}
