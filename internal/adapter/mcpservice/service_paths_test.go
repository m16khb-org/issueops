package mcpservice

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/contract/mcpservice"
	"issueops/internal/contract/processidentity"
)

// serviceWithRun routes supervisor commands through hook first; a hook that
// returns handled=false falls through to the fake world.
func (w *fakeWorld) serviceWithRun(address string, hook func(args []string) (out []byte, err error, handled bool)) *Service {
	service := w.service(address)
	service.supervisor = launchd{label: "io.issueops.test", uid: 501, home: service.cfg.Home,
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if out, err, handled := hook(args); handled {
				return out, err
			}
			return w.run(ctx, name, args...)
		}}
	return service
}

func TestProbeRejectsResponsesThatDoNotProveAnIssueopsServer(t *testing.T) {
	const okBody = `{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"issueops"}}}`
	for _, tc := range []struct {
		name    string
		status  int
		ctype   string
		body    string
		headers map[string]string
		want    string
	}{
		{name: "non-200", status: http.StatusUnauthorized, body: "no", want: "HTTP 401"},
		{name: "json body from another server", status: 200, ctype: "application/json", body: `{"result":{"serverInfo":{"name":"other"}}}`, want: "did not come from issueops"},
		{name: "json body that does not decode", status: 200, ctype: "application/json", body: "<html>", want: "decode initialize response"},
		{name: "event stream with garbage data", status: 200, ctype: "text/event-stream", body: "data: nope\n\n", want: "decode initialize response"},
		{name: "event stream without a data line", status: 200, ctype: "text/event-stream", body: "event: ping\n\n", want: "decode initialize response"},
		{name: "missing pid header", status: 200, ctype: "application/json", body: okBody, want: HeaderPID},
		{name: "non-numeric pid header", status: 200, ctype: "application/json", body: okBody, headers: map[string]string{HeaderPID: "abc"}, want: HeaderPID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer "+testBearer {
					t.Errorf("probe sent Authorization %q", r.Header.Get("Authorization"))
				}
				if tc.ctype != "" {
					rw.Header().Set("Content-Type", tc.ctype)
				}
				for k, v := range tc.headers {
					rw.Header().Set(k, v)
				}
				rw.WriteHeader(tc.status)
				_, _ = rw.Write([]byte(tc.body))
			}))
			defer server.Close()
			w := newFakeWorld(t)
			_, _, err := w.service(strings.TrimPrefix(server.URL, "http://")).probe(t.Context())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("probe error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestProbeAcceptsPlainJSONResponseAndReturnsIdentityHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set(HeaderPID, "321")
		rw.Header().Set(HeaderBuildID, "build-x")
		_, _ = rw.Write([]byte(`{"result":{"serverInfo":{"name":"issueops"}}}`))
	}))
	defer server.Close()
	w := newFakeWorld(t)
	pid, build, err := w.service(strings.TrimPrefix(server.URL, "http://")).probe(t.Context())
	if err != nil || pid != 321 || build != "build-x" {
		t.Fatalf("pid=%d build=%q err=%v", pid, build, err)
	}
}

