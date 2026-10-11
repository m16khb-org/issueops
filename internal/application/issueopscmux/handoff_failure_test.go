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
	issueopsdomain "issueops/internal/domain/issueops"
	port "issueops/internal/port/cmux"
)

// fakeDir is a configurable canonical-worktree identity.
type fakeDir struct {
	path     string
	sameAs   func(string) bool
	pinErr   error
	notMatch bool
	closed   int
}

func (d *fakeDir) Path() string { return d.path }
func (d *fakeDir) SamePath(other string) bool {
	if d.sameAs != nil {
		return d.sameAs(other)
	}
	return other == d.path
}
func (d *fakeDir) Pin() (port.DirectoryPin, error) {
	if d.pinErr != nil {
		return nil, d.pinErr
	}
	return d, nil
}
func (d *fakeDir) Matches(port.Directory) bool { return !d.notMatch }
func (d *fakeDir) Close()                      { d.closed++ }

func fenceEnv(dir *fakeDir) port.RootEnvironment {
	return port.RootEnvironment{
		Read:      func(string, string) (model.IssueOpsRecord, error) { return releasedRecord(), nil },
		Directory: func(string) (port.Directory, error) { return dir, nil },
		Getwd:     func() (string, error) { return "/workspace", nil },
		GitTop:    func(path string) (string, error) { return path, nil },
	}
}

func fenceRequest() model.ExecutionCmuxHandoffRequest {
	return model.ExecutionCmuxHandoffRequest{ID: "cycle", Generation: 2, CWD: "/workspace"}
}

