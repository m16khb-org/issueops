package mcpservice

import (
	"context"
	"errors"
	"fmt"
	"net"
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

const testBearer = "test-bearer-value"

type fakeWorld struct {
	t          *testing.T
	stateDir   string
	binary     string
	holder     int
	loaded     bool
	serving    *Record
	identities map[int]processidentity.Identity
	commands   []string
	onLoad     func()
	waits      int
	maxWaits   int
	address    string
	server     *httptest.Server
}

func newFakeWorld(t *testing.T) *fakeWorld {
	t.Helper()
	state := t.TempDir()
	if err := os.MkdirAll(httpDir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(httpDir(state), bearerFileName), []byte(testBearer+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "issueops")
	if err := os.WriteFile(binary, []byte("build-one"), 0o755); err != nil {
		t.Fatal(err)
	}
	w := &fakeWorld{t: t, stateDir: state, binary: binary, identities: map[int]processidentity.Identity{}, maxWaits: 3, address: freeAddress(t)}
	t.Cleanup(w.closeEndpoint)
	return w
}

func (w *fakeWorld) listen() {
	if w.server != nil {
		return
	}
	listener, err := net.Listen("tcp", w.address)
	if err != nil {
		w.t.Fatal(err)
	}
	w.server = httptest.NewUnstartedServer(w.handler())
	w.server.Listener = listener
	w.server.Start()
}

func (w *fakeWorld) closeEndpoint() {
	if w.server != nil {
		w.server.Close()
		w.server = nil
	}
}

func (w *fakeWorld) buildID() string {
	id, err := BuildID(w.binary)
	if err != nil {
		w.t.Fatal(err)
	}
	return id
}

func (w *fakeWorld) serve(pid int, buildID string) Record {
	record := Record{SchemaVersion: recordSchema, PID: pid, StartedAt: "2026-10-02T00:00:00Z", Executable: w.binary, BuildID: buildID}
	if err := writeRecord(filepath.Join(httpDir(w.stateDir), recordFileName), record); err != nil {
		w.t.Fatal(err)
	}
	w.identities[pid] = processidentity.Identity{StartTime: record.StartedAt, Executable: record.Executable}
	w.holder = pid
	w.serving = &record
	w.listen()
	return record
}

func (w *fakeWorld) exit() {
	w.holder = 0
	w.serving = nil
	w.closeEndpoint()
	_ = os.Remove(filepath.Join(httpDir(w.stateDir), recordFileName))
}

func (w *fakeWorld) handler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if w.serving == nil {
			http.Error(rw, "down", http.StatusServiceUnavailable)
			return
		}
		IdentityHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+testBearer {
				http.Error(rw, "unauthorized", http.StatusUnauthorized)
				return
			}
			rw.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(rw, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"serverInfo\":{\"name\":\"issueops\"}}}\n\n")
		}), *w.serving).ServeHTTP(rw, r)
	})
}

func (w *fakeWorld) run(_ context.Context, name string, args ...string) ([]byte, error) {
	w.commands = append(w.commands, name+" "+strings.Join(args, " "))
	switch args[0] {
	case "print":
		if !w.loaded {
			return []byte("Could not find service"), errors.New("exit status 113")
		}
		pid := 0
		if w.serving != nil {
			pid = w.serving.PID
		}
		return []byte(fmt.Sprintf("gui/501/io.issueops.test = {\n\tstate = running\n\tpid = %d\n}\n", pid)), nil
	case "bootstrap":
		w.loaded = true
		if w.onLoad != nil {
			w.onLoad()
		}
	case "bootout":
		w.loaded = false
		w.exit()
	}
	return nil, nil
}

func (w *fakeWorld) service(address string) *Service {
	return New(Config{
		GOOS: "darwin", Home: w.t.TempDir(), StateDir: w.stateDir, Root: "/src/issueops", Binary: w.binary,
		Address: address, Path: "/mcp", Label: "io.issueops.test", UID: 501,
		Run:      w.run,
		LookPath: func(string) (string, error) { return "/bin/launchctl", nil },
		Inspect: func(pid int) (processidentity.Identity, error) {
			identity, ok := w.identities[pid]
			if !ok {
				return processidentity.Identity{}, errors.New("no such process")
			}
			return identity, nil
		},
		EnsureBearer: func() (string, error) { return testBearer, nil },
		LockHolder:   func(string) (int, error) { return w.holder, nil },
		Wait: func(ctx context.Context, _ time.Duration) error {
			w.waits++
			if w.waits > w.maxWaits {
				return context.DeadlineExceeded
			}
			return ctx.Err()
		},
	})
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func occupyAddress(t *testing.T, address string) {
	t.Helper()
	foreign, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = foreign.Close() })
}