func TestProbeFailsWithoutAnInstalledOrSafeBearer(t *testing.T) {
	w := newFakeWorld(t)
	path := filepath.Join(httpDir(w.stateDir), bearerFileName)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.service(w.address).probe(t.Context()); err == nil || !strings.Contains(err.Error(), "owner-only") {
		t.Fatalf("shared bearer probe error = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.service(w.address).probe(t.Context()); err == nil || !strings.Contains(err.Error(), "bearer is not installed") {
		t.Fatalf("missing bearer probe error = %v", err)
	}
}

func TestStatusReportsUnreadableLockAsStateUnreadable(t *testing.T) {
	w := newFakeWorld(t)
	service := w.service(w.address)
	service.cfg.LockHolder = func(string) (int, error) { return 0, os.ErrPermission }
	status, err := service.Status(t.Context())
	if !errors.Is(err, os.ErrPermission) || status.ErrorCode != CodeStateUnreadable || status.OK {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	start, err := service.Start(t.Context())
	if err == nil || start.ErrorCode != CodeStateUnreadable || countCommands(w.commands, "launchctl bootstrap") != 0 {
		t.Fatalf("start status=%+v err=%v commands=%v", start, err, w.commands)
	}
}

func TestStatusReportsMismatchWhenRecordIsMissingOrUnreadableForALockHolder(t *testing.T) {
	w := newFakeWorld(t)
	w.holder = 4242
	status, err := w.service(w.address).Status(t.Context())
	if err == nil || status.Status != mcpservice.StatusConflict || status.ErrorCode != CodeIdentityMismatch || status.PID != 4242 || !strings.Contains(err.Error(), "instance record unavailable") {
		t.Fatalf("no record: status=%+v err=%v", status, err)
	}
	w.serve(4243, w.buildID())
	w.holder = 4242
	status, err = w.service(w.address).Status(t.Context())
	if err == nil || status.ErrorCode != CodeIdentityMismatch || !strings.Contains(err.Error(), "record pid 4243") {
		t.Fatalf("pid differs: status=%+v err=%v", status, err)
	}
	w.holder = 4243
	delete(w.identities, 4243)
	status, err = w.service(w.address).Status(t.Context())
	if err == nil || status.ErrorCode != CodeIdentityMismatch || !strings.Contains(err.Error(), "process identity unavailable") {
		t.Fatalf("no identity: status=%+v err=%v", status, err)
	}
}

func TestStartRefusesToReplaceRunningServiceOfAnotherBuild(t *testing.T) {
	w := newFakeWorld(t)
	w.serve(4242, "previous-build")
	status, err := w.service(w.address).Start(t.Context())
	if err == nil || status.ErrorCode != CodeBuildMismatch || status.Status != mcpservice.StatusRunning || status.PID != 4242 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if len(w.commands) != 0 {
		t.Fatalf("Start touched the supervisor: %v", w.commands)
	}
}

func TestStartReportsMissingBinaryWithoutObservingOrLoading(t *testing.T) {
	w := newFakeWorld(t)
	service := w.service(w.address)
	service.cfg.Binary = filepath.Join(t.TempDir(), "gone")
	service.cfg.LockHolder = func(string) (int, error) { t.Error("observed before the binary check"); return 0, nil }
	status, err := service.Start(t.Context())
	if err == nil || status.ErrorCode != CodeBuildMismatch || !strings.Contains(err.Error(), "read service binary") || len(w.commands) != 0 {
		t.Fatalf("status=%+v err=%v commands=%v", status, err, w.commands)
	}
}

func TestStartDoesNotLoadOverConflictOrSilentHolder(t *testing.T) {
	w := newFakeWorld(t)
	w.serve(4242, w.buildID())
	w.identities[4242] = processidentity.Identity{StartTime: "reused", Executable: w.binary}
	status, err := w.service(w.address).Start(t.Context())
	if err == nil || status.Status != mcpservice.StatusConflict || status.ErrorCode != CodeIdentityMismatch {
		t.Fatalf("conflict: status=%+v err=%v", status, err)
	}

	silent := newFakeWorld(t)
	silent.serve(4242, silent.buildID())
	silent.serving = nil
	status, err = silent.service(silent.address).Start(t.Context())
	if err == nil || status.Status != mcpservice.StatusStale || status.ErrorCode != CodeNotReady {
		t.Fatalf("silent holder: status=%+v err=%v", status, err)
	}
	if len(w.commands)+len(silent.commands) != 0 {
		t.Fatalf("supervisor was driven: %v %v", w.commands, silent.commands)
	}
}

func TestStartCannotClearStaleRecordItCannotRemove(t *testing.T) {
	w := newFakeWorld(t)
	record := filepath.Join(httpDir(w.stateDir), recordFileName)
	if err := os.MkdirAll(filepath.Join(record, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	status, err := w.service(w.address).Start(t.Context())
	if err == nil || status.ErrorCode != CodeStateUnreadable || status.Status != mcpservice.StatusStale || countCommands(w.commands, "launchctl bootstrap") != 0 {
		t.Fatalf("status=%+v err=%v commands=%v", status, err, w.commands)
	}
}

func TestStartReportsEachLoadStageFailureWithItsCode(t *testing.T) {
	boom := errors.New("boom")
	for _, tc := range []struct {
		name     string
		loaded   bool
		hook     func(args []string) ([]byte, error, bool)
		bearer   error
		wantCode string
		wantErr  string
	}{
		{name: "credential", bearer: boom, wantCode: CodeCredentialFailed, wantErr: "prepare mcp http credential"},
		{name: "supervisor state unparsable", hook: func(args []string) ([]byte, error, bool) {
			if args[0] == "print" {
				return []byte("pid = x\n"), nil, true
			}
			return nil, nil, false
		}, wantCode: CodeSupervisorFailed, wantErr: "parse launchd pid"},
		{name: "idle job cannot be unloaded", loaded: true, hook: func(args []string) ([]byte, error, bool) {
			switch args[0] {
			case "print":
				return []byte("pid = 0\n"), nil, true
			case "bootout":
				return nil, boom, true
			}
			return nil, nil, false
		}, wantCode: CodeSupervisorFailed, wantErr: "unload idle mcp service job"},
		{name: "bootstrap refused", hook: func(args []string) ([]byte, error, bool) {
			if args[0] == "bootstrap" {
				return []byte("Bootstrap failed: 5"), boom, true
			}
			return nil, nil, false
		}, wantCode: CodeSupervisorFailed, wantErr: "Bootstrap failed: 5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newFakeWorld(t)
			w.loaded = tc.loaded
			hook := tc.hook
			if hook == nil {
				hook = func([]string) ([]byte, error, bool) { return nil, nil, false }
			}
			service := w.serviceWithRun(w.address, hook)
			if tc.bearer != nil {
				service.cfg.EnsureBearer = func() (string, error) { return "", tc.bearer }
			}
			status, err := service.Start(t.Context())
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || status.ErrorCode != tc.wantCode || status.Status != mcpservice.StatusStopped {
				t.Fatalf("status=%+v err=%v", status, err)
			}
		})
	}
}

func TestStartUnloadsIdleJobBeforeLoadingAndRejectsWrongBuildAfterLoad(t *testing.T) {
	w := newFakeWorld(t)
	w.loaded = true
	w.onLoad = func() { w.serve(5150, "unexpected-build") }
	status, err := w.service(w.address).Start(t.Context())
	if err == nil || status.ErrorCode != CodeBuildMismatch || status.Status != mcpservice.StatusRunning {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	var verbs []string
	for _, command := range w.commands {
		if fields := strings.Fields(command); len(fields) > 1 && fields[1] != "print" {
			verbs = append(verbs, fields[1])
		}
	}
	if strings.Join(verbs, ",") != "bootout,bootstrap,bootout" {
		t.Fatalf("supervisor verbs = %v, want idle unload, load, unload of the wrong build", verbs)
	}
}

func TestStopOfStoppedServiceUnloadsOnlyAnIdleLoadedJob(t *testing.T) {
	w := newFakeWorld(t)
	status, err := w.service(w.address).Stop(t.Context())
	if err != nil || status.Status != mcpservice.StatusStopped || countCommands(w.commands, "launchctl bootout") != 0 {
		t.Fatalf("not loaded: status=%+v err=%v commands=%v", status, err, w.commands)
	}
	w.loaded = true
	status, err = w.service(w.address).Stop(t.Context())
	if err != nil || status.Status != mcpservice.StatusStopped || countCommands(w.commands, "launchctl bootout") != 1 || w.loaded {
		t.Fatalf("loaded idle: status=%+v err=%v commands=%v", status, err, w.commands)
	}
}

func TestStopReportsSupervisorFailuresWhileUnloadingIdleJob(t *testing.T) {
	boom := errors.New("boom")
	w := newFakeWorld(t)
	status, err := w.serviceWithRun(w.address, func(args []string) ([]byte, error, bool) {
		if args[0] == "print" {
			return []byte("pid = x\n"), nil, true
		}
		return nil, nil, false
	}).Stop(t.Context())
	if err == nil || status.ErrorCode != CodeSupervisorFailed || status.Status != mcpservice.StatusStopped {
		t.Fatalf("print failure: status=%+v err=%v", status, err)
	}
	status, err = w.serviceWithRun(w.address, func(args []string) ([]byte, error, bool) {
		switch args[0] {
		case "print":
			return []byte("pid = 0\n"), nil, true
		case "bootout":
			return nil, boom, true
		}
		return nil, nil, false
	}).Stop(t.Context())
	if !errors.Is(err, boom) || status.ErrorCode != CodeSupervisorFailed {
		t.Fatalf("bootout failure: status=%+v err=%v", status, err)
	}
}

func TestStopClearsStaleRecordAndUnloadsLoadedJob(t *testing.T) {
	w := newFakeWorld(t)
	w.serve(4242, w.buildID())
	w.holder = 0
	w.closeEndpoint()
	w.loaded = true
	status, err := w.service(w.address).Stop(t.Context())
	if err != nil || status.Status != mcpservice.StatusStopped || status.PID != 0 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if _, err := os.Stat(filepath.Join(httpDir(w.stateDir), recordFileName)); !os.IsNotExist(err) {
		t.Fatalf("stale record survived: %v", err)
	}
	if countCommands(w.commands, "launchctl bootout") != 1 {
		t.Fatalf("commands=%v", w.commands)
	}
}

func TestStopCannotClearStaleRecordItCannotRemove(t *testing.T) {
	w := newFakeWorld(t)
	if err := os.MkdirAll(filepath.Join(httpDir(w.stateDir), recordFileName, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	status, err := w.service(w.address).Stop(t.Context())
	if err == nil || status.ErrorCode != CodeStateUnreadable || countCommands(w.commands, "launchctl bootout") != 0 {
		t.Fatalf("status=%+v err=%v commands=%v", status, err, w.commands)
	}
}

func TestStopRefusesConflictingOrUnverifiedInstanceWithoutUnloading(t *testing.T) {
	w := newFakeWorld(t)
	w.loaded = true
	w.serve(4242, w.buildID())
	w.identities[4242] = processidentity.Identity{StartTime: "reused", Executable: w.binary}
	status, err := w.service(w.address).Stop(t.Context())
	if err == nil || status.Status != mcpservice.StatusConflict || status.ErrorCode != CodeIdentityMismatch {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if countCommands(w.commands, "launchctl bootout") != 0 {
		t.Fatalf("conflicting instance was unloaded: %v", w.commands)
	}

	silent := newFakeWorld(t)
	silent.loaded = true
	silent.serve(4242, silent.buildID())
	silent.serving = nil
	status, err = silent.service(silent.address).Stop(t.Context())
	if err == nil || status.Status != mcpservice.StatusStale || status.ErrorCode != CodeNotReady || countCommands(silent.commands, "launchctl bootout") != 0 {
		t.Fatalf("unverified: status=%+v err=%v commands=%v", status, err, silent.commands)
	}
}

func TestStopFailsWhenSupervisorStateOrUnloadFails(t *testing.T) {
	boom := errors.New("boom")
	w := newFakeWorld(t)
	w.serve(4242, w.buildID())
	status, err := w.serviceWithRun(w.address, func(args []string) ([]byte, error, bool) {
		if args[0] == "print" {
			return []byte("pid = x\n"), nil, true
		}
		return nil, nil, false
	}).Stop(t.Context())
	if err == nil || status.ErrorCode != CodeSupervisorFailed || status.Status != mcpservice.StatusRunning {
		t.Fatalf("supervised failure: status=%+v err=%v", status, err)
	}
	status, err = w.serviceWithRun(w.address, func(args []string) ([]byte, error, bool) {
		switch args[0] {
		case "print":
			return []byte("pid = 4242\n"), nil, true
		case "bootout":
			return nil, boom, true
		}
		return nil, nil, false
	}).Stop(t.Context())
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "unload mcp service job") || status.ErrorCode != CodeSupervisorFailed {
		t.Fatalf("unload failure: status=%+v err=%v", status, err)
	}
	if _, statErr := os.Stat(filepath.Join(httpDir(w.stateDir), recordFileName)); statErr != nil {
		t.Fatalf("record removed although unload failed: %v", statErr)
	}
}

func TestStopReportsTimeoutWhenHolderNeverReleasesTheLock(t *testing.T) {
	w := newFakeWorld(t)
	w.serve(4242, w.buildID())
	w.loaded = true
	status, err := w.serviceWithRun(w.address, func(args []string) ([]byte, error, bool) {
		switch args[0] {
		case "print":
			return []byte("pid = 4242\n"), nil, true
		case "bootout":
			return nil, nil, true
		}
		return nil, nil, false
	}).Stop(t.Context())
	if !errors.Is(err, context.DeadlineExceeded) || status.ErrorCode != CodeStopTimeout || status.Status != mcpservice.StatusRunning {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if _, statErr := os.Stat(filepath.Join(httpDir(w.stateDir), recordFileName)); statErr != nil {
		t.Fatalf("record removed while holder still runs: %v", statErr)
	}
}

func TestStopReportsLockReadFailureWhileWaitingForRelease(t *testing.T) {
	w := newFakeWorld(t)
	w.serve(4242, w.buildID())
	w.loaded = true
	service := w.service(w.address)
	calls := 0
	service.cfg.LockHolder = func(string) (int, error) {
		calls++
		if calls == 1 {
			return 4242, nil
		}
		return 0, os.ErrPermission
	}
	status, err := service.Stop(t.Context())
	if !errors.Is(err, os.ErrPermission) || status.ErrorCode != CodeStateUnreadable {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestStopKeepsRecordPublishedByASuccessor(t *testing.T) {
	w := newFakeWorld(t)
	w.serve(4242, w.buildID())
	w.loaded = true
	path := filepath.Join(httpDir(w.stateDir), recordFileName)
	successor := Record{SchemaVersion: recordSchema, PID: 6000, StartedAt: "later", Executable: w.binary, BuildID: w.buildID()}
	status, err := w.serviceWithRun(w.address, func(args []string) ([]byte, error, bool) {
		switch args[0] {
		case "print":
			return []byte("pid = 4242\n"), nil, true
		case "bootout":
			w.holder = 0
			w.closeEndpoint()
			return nil, writeRecord(path, successor), true
		}
		return nil, nil, false
	}).Stop(t.Context())
	if err != nil || status.Status != mcpservice.StatusStopped {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if kept, err := readRecord(path); err != nil || kept.PID != 6000 {
		t.Fatalf("successor record = %+v, %v", kept, err)
	}
}

func TestEnsureRunningRefusesOtherBinaryAndPropagatesStopFailure(t *testing.T) {
	w := newFakeWorld(t)
	status, err := w.service(w.address).EnsureRunning(t.Context(), filepath.Join(t.TempDir(), "other"))
	if err == nil || status.ErrorCode != CodeBuildMismatch || len(w.commands) != 0 {
		t.Fatalf("other binary: status=%+v err=%v commands=%v", status, err, w.commands)
	}
	// An older build that the supervisor does not own is neither stopped nor replaced.
	w.serve(4242, "previous-build")
	status, err = w.service(w.address).EnsureRunning(t.Context(), w.binary)
	if err == nil || status.ErrorCode != CodeNotSupervised || countCommands(w.commands, "launchctl bootstrap") != 0 {
		t.Fatalf("unowned older build: status=%+v err=%v commands=%v", status, err, w.commands)
	}
}

func TestEnsureRunningReturnsStartErrorsOtherThanBuildMismatch(t *testing.T) {
	w := newFakeWorld(t)
	occupyAddress(t, w.address)
	status, err := w.service(w.address).EnsureRunning(t.Context(), w.binary)
	if err == nil || status.ErrorCode != CodePortInUse || countCommands(w.commands, "launchctl bootout") != 0 {
		t.Fatalf("status=%+v err=%v commands=%v", status, err, w.commands)
	}
}

func TestPrepareReportsEachSetupFailure(t *testing.T) {
	boom := errors.New("boom")
	t.Run("credential", func(t *testing.T) {
		w := newFakeWorld(t)
		service := w.service(w.address)
		service.cfg.EnsureBearer = func() (string, error) { return "", boom }
		if _, err := service.Prepare(t.Context()); !errors.Is(err, boom) || !strings.Contains(err.Error(), "prepare mcp http credential") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("log file cannot be opened", func(t *testing.T) {
		w := newFakeWorld(t)
		service := w.service(w.address)
		if err := os.MkdirAll(ServiceLogPath(w.stateDir), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Prepare(t.Context()); err == nil || !strings.Contains(err.Error(), "prepare mcp http log") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unit directory is blocked", func(t *testing.T) {
		w := newFakeWorld(t)
		service := w.service(w.address)
		if err := os.WriteFile(service.cfg.Home+"/Library", nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Prepare(t.Context()); err == nil {
			t.Fatal("Prepare succeeded with a regular file where the unit directory belongs")
		}
	})
	t.Run("unit file path is a directory", func(t *testing.T) {
		w := newFakeWorld(t)
		service := w.service(w.address)
		if err := os.MkdirAll(service.supervisor.unitPath(), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Prepare(t.Context()); err == nil {
			t.Fatal("Prepare succeeded although the unit path is a directory")
		}
	})
}

func TestPrepareRewritesChangedUnitAndLeavesIdenticalUnitUntouched(t *testing.T) {
	w := newFakeWorld(t)
	service := w.service(w.address)
	if _, err := service.Prepare(t.Context()); err != nil {
		t.Fatal(err)
	}
	path := service.supervisor.unitPath()
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Prepare(t.Context()); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || !info.ModTime().Equal(old) {
		t.Fatalf("identical unit was rewritten: %v %v", info.ModTime(), err)
	}
	if err := os.WriteFile(path, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Prepare(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(good) {
		t.Fatalf("unit not restored: %q, %v", got, err)
	}
}

func TestWaitContextSleepsThenReturnsNilOrStopsOnCancel(t *testing.T) {
	if err := waitContext(t.Context(), time.Millisecond); err != nil {
		t.Fatalf("elapsed wait = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := waitContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait = %v", err)
	}
}

func TestNewAppliesDefaultsAndLeavesUnsupportedGOOSWithoutSupervisor(t *testing.T) {
	service := New(Config{GOOS: "plan9"})
	if service.supervisor != nil || service.cfg.ReadyTimeout != 20*time.Second || service.cfg.StopTimeout != 20*time.Second || service.cfg.PollInterval != 100*time.Millisecond || service.cfg.Client == nil || service.cfg.Wait == nil || service.cfg.LockHolder == nil {
		t.Fatalf("service = %+v", service.cfg)
	}
	custom := New(Config{GOOS: "linux", ReadyTimeout: time.Second, StopTimeout: 2 * time.Second, PollInterval: 3 * time.Second})
	if custom.cfg.ReadyTimeout != time.Second || custom.cfg.StopTimeout != 2*time.Second || custom.cfg.PollInterval != 3*time.Second {
		t.Fatalf("explicit timings overridden: %+v", custom.cfg)
	}
}