func TestOpenRootFenceRejectsEachIdentityMismatchWithItsOwnReason(t *testing.T) {
	readFailure := errors.New("record unreadable")
	for _, tc := range []struct {
		name    string
		mutate  func(env *port.RootEnvironment, dir *fakeDir)
		request func(r *model.ExecutionCmuxHandoffRequest)
		want    string
		wantIs  error
	}{
		{
			name: "record read fails",
			mutate: func(env *port.RootEnvironment, _ *fakeDir) {
				env.Read = func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, readFailure }
			},
			wantIs: readFailure,
		},
		{
			name: "execution is not a released direct lease",
			mutate: func(env *port.RootEnvironment, _ *fakeDir) {
				env.Read = func(string, string) (model.IssueOpsRecord, error) {
					record := releasedRecord()
					record.Execution.Lease.Status = model.LeaseStatusActive
					return record, nil
				}
			},
			want: "exact released direct execution generation",
		},
		{
			name: "workspace directory cannot be observed",
			mutate: func(env *port.RootEnvironment, _ *fakeDir) {
				env.Directory = func(string) (port.Directory, error) { return nil, errors.New("gone") }
			},
			want: "canonical worktree identity mismatch",
		},
		{
			name: "durable worktree path is another directory",
			mutate: func(_ *port.RootEnvironment, dir *fakeDir) {
				dir.sameAs = func(other string) bool { return other != "/workspace" }
			},
			want: "durable worktree is not the canonical worktree",
		},
		{
			name:    "request cwd is another directory",
			request: func(r *model.ExecutionCmuxHandoffRequest) { r.CWD = "/elsewhere" },
			want:    "request cwd is not the canonical worktree",
		},
		{
			name: "process cwd is another directory",
			mutate: func(env *port.RootEnvironment, _ *fakeDir) {
				env.Getwd = func() (string, error) { return "/elsewhere", nil }
			},
			want: "process cwd is not the canonical worktree",
		},
		{
			name: "process cwd cannot be read",
			mutate: func(env *port.RootEnvironment, _ *fakeDir) {
				env.Getwd = func() (string, error) { return "", errors.New("no cwd") }
			},
			want: "process cwd is not the canonical worktree",
		},
		{
			name: "git top is a different directory",
			mutate: func(env *port.RootEnvironment, _ *fakeDir) {
				env.GitTop = func(string) (string, error) { return "/parent", nil }
			},
			want: "cwd is not the canonical Git worktree",
		},
		{
			name: "git top lookup fails",
			mutate: func(env *port.RootEnvironment, _ *fakeDir) {
				env.GitTop = func(string) (string, error) { return "", errors.New("not git") }
			},
			want: "cwd is not the canonical Git worktree",
		},
		{
			name:   "pinning the root fails",
			mutate: func(_ *port.RootEnvironment, dir *fakeDir) { dir.pinErr = errors.New("EMFILE") },
			want:   "open cmux handoff canonical worktree: EMFILE",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := &fakeDir{path: "/workspace"}
			env := fenceEnv(dir)
			request := fenceRequest()
			if tc.mutate != nil {
				tc.mutate(&env, dir)
			}
			if tc.request != nil {
				tc.request(&request)
			}

			fence, _, err := openRootFence(env, "state", request)

			if err == nil || fence != nil {
				t.Fatalf("fence=%v err=%v; want rejection", fence, err)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Fatalf("err = %v, want it to wrap %v", err, tc.wantIs)
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestRootFenceCheckDetectsDriftAfterOpen(t *testing.T) {
	for _, tc := range []struct {
		name   string
		drift  func(record *model.IssueOpsRecord, dir *fakeDir)
		want   string
		closed bool
	}{
		{
			name:  "execution removed",
			drift: func(record *model.IssueOpsRecord, _ *fakeDir) { record.Execution = nil },
			want:  "durable worktree path changed",
		},
		{
			name:  "workspace root rewritten",
			drift: func(record *model.IssueOpsRecord, _ *fakeDir) { record.Execution.Workspace.Root = "/other" },
			want:  "durable worktree path changed",
		},
		{
			name:  "worktree path rewritten",
			drift: func(record *model.IssueOpsRecord, _ *fakeDir) { record.WorktreePath = "/other" },
			want:  "durable worktree path changed",
		},
		{
			name: "lease no longer released",
			drift: func(record *model.IssueOpsRecord, _ *fakeDir) {
				record.Execution.Lease.Status = model.LeaseStatusActive
			},
			want: "exact released direct execution generation",
		},
		{
			name:  "filesystem identity replaced",
			drift: func(_ *model.IssueOpsRecord, dir *fakeDir) { dir.notMatch = true },
			want:  "filesystem identity changed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := &fakeDir{path: "/workspace"}
			env := fenceEnv(dir)
			drifted := false
			env.Read = func(string, string) (model.IssueOpsRecord, error) {
				record := releasedRecord()
				if drifted {
					tc.drift(&record, dir)
				}
				return record, nil
			}
			fence, _, err := openRootFence(env, "state", fenceRequest())
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer fence.Close()

			drifted = true
			_, err = fence.Check()

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Check err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRootFenceCloseToleratesNilFence(t *testing.T) {
	var fence *rootFence
	fence.Close()
	(&rootFence{}).Close()
}

// handoffHarness drives Service.Handoff with every collaborator observable. Reads of
// the durable record are counted so a test can make the Nth fence check fail.
type handoffHarness struct {
	dir                                *fakeDir
	service                            Service
	client                             *harnessClient
	reads, observeCalls                int
	failReadAt, failObserveAt          int
	observations                       []model.IssueOpsHandoffDeliveryObservation
	prepared, awaited, cleaned         int
	prepareErr, receiptErr, cleanupErr error
	request                            model.ExecutionCmuxHandoffRequest
	created                            cmux.CreatedWorkspace
	existing                           []model.IssueOpsHandoffDeliveryObservation
	readAuditErr, promptErr, execErr   error
	profileErr                         error
	preflightCalls                     int
	samePath                           func(string, string) bool
	promptReads                        []string
}

type harnessClient struct {
	h                   *handoffHarness
	preflightErr        error
	createErr, sendErr  error
	sendAccepted        bool
	create, send        int
	sentCommand         string
	preflightedSocket   string
	createdCWD, attempt string
}

func (c *harnessClient) Preflight(_ context.Context, request cmux.PreflightRequest) (cmux.PreflightResult, error) {
	c.h.preflightCalls++
	c.preflightedSocket = request.SocketPath
	return cmux.PreflightResult{Build: "build-7", Executable: "/bin/cmux"}, c.preflightErr
}

func (c *harnessClient) CreateWorkspace(_ context.Context, request cmux.CreateRequest) (cmux.CreatedWorkspace, error) {
	c.create++
	c.createdCWD, c.attempt = request.CWD, request.AttemptID
	return c.h.created, c.createErr
}

func (c *harnessClient) Send(_ context.Context, request cmux.SendRequest) (cmux.SendReceipt, error) {
	c.send++
	c.sentCommand = request.Command
	return cmux.SendReceipt{Accepted: c.sendAccepted}, c.sendErr
}

func newHarness() *handoffHarness {
	h := &handoffHarness{
		dir:     &fakeDir{path: "/workspace"},
		created: cmux.CreatedWorkspace{WorkspaceID: "ws", SurfaceID: "sf", CWD: "/workspace"},
		request: model.ExecutionCmuxHandoffRequest{
			ID: "cycle", Generation: 2, CWD: "/workspace", PromptFile: "/workspace/prompt",
			PromptSHA256: strings.Repeat("a", 64), MaterialSHA256: strings.Repeat("b", 64),
			Host: "codex", HostExecutable: "/codex", Model: "m", Effort: "high", SocketPath: "/sock",
		},
		samePath: func(string, string) bool { return true },
	}
	h.client = &harnessClient{h: h, sendAccepted: true}
	env := fenceEnv(h.dir)
	env.Read = func(string, string) (model.IssueOpsRecord, error) {
		h.reads++
		if h.failReadAt != 0 && h.reads == h.failReadAt {
			return model.IssueOpsRecord{}, errors.New("record read failed")
		}
		return releasedRecord(), nil
	}
	h.service = Service{
		Client: h.client, Roots: env, ArtifactRoot: "/artifacts",
		Now:       func() time.Time { return time.Unix(100, 0) },
		SamePath:  func(a, b string) bool { return h.samePath(a, b) },
		ReadAudit: func() ([]model.IssueOpsHandoffDeliveryObservation, error) { return h.existing, h.readAuditErr },
		Observe: func(_ model.IssueOpsRecord, o model.IssueOpsHandoffDeliveryObservation) (audit.HandoffDeliveryAuditRecord, error) {
			h.observeCalls++
			if o.Timing != nil {
				snapshot := *o.Timing
				o.Timing = &snapshot
			}
			h.observations = append(h.observations, o)
			if h.failObserveAt != 0 && h.observeCalls == h.failObserveAt {
				return audit.HandoffDeliveryAuditRecord{Observation: o}, errors.New("audit write failed")
			}
			return audit.HandoffDeliveryAuditRecord{Observation: o}, nil
		},
		ReadPrompt: func(_, path, _ string) ([]byte, error) {
			h.promptReads = append(h.promptReads, path)
			return []byte("prompt"), h.promptErr
		},
		ValidateHostExecutable: func(string) error { return h.execErr },
		ValidateHostProfile:    func(string, string, string, string) error { return h.profileErr },
		Prepare: func(cmux.ArtifactRequest) (cmux.PreparedLauncher, error) {
			h.prepared++
			return cmux.PreparedLauncher{Directory: "/recovery", Command: "launch"}, h.prepareErr
		},
		AwaitReceipt: func(context.Context, cmux.PreparedLauncher, cmux.BootstrapExpectation) (model.NativeProcessReceipt, error) {
			h.awaited++
			return model.NativeProcessReceipt{PID: 42, StartedAt: "t0", Executable: "/codex"}, h.receiptErr
		},
		Cleanup: func(cmux.PreparedLauncher) error { h.cleaned++; return h.cleanupErr },
	}
	return h
}

func (h *handoffHarness) run(t *testing.T) (model.ExecutionCmuxHandoffResult, error) {
	t.Helper()
	return h.service.Handoff(t.Context(), "state", h.request)
}

func TestHandoffRejectsBeforeAnyExternalCall(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(h *handoffHarness)
		want  string
	}{
		{"audit history unreadable", func(h *handoffHarness) { h.readAuditErr = errors.New("audit unreadable") }, "audit unreadable"},
		{"lineage already attempted", func(h *handoffHarness) {
			_, lineageID := issueopsdomain.ManualCmuxHandoffIDs(h.request)
			h.existing = []model.IssueOpsHandoffDeliveryObservation{{LifecycleID: "cycle", LineageID: lineageID}}
		}, "attempt already exists"},
		{"generation already staged under another lineage", func(h *handoffHarness) {
			h.existing = []model.IssueOpsHandoffDeliveryObservation{{
				LifecycleID: "cycle", LineageID: "other", SourceGeneration: 2,
				Launcher:   model.IssueOpsHandoffDeliveryLauncher{Name: model.IssueOpsHandoffDeliveryLauncherCmux},
				CallStaged: model.IssueOpsHandoffDeliveryState{Status: model.IssueOpsHandoffDeliveryStateObserved},
			}}
		}, "generation already has a staged attempt"},
		{"prompt outside the worktree", func(h *handoffHarness) { h.request.PromptFile = "/tmp/prompt" }, "prompt file must be inside the canonical worktree"},
		{"prompt path not clean", func(h *handoffHarness) { h.request.PromptFile = "/workspace/../etc/prompt" }, "clean path"},
		{"prompt unreadable", func(h *handoffHarness) { h.promptErr = errors.New("prompt digest mismatch") }, "prompt digest mismatch"},
		{"material digest malformed", func(h *handoffHarness) { h.request.MaterialSHA256 = strings.Repeat("B", 64) }, "material digest is invalid"},
		{"host executable rejected", func(h *handoffHarness) { h.execErr = errors.New("host binary untrusted") }, "host binary untrusted"},
		{"host profile rejected", func(h *handoffHarness) { h.profileErr = errors.New("profile unsupported") }, "profile unsupported"},
		{"request carries NUL", func(h *handoffHarness) { h.request.Model = "m\x00x" }, "NUL bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			tc.setup(h)

			result, err := h.run(t)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if result.Status != "rejected" || result.OK {
				t.Fatalf("result = %+v, want rejected", result)
			}
			if h.preflightCalls != 0 || h.client.create != 0 || h.client.send != 0 || h.observeCalls != 0 {
				t.Fatalf("external effects leaked: preflight=%d create=%d send=%d observe=%d", h.preflightCalls, h.client.create, h.client.send, h.observeCalls)
			}
			if h.dir.closed > 1 {
				t.Fatalf("pin closed %d times", h.dir.closed)
			}
		})
	}
}

func TestHandoffRejectsMissingDependencies(t *testing.T) {
	h := newHarness()
	for name, strip := range map[string]func(*Service){
		"client":          func(s *Service) { s.Client = nil },
		"clock":           func(s *Service) { s.Now = nil },
		"git top":         func(s *Service) { s.Roots.GitTop = nil },
		"host executable": func(s *Service) { s.ValidateHostExecutable = nil },
	} {
		service := h.service
		strip(&service)
		result, err := service.Handoff(t.Context(), "state", h.request)
		if err == nil || !strings.Contains(err.Error(), "dependencies are unavailable") || result.Status != "rejected" || result.ID != "cycle" || result.Generation != 2 {
			t.Fatalf("%s: result=%+v err=%v", name, result, err)
		}
	}
	if h.reads != 0 {
		t.Fatalf("the record was read despite missing dependencies: %d", h.reads)
	}
}

func TestHandoffPreflightFailureReportsUnavailableWithoutStaging(t *testing.T) {
	h := newHarness()
	h.client.preflightErr = errors.New("cmux not running")

	result, err := h.run(t)

	if err == nil || !strings.Contains(err.Error(), "cmux not running") || result.Status != "unavailable" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if h.observeCalls != 0 || h.client.create != 0 {
		t.Fatalf("an unavailable cmux must leave no audit or workspace: observe=%d create=%d", h.observeCalls, h.client.create)
	}
	if h.client.preflightedSocket != "/sock" {
		t.Fatalf("preflight must target the requested socket, got %q", h.client.preflightedSocket)
	}
	if h.dir.closed != 1 {
		t.Fatalf("pin closed %d times, want 1", h.dir.closed)
	}
}

// Each fence re-check between phases must stop the next external effect. Reads: 1-2 open the
// fence, 3 precedes preflight, 4 precedes workspace creation, 5 precedes the launcher send.
func TestHandoffFenceDriftStopsTheNextEffect(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		failReadAt                int
		wantStatus                string
		wantPreflight, wantCreate int
		wantSend                  int
		wantRecovery              string
		wantObservations          int
	}{
		{"before preflight", 3, "rejected", 0, 0, 0, "", 0},
		{"before workspace creation", 4, "staged", 1, 0, 0, "", 1},
		{"before sending the launcher", 5, "pre_send_failed", 1, 1, 0, "/recovery", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			h.failReadAt = tc.failReadAt

			result, err := h.run(t)

			if err == nil || !strings.Contains(err.Error(), "record read failed") {
				t.Fatalf("err = %v", err)
			}
			if result.Status != tc.wantStatus || result.RecoveryArtifactDir != tc.wantRecovery || result.InputAccepted {
				t.Fatalf("result = %+v", result)
			}
			if h.preflightCalls != tc.wantPreflight || h.client.create != tc.wantCreate || h.client.send != tc.wantSend {
				t.Fatalf("effects preflight=%d create=%d send=%d", h.preflightCalls, h.client.create, h.client.send)
			}
			if h.observeCalls != tc.wantObservations {
				t.Fatalf("observe calls = %d, want %d", h.observeCalls, tc.wantObservations)
			}
		})
	}
}

func TestHandoffCreateWorkspaceFailures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		setup      func(h *handoffHarness)
		wantErr    string
		wantStatus string
		wantObs    int
	}{
		{
			name:       "plain create error is a definite failure",
			setup:      func(h *handoffHarness) { h.client.createErr = errors.New("socket refused") },
			wantErr:    "socket refused",
			wantStatus: "failed",
			wantObs:    2,
		},
		{
			name: "created cwd is another directory",
			setup: func(h *handoffHarness) {
				h.samePath = func(string, string) bool { return false }
			},
			wantErr:    "created workspace cwd target mismatch",
			wantStatus: "failed",
			wantObs:    2,
		},
		{
			name:       "created surface missing",
			setup:      func(h *handoffHarness) { h.created.SurfaceID = "" },
			wantErr:    "created target identity is incomplete",
			wantStatus: "failed",
			wantObs:    2,
		},
		{
			name:       "created workspace missing",
			setup:      func(h *handoffHarness) { h.created.WorkspaceID = "" },
			wantErr:    "created target identity is incomplete",
			wantStatus: "failed",
			wantObs:    2,
		},
		{
			name: "create error and its audit both fail",
			setup: func(h *handoffHarness) {
				h.client.createErr = errors.New("socket refused")
				h.failObserveAt = 2
			},
			wantErr:    "audit write failed",
			wantStatus: "staged",
			wantObs:    2,
		},
		{
			name:       "audit of a successful create fails",
			setup:      func(h *handoffHarness) { h.failObserveAt = 2 },
			wantErr:    "audit write failed",
			wantStatus: "staged",
			wantObs:    2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			tc.setup(h)

			result, err := h.run(t)

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			if result.Status != tc.wantStatus || result.OK || result.InputAccepted {
				t.Fatalf("result = %+v, want status %q", result, tc.wantStatus)
			}
			if h.prepared != 0 || h.client.send != 0 || h.awaited != 0 {
				t.Fatalf("a failed create must stop the launcher: prepared=%d send=%d awaited=%d", h.prepared, h.client.send, h.awaited)
			}
			if h.observeCalls != tc.wantObs {
				t.Fatalf("observe calls = %d, want %d", h.observeCalls, tc.wantObs)
			}
			if last := h.observations[len(h.observations)-1]; last.Ambiguous.Status == model.IssueOpsHandoffDeliveryStateObserved {
				t.Fatalf("a definite failure must not be flagged ambiguous: %+v", last.Ambiguous)
			}
		})
	}
}

