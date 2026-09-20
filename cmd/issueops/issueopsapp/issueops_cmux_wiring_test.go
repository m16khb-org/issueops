package issueopsapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	auditadapter "issueops/internal/adapter/audit"
	cmuxadapter "issueops/internal/adapter/cmux"
	issueopscontract "issueops/internal/contract/issueops"
)

const (
	cmuxPath      = "/Applications/cmux.app/Contents/Resources/bin/cmux"
	socketPath    = "/private/tmp/cmux-test.sock"
	testWindow    = "11111111-1111-4111-8111-111111111111"
	testWorkspace = "22222222-2222-4222-8222-222222222222"
	testPane      = "33333333-3333-4333-8333-333333333333"
	testSurface   = "44444444-4444-4444-8444-444444444444"
)

func TestCmuxHandoffStagesBeforeCreateAndEnrichesOneNonAuthoritativeLineage(t *testing.T) {
	stateRoot := t.TempDir()
	record := seedReleasedDirectHandoffRecord(t, stateRoot)
	promptPath, promptDigest := writeCmuxPromptFixture(t, record.WorktreePath)
	clock := cmuxTestClock(time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC))
	process := issueopscontract.NativeProcessReceipt{PID: 8080, StartedAt: "2026-09-20T10:00:03Z", Executable: "/opt/native/codex"}
	fake := &cmuxClientFake{t: t, stateRoot: stateRoot, recordID: record.ID, preflight: cmuxPreflightFixture(), created: cmuxCreatedFixture()}
	fake.created.CWD = record.WorktreePath
	artifactDir := filepath.Join(t.TempDir(), "artifact")
	if err := os.Mkdir(artifactDir, 0o700); err != nil {
		t.Fatal(err)
	}
	request := cmuxHandoffRequestFixture(record.ID, promptPath, promptDigest)
	result, err := issueOpsCmuxHandoffHandlerWithDeps(context.Background(), stateRoot, request, cmuxHandoffDependencies{
		Client: fake, Now: clock,
		GitTop:                 func(root string) (string, error) { return root, nil },
		ValidateHostExecutable: func(string) error { return nil },
		Prepare: func(cmuxadapter.ArtifactRequest) (cmuxadapter.PreparedLauncher, error) {
			return cmuxadapter.PreparedLauncher{Directory: artifactDir, LauncherPath: filepath.Join(artifactDir, "launch.sh"), Command: "/bin/sh '/tmp/launch.sh'"}, nil
		},
		AwaitReceipt: func(context.Context, cmuxadapter.PreparedLauncher, cmuxadapter.BootstrapExpectation) (issueopscontract.NativeProcessReceipt, error) {
			return process, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Status != "input_accepted" || !result.InputAccepted || result.NativeTurnObserved || result.OwnerClaimed || result.Target.Process == nil {
		t.Fatalf("result=%+v", result)
	}
	if fake.createCalls != 1 || fake.sendCalls != 1 {
		t.Fatalf("create=%d send=%d", fake.createCalls, fake.sendCalls)
	}
	observations, err := auditadapter.ReadHandoffDeliveryAuditObservationsAt(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 4 {
		t.Fatalf("observation count=%d observations=%+v", len(observations), observations)
	}
	staged, enriched, accepted, correlated := observations[0], observations[1], observations[2], observations[3]
	if staged.CallStaged.Status != "observed" || staged.Target.WindowID == "" || staged.Target.CWD != record.WorktreePath || staged.Target.WorkspaceID != "" || staged.Target.SurfaceID != "" {
		t.Fatalf("invalid pre-create stage: %+v", staged)
	}
	if staged.LineageID != enriched.LineageID || staged.LineageID != accepted.LineageID || staged.LineageID != correlated.LineageID || enriched.Target.WorkspaceID != testWorkspace || enriched.Target.SurfaceID != testSurface {
		t.Fatalf("lineage enrichment diverged: %+v %+v %+v", staged, enriched, accepted)
	}
	if accepted.InputAccepted.Evidence != issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput || accepted.NativeTurnObserved.Status != "not_observed" || accepted.OwnerClaimed.Status != "not_observed" {
		t.Fatalf("raw input promoted authority: %+v", accepted)
	}
	if correlated.Target.Process == nil || correlated.Target.Process.PID != process.PID || correlated.NativeTurnObserved.Status != "not_observed" || correlated.OwnerClaimed.Status != "not_observed" {
		t.Fatalf("receiver correlation promoted authority: %+v", correlated)
	}
}

func TestCmuxHandoffDuplicateAttemptFailsBeforeAnyCmuxCall(t *testing.T) {
	stateRoot := t.TempDir()
	record := seedReleasedDirectHandoffRecord(t, stateRoot)
	promptPath, promptDigest := writeCmuxPromptFixture(t, record.WorktreePath)
	request := cmuxHandoffRequestFixture(record.ID, promptPath, promptDigest)
	observation := manualCmuxHandoffObservation(record.ID, 1)
	observation.AttemptID, observation.LineageID = cmuxHandoffIDs(request)
	observation.Target.CWD = record.WorktreePath
	observation.Target.WindowID = request.WindowID
	observation.Launcher.EndpointIncarnation = cmuxEndpointFixture()
	if _, err := auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, observation); err != nil {
		t.Fatal(err)
	}
	fake := &cmuxClientFake{t: t, preflight: cmuxPreflightFixture()}
	_, err := issueOpsCmuxHandoffHandlerWithDeps(context.Background(), stateRoot, request, cmuxHandoffDependencies{Client: fake, Now: time.Now, GitTop: func(root string) (string, error) { return root, nil }, ValidateHostExecutable: func(string) error { return nil }})
	if err == nil || !strings.Contains(err.Error(), "already exists") || fake.preflightCalls != 0 || fake.createCalls != 0 || fake.sendCalls != 0 {
		t.Fatalf("duplicate error=%v calls=%d/%d/%d", err, fake.preflightCalls, fake.createCalls, fake.sendCalls)
	}
}

func TestCmuxHandoffRejectsPromptThroughSymlinkedParentBeforePreflight(t *testing.T) {
	stateRoot := t.TempDir()
	record := seedReleasedDirectHandoffRecord(t, stateRoot)
	external := t.TempDir()
	value := []byte("outside canonical worktree\n")
	if err := os.WriteFile(filepath.Join(external, "prompt"), value, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(record.WorktreePath, "linked")
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}
	fake := &cmuxClientFake{t: t, preflight: cmuxPreflightFixture()}
	request := cmuxHandoffRequestFixture(record.ID, filepath.Join(link, "prompt"), digestBytes(value))
	_, err := issueOpsCmuxHandoffHandlerWithDeps(context.Background(), stateRoot, request, cmuxHandoffDependencies{
		Client: fake, Now: time.Now,
		GitTop:                 func(root string) (string, error) { return root, nil },
		ValidateHostExecutable: func(string) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "inside the canonical worktree") {
		t.Fatalf("symlinked parent error=%v", err)
	}
	if fake.preflightCalls != 0 || fake.createCalls != 0 || fake.sendCalls != 0 {
		t.Fatalf("unsafe prompt reached cmux: preflight=%d create=%d send=%d", fake.preflightCalls, fake.createCalls, fake.sendCalls)
	}
}

func TestCmuxHandoffCreateAndSendAmbiguityNeverRetries(t *testing.T) {
	for _, phase := range []string{"workspace_create", "target_resolve", "input_send"} {
		t.Run(phase, func(t *testing.T) {
			stateRoot := t.TempDir()
			record := seedReleasedDirectHandoffRecord(t, stateRoot)
			promptPath, promptDigest := writeCmuxPromptFixture(t, record.WorktreePath)
			fake := &cmuxClientFake{t: t, stateRoot: stateRoot, recordID: record.ID, preflight: cmuxPreflightFixture(), created: cmuxCreatedFixture()}
			fake.created.CWD = record.WorktreePath
			mutation := &cmuxadapter.MutationError{Phase: phase, Ambiguous: true, Cause: errors.New("response lost")}
			if phase == "workspace_create" || phase == "target_resolve" {
				fake.createErr = mutation
			} else {
				fake.sendErr = mutation
			}
			artifactDir := filepath.Join(t.TempDir(), "recovery")
			if err := os.Mkdir(artifactDir, 0o700); err != nil {
				t.Fatal(err)
			}
			_, err := issueOpsCmuxHandoffHandlerWithDeps(context.Background(), stateRoot, cmuxHandoffRequestFixture(record.ID, promptPath, promptDigest), cmuxHandoffDependencies{
				Client: fake, Now: cmuxTestClock(time.Now().UTC()), GitTop: func(root string) (string, error) { return root, nil }, ValidateHostExecutable: func(string) error { return nil },
				Prepare: func(cmuxadapter.ArtifactRequest) (cmuxadapter.PreparedLauncher, error) {
					return cmuxadapter.PreparedLauncher{Directory: artifactDir, Command: "/bin/sh '/tmp/launch.sh'"}, nil
				},
			})
			if err == nil || fake.createCalls != 1 || fake.sendCalls > 1 {
				t.Fatalf("error=%v create=%d send=%d", err, fake.createCalls, fake.sendCalls)
			}
			observations, readErr := auditadapter.ReadHandoffDeliveryAuditObservationsAt(stateRoot)
			if readErr != nil {
				t.Fatal(readErr)
			}
			last := observations[len(observations)-1]
			if last.Ambiguous.Status != "observed" || last.Ambiguous.Evidence != issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost {
				t.Fatalf("ambiguous observation=%+v", last)
			}
			if phase == "input_send" {
				if _, statErr := os.Stat(artifactDir); statErr != nil {
					t.Fatalf("send ambiguity removed recovery artifact: %v", statErr)
				}
			}
		})
	}
}

func TestCmuxHandoffPostCreateFailuresStayInLineageAndPreserveRecovery(t *testing.T) {
	for _, test := range []struct {
		name, evidence string
		prepareErr     error
		receiptErr     error
	}{
		{name: "artifact preparation", evidence: issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost, prepareErr: errors.New("artifact boundary unsafe")},
		{name: "bootstrap receipt", evidence: issueopscontract.IssueOpsHandoffDeliveryEvidenceTimeout, receiptErr: errors.New("receiver cwd mismatch")},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			record := seedReleasedDirectHandoffRecord(t, stateRoot)
			promptPath, promptDigest := writeCmuxPromptFixture(t, record.WorktreePath)
			fake := &cmuxClientFake{t: t, stateRoot: stateRoot, recordID: record.ID, preflight: cmuxPreflightFixture(), created: cmuxCreatedFixture()}
			fake.created.CWD = record.WorktreePath
			artifactDir := filepath.Join(t.TempDir(), "recovery")
			if err := os.Mkdir(artifactDir, 0o700); err != nil {
				t.Fatal(err)
			}
			_, err := issueOpsCmuxHandoffHandlerWithDeps(context.Background(), stateRoot, cmuxHandoffRequestFixture(record.ID, promptPath, promptDigest), cmuxHandoffDependencies{
				Client: fake, Now: cmuxTestClock(time.Now().UTC()), GitTop: func(root string) (string, error) { return root, nil }, ValidateHostExecutable: func(string) error { return nil },
				Prepare: func(cmuxadapter.ArtifactRequest) (cmuxadapter.PreparedLauncher, error) {
					if test.prepareErr != nil {
						return cmuxadapter.PreparedLauncher{}, test.prepareErr
					}
					return cmuxadapter.PreparedLauncher{Directory: artifactDir, Command: "/bin/sh '/tmp/launch.sh'"}, nil
				},
				AwaitReceipt: func(context.Context, cmuxadapter.PreparedLauncher, cmuxadapter.BootstrapExpectation) (issueopscontract.NativeProcessReceipt, error) {
					return issueopscontract.NativeProcessReceipt{}, test.receiptErr
				},
			})
			if err == nil || fake.createCalls != 1 || fake.sendCalls > 1 {
				t.Fatalf("error=%v create=%d send=%d", err, fake.createCalls, fake.sendCalls)
			}
			observations, readErr := auditadapter.ReadHandoffDeliveryAuditObservationsAt(stateRoot)
			if readErr != nil {
				t.Fatal(readErr)
			}
			last := observations[len(observations)-1]
			if last.Ambiguous.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved || last.Ambiguous.Evidence != test.evidence || last.Target.WorkspaceID != testWorkspace {
				t.Fatalf("terminal observation=%+v", last)
			}
			for _, observation := range observations {
				if observation.LineageID != observations[0].LineageID {
					t.Fatalf("failure escaped lineage: %+v", observations)
				}
			}
			if test.receiptErr != nil {
				if _, statErr := os.Stat(artifactDir); statErr != nil {
					t.Fatalf("bootstrap failure removed recovery artifact: %v", statErr)
				}
			}
		})
	}
}

type cmuxClientFake struct {
	t                                      *testing.T
	stateRoot, recordID                    string
	preflight                              cmuxadapter.PreflightResult
	created                                cmuxadapter.CreatedWorkspace
	preflightCalls, createCalls, sendCalls int
	createErr, sendErr                     error
}

func (fake *cmuxClientFake) Preflight(context.Context, cmuxadapter.PreflightRequest) (cmuxadapter.PreflightResult, error) {
	fake.preflightCalls++
	return fake.preflight, nil
}

func (fake *cmuxClientFake) CreateWorkspace(context.Context, cmuxadapter.CreateRequest) (cmuxadapter.CreatedWorkspace, error) {
	fake.createCalls++
	if fake.stateRoot != "" {
		observations, err := auditadapter.ReadHandoffDeliveryAuditObservationsAt(fake.stateRoot)
		if err != nil || len(observations) != 1 || observations[0].CallStaged.Status != "observed" || observations[0].Target.WorkspaceID != "" {
			fake.t.Fatalf("workspace created before stage audit: observations=%+v err=%v", observations, err)
		}
	}
	return fake.created, fake.createErr
}

func (fake *cmuxClientFake) Send(context.Context, cmuxadapter.SendRequest) (cmuxadapter.SendReceipt, error) {
	fake.sendCalls++
	if fake.stateRoot != "" {
		observations, err := auditadapter.ReadHandoffDeliveryAuditObservationsAt(fake.stateRoot)
		if err != nil || len(observations) != 2 || observations[1].Target.WorkspaceID == "" {
			fake.t.Fatalf("input sent before target enrichment audit: observations=%+v err=%v", observations, err)
		}
	}
	return cmuxadapter.SendReceipt{Accepted: fake.sendErr == nil, SendMS: 6}, fake.sendErr
}

func cmuxHandoffRequestFixture(id, promptPath, promptDigest string) issueopscontract.ExecutionCmuxHandoffRequest {
	return issueopscontract.ExecutionCmuxHandoffRequest{
		ID: id, Generation: 1, CmuxExecutable: cmuxPath, CmuxVersion: "0.64.10", SocketPath: socketPath,
		WindowID: testWindow, Host: "codex", HostExecutable: "/opt/native/codex", Model: "gpt-5.6-terra", Effort: "high",
		PromptFile: promptPath, PromptSHA256: promptDigest, MaterialSHA256: strings.Repeat("b", 64),
	}
}

func writeCmuxPromptFixture(t *testing.T, root string) (string, string) {
	t.Helper()
	path := filepath.Join(root, "cmux-prompt")
	value := []byte("sealed cmux prompt\n")
	if err := os.WriteFile(path, value, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, digestBytes(value)
}

func cmuxPreflightFixture() cmuxadapter.PreflightResult {
	return cmuxadapter.PreflightResult{Executable: cmuxPath, Version: "0.64.10", SocketPath: socketPath, WindowID: testWindow, Endpoint: *cmuxEndpointFixture()}
}

func cmuxCreatedFixture() cmuxadapter.CreatedWorkspace {
	return cmuxadapter.CreatedWorkspace{Preflight: cmuxPreflightFixture(), WindowID: testWindow, WorkspaceID: testWorkspace, PaneID: testPane, SurfaceID: testSurface, CWD: "/placeholder", CreateMS: 31, ResolveMS: 8}
}

func cmuxEndpointFixture() *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation {
	return &issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation{
		Path: socketPath, Kind: "unix_socket", Device: 1, Inode: 2, CTimeNS: 3,
		OwnerUID: 501, OwnerGID: 20, Mode: 0o600, ParentPath: "/private/tmp", ParentDevice: 1,
		ParentInode: 4, ParentOwnerUID: 0, ParentOwnerGID: 0, ParentMode: 0o1777,
	}
}

func cmuxTestClock(start time.Time) func() time.Time {
	next := start
	return func() time.Time {
		current := next
		next = next.Add(time.Second)
		return current
	}
}

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