func countCommands(commands []string, prefix string) int {
	count := 0
	for _, command := range commands {
		if strings.HasPrefix(command, prefix) {
			count++
		}
	}
	return count
}

func TestStatusReportsStoppedWhenNoLockRecordOrListener(t *testing.T) {
	w := newFakeWorld(t)
	status, err := w.service(freeAddress(t)).Status(t.Context())
	if err != nil || !status.OK || status.Status != mcpservice.StatusStopped || status.PID != 0 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestStatusReportsPortConflictInsteadOfAnotherPort(t *testing.T) {
	w := newFakeWorld(t)
	address := w.address
	occupyAddress(t, address)
	status, err := w.service(address).Status(t.Context())
	if err == nil || status.Status != mcpservice.StatusConflict || status.ErrorCode != CodePortInUse || status.URL != "http://"+address+"/mcp" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestStatusReportsStaleRecordWhenNoProcessHoldsTheLock(t *testing.T) {
	w := newFakeWorld(t)
	w.serve(4242, w.buildID())
	w.holder = 0
	w.closeEndpoint()
	status, err := w.service(w.address).Status(t.Context())
	if err == nil || status.Status != mcpservice.StatusStale || status.ErrorCode != CodeStaleRecord || status.PID != 4242 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestStatusRunningRequiresIdentityAndAuthenticatedResponse(t *testing.T) {
	w := newFakeWorld(t)
	address := w.address
	w.serve(4242, w.buildID())
	status, err := w.service(address).Status(t.Context())
	if err != nil || !status.OK || status.Status != mcpservice.StatusRunning || status.PID != 4242 || status.BuildID != w.buildID() {
		t.Fatalf("status=%+v err=%v", status, err)
	}

	w.identities[4242] = processidentity.Identity{StartTime: "2026-10-02T09:00:00Z", Executable: w.binary}
	status, err = w.service(address).Status(t.Context())
	if err == nil || status.Status != mcpservice.StatusConflict || status.ErrorCode != CodeIdentityMismatch {
		t.Fatalf("reused pid with a different start time: status=%+v err=%v", status, err)
	}
}

func TestStatusRejectsAnswerFromADifferentProcess(t *testing.T) {
	w := newFakeWorld(t)
	address := w.address
	w.serve(4242, w.buildID())
	impostor := *w.serving
	impostor.PID = 999
	w.serving = &impostor
	status, err := w.service(address).Status(t.Context())
	if err == nil || status.Status != mcpservice.StatusConflict || status.ErrorCode != CodeIdentityMismatch {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestStatusTreatsSilentLockHolderAsNotReadyAndStopRefuses(t *testing.T) {
	w := newFakeWorld(t)
	address := w.address
	w.serve(4242, w.buildID())
	w.serving = nil
	w.loaded = true
	service := w.service(address)
	status, err := service.Status(t.Context())
	if err == nil || status.Status != mcpservice.StatusStale || status.ErrorCode != CodeNotReady || status.PID != 4242 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if _, err := service.Stop(t.Context()); err == nil {
		t.Fatal("stop accepted an instance whose build could not be verified")
	}
	if countCommands(w.commands, "launchctl bootout") != 0 {
		t.Fatalf("unverified instance was booted out: %v", w.commands)
	}
}

func TestStartLoadsSupervisorAndSecondStartIsIdempotent(t *testing.T) {
	w := newFakeWorld(t)
	address := w.address
	w.onLoad = func() { w.serve(5150, w.buildID()) }
	service := w.service(address)
	status, err := service.Start(t.Context())
	if err != nil || !status.OK || status.Status != mcpservice.StatusRunning || status.PID != 5150 || status.BuildID != w.buildID() {
		t.Fatalf("start status=%+v err=%v", status, err)
	}
	unit, err := os.ReadFile(service.supervisor.unitPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(unit), testBearer) {
		t.Fatal("unit file must not contain the bearer")
	}
	again, err := service.Start(t.Context())
	if err != nil || again.PID != 5150 || again.Status != mcpservice.StatusRunning {
		t.Fatalf("second start status=%+v err=%v", again, err)
	}
	if got := countCommands(w.commands, "launchctl bootstrap gui/501 "); got != 1 {
		t.Fatalf("bootstrap count = %d, commands=%v", got, w.commands)
	}
}

func TestStartFailsAsConflictWhenPortIsTakenAndNeverLoads(t *testing.T) {
	w := newFakeWorld(t)
	address := w.address
	occupyAddress(t, address)
	status, err := w.service(address).Start(t.Context())
	if err == nil || status.Status != mcpservice.StatusConflict || status.ErrorCode != CodePortInUse {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if countCommands(w.commands, "launchctl bootstrap") != 0 {
		t.Fatalf("supervisor loaded despite conflict: %v", w.commands)
	}
}

func TestStartClearsStaleRecordBeforeLoading(t *testing.T) {
	w := newFakeWorld(t)
	address := w.address
	w.serve(4242, "old-build")
	w.holder = 0
	w.serving = nil
	w.closeEndpoint()
	w.onLoad = func() { w.serve(5150, w.buildID()) }
	status, err := w.service(address).Start(t.Context())
	if err != nil || status.PID != 5150 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestStartTimesOutUnloadsAndReportsReadinessFailure(t *testing.T) {
	w := newFakeWorld(t)
	address := freeAddress(t)
	status, err := w.service(address).Start(t.Context())
	if err == nil || status.ErrorCode != CodeReadinessTimeout || status.OK {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if countCommands(w.commands, "launchctl bootout gui/501/io.issueops.test") != 1 {
		t.Fatalf("failed start must unload the job: %v", w.commands)
	}
}

func TestStopRequiresSupervisedPIDAndWaitsForLockRelease(t *testing.T) {
	w := newFakeWorld(t)
	address := w.address
	w.serve(5150, w.buildID())
	w.loaded = true
	status, err := w.service(address).Stop(t.Context())
	if err != nil || !status.OK || status.Status != mcpservice.StatusStopped {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if _, err := os.Stat(filepath.Join(httpDir(w.stateDir), recordFileName)); !os.IsNotExist(err) {
		t.Fatalf("record survived stop: %v", err)
	}
}

func TestStopRefusesForegroundInstanceTheSupervisorDoesNotOwn(t *testing.T) {
	w := newFakeWorld(t)
	address := w.address
	w.serve(5150, w.buildID())
	status, err := w.service(address).Stop(t.Context())
	if err == nil || status.ErrorCode != CodeNotSupervised || status.Status != mcpservice.StatusRunning {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if countCommands(w.commands, "launchctl bootout") != 0 {
		t.Fatalf("foreground instance was booted out: %v", w.commands)
	}
}

func TestEnsureRunningReplacesOlderBuildThroughStopAndStart(t *testing.T) {
	w := newFakeWorld(t)
	address := w.address
	w.serve(4242, "previous-build")
	w.loaded = true
	w.onLoad = func() { w.serve(5150, w.buildID()) }
	status, err := w.service(address).EnsureRunning(t.Context(), w.binary)
	if err != nil || status.PID != 5150 || status.BuildID != w.buildID() {
		t.Fatalf("status=%+v err=%v commands=%v", status, err, w.commands)
	}
	if countCommands(w.commands, "launchctl bootout") != 1 || countCommands(w.commands, "launchctl bootstrap") != 1 {
		t.Fatalf("commands=%v", w.commands)
	}
}

func TestUnsupportedOrMissingSupervisorIsAnExplicitServiceError(t *testing.T) {
	w := newFakeWorld(t)
	service := w.service(freeAddress(t))
	service.supervisor = nil
	status, err := service.Start(t.Context())
	if err == nil || status.ErrorCode != CodeSupervisorUnsupported || status.Status != "" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	missing := w.service(freeAddress(t))
	missing.cfg.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	status, err = missing.Status(t.Context())
	if err == nil || status.ErrorCode != CodeSupervisorUnavailable {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestIdentityHeadersOnlyOnSuccessfulResponses(t *testing.T) {
	record := Record{PID: 7, BuildID: "abc"}
	handler := IdentityHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(rw, "unauthorized", http.StatusUnauthorized)
			return
		}
		rw.(http.Flusher).Flush()
	}), record)
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if denied.Code != http.StatusUnauthorized || denied.Header().Get(HeaderPID) != "" || denied.Header().Get(HeaderBuildID) != "" {
		t.Fatalf("denied response leaked identity: %d %v", denied.Code, denied.Header())
	}
	request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	request.Header.Set("Authorization", "Bearer x")
	allowed := httptest.NewRecorder()
	handler.ServeHTTP(allowed, request)
	if !allowed.Flushed || allowed.Header().Get(HeaderPID) != "7" || allowed.Header().Get(HeaderBuildID) != "abc" {
		t.Fatalf("allowed response headers = %v flushed=%v", allowed.Header(), allowed.Flushed)
	}
}

func TestReadBearerIsReadOnlyAndRejectsSharedFiles(t *testing.T) {
	state := t.TempDir()
	if bearer, err := ReadBearer(state); err != nil || bearer != "" {
		t.Fatalf("missing bearer = %q, %v", bearer, err)
	}
	if _, err := os.Stat(httpDir(state)); !os.IsNotExist(err) {
		t.Fatalf("ReadBearer created state: %v", err)
	}
	if err := os.MkdirAll(httpDir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(httpDir(state), bearerFileName), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBearer(state); err == nil {
		t.Fatal("group-readable bearer accepted")
	}
}