func TestHandoffRecordsCreatedTargetAndTimingBeforeSending(t *testing.T) {
	h := newHarness()
	h.created.CreateMS, h.created.ResolveMS = 11, 7
	h.created.WorkspaceID, h.created.SurfaceID = "ws-9", "sf-9"

	result, err := h.run(t)

	if err != nil || result.Status != "input_accepted" || !result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	staged := h.observations[0]
	if staged.CallStaged.Status != model.IssueOpsHandoffDeliveryStateObserved || staged.InputAccepted.Status == model.IssueOpsHandoffDeliveryStateObserved {
		t.Fatalf("first audit must stage the call before any input: %+v", staged)
	}
	if staged.Launcher.Version != "build-7" || staged.Launcher.Path != "/bin/cmux" || staged.Timing.PreflightMS < 1 {
		t.Fatalf("staged launcher/timing = %+v / %+v", staged.Launcher, staged.Timing)
	}
	created := h.observations[1]
	if created.Target.WorkspaceID != "ws-9" || created.Target.SurfaceID != "sf-9" || created.Timing.WorkspaceCreateMS != 11 || created.Timing.TargetResolveMS != 7 {
		t.Fatalf("created target/timing not audited: %+v / %+v", created.Target, created.Timing)
	}
	final := h.observations[len(h.observations)-1]
	if final.Target.Process == nil || final.Target.Process.PID != 42 || final.Target.ProcessIncarnation != "42:t0:/codex" {
		t.Fatalf("receiver process not audited: %+v", final.Target)
	}
	if final.Timing.ReceiverReceiptMS < 1 {
		t.Fatalf("receiver receipt timing must be at least 1ms, got %d", final.Timing.ReceiverReceiptMS)
	}
	if len(h.promptReads) != 1 || h.promptReads[0] != "/workspace/prompt" {
		t.Fatalf("prompt must be read once from the canonical path: %v", h.promptReads)
	}
	if h.client.createdCWD != "/workspace" || h.client.sentCommand != "launch" {
		t.Fatalf("create cwd=%q sent=%q", h.client.createdCWD, h.client.sentCommand)
	}
}

func TestHandoffPrepareFailureStopsBeforeSend(t *testing.T) {
	h := newHarness()
	h.prepareErr = errors.New("artifact dir not writable")

	result, err := h.run(t)

	if err == nil || !strings.Contains(err.Error(), "artifact dir not writable") {
		t.Fatalf("err = %v", err)
	}
	if result.Status != "pre_send_failed" || result.RecoveryArtifactDir != "" || result.OK {
		t.Fatalf("result = %+v", result)
	}
	if h.client.send != 0 || h.awaited != 0 || h.cleaned != 0 {
		t.Fatalf("send=%d awaited=%d cleaned=%d", h.client.send, h.awaited, h.cleaned)
	}
	if h.client.create != 1 {
		t.Fatalf("the workspace was already created: %d", h.client.create)
	}
}

func TestHandoffSendOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		setup      func(h *handoffHarness)
		wantErr    string
		wantStatus string
	}{
		{
			name:       "plain send error is a definite failure",
			setup:      func(h *handoffHarness) { h.client.sendErr = errors.New("surface closed") },
			wantErr:    "surface closed",
			wantStatus: "failed",
		},
		{
			name:       "send returns no accepted receipt",
			setup:      func(h *handoffHarness) { h.client.sendAccepted = false },
			wantErr:    "send returned no accepted receipt",
			wantStatus: "failed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			tc.setup(h)

			result, err := h.run(t)

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			if result.Status != tc.wantStatus || result.OK || result.InputAccepted || result.RecoveryArtifactDir != "/recovery" {
				t.Fatalf("result = %+v", result)
			}
			if h.awaited != 0 || h.cleaned != 0 {
				t.Fatalf("no receipt may be awaited after a failed send: awaited=%d cleaned=%d", h.awaited, h.cleaned)
			}
			if last := h.observations[len(h.observations)-1]; last.Ambiguous.Status == model.IssueOpsHandoffDeliveryStateObserved || last.InputAccepted.Status == model.IssueOpsHandoffDeliveryStateObserved {
				t.Fatalf("definite failure audited as accepted/ambiguous: %+v", last)
			}
		})
	}
}

func TestHandoffAuditFailureAfterAcceptedInputStillReportsAcceptance(t *testing.T) {
	h := newHarness()
	h.failObserveAt = 3 // stage, create, input accepted

	result, err := h.run(t)

	if err == nil || !strings.Contains(err.Error(), "audit write failed") {
		t.Fatalf("err = %v", err)
	}
	if result.Status != "input_accepted" || !result.InputAccepted || result.RecoveryArtifactDir != "/recovery" {
		t.Fatalf("input was delivered, so the result must say so and keep recovery: %+v", result)
	}
	if h.awaited != 0 || h.cleaned != 0 {
		t.Fatalf("awaited=%d cleaned=%d; an unaudited acceptance must stop here", h.awaited, h.cleaned)
	}
}

func TestHandoffFinalAuditFailureKeepsRecoveryArtifact(t *testing.T) {
	for _, tc := range []struct {
		name        string
		receiptErr  error
		failAt      int
		wantStatus  string
		wantCleaned int
	}{
		{"receipt audit fails", nil, 4, "input_accepted", 0},
		{"unverified receiver audit fails", errors.New("no receipt"), 4, "input_accepted_receiver_unverified", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			h.receiptErr = tc.receiptErr
			h.failObserveAt = tc.failAt

			result, err := h.run(t)

			if err == nil || !strings.Contains(err.Error(), "audit write failed") {
				t.Fatalf("err = %v", err)
			}
			if result.Status != tc.wantStatus || !result.InputAccepted || result.RecoveryArtifactDir != "/recovery" {
				t.Fatalf("result = %+v", result)
			}
			if h.cleaned != tc.wantCleaned {
				t.Fatalf("recovery artifact cleaned %d times, want %d", h.cleaned, tc.wantCleaned)
			}
		})
	}
}

func TestHandoffReceiverUnverifiedLeavesProcessUnrecorded(t *testing.T) {
	h := newHarness()
	h.receiptErr = errors.New("receipt timeout")

	result, err := h.run(t)

	if err != nil || !result.OK || result.Status != "input_accepted_receiver_unverified" || result.RecoveryArtifactDir != "/recovery" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result.Target.Process != nil || result.Target.ProcessIncarnation != "" {
		t.Fatalf("an unverified receiver must not be attributed a process: %+v", result.Target)
	}
	if h.cleaned != 0 {
		t.Fatalf("recovery artifact must survive an unverified receiver, cleaned=%d", h.cleaned)
	}
}
