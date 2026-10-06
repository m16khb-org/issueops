package mcpservice

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"issueops/internal/contract/mcpservice"
	"issueops/internal/contract/processidentity"
)

const (
	CodeSupervisorUnsupported = "supervisor_unsupported"
	CodeSupervisorUnavailable = "supervisor_unavailable"
	CodeSupervisorFailed      = "supervisor_failed"
	CodePortInUse             = "port_in_use"
	CodeIdentityMismatch      = "identity_mismatch"
	CodeStaleRecord           = "stale_record"
	CodeNotReady              = "not_ready"
	CodeNotSupervised         = "not_supervised"
	CodeBuildMismatch         = "build_mismatch"
	CodeReadinessTimeout      = "readiness_timeout"
	CodeStopTimeout           = "stop_timeout"
	CodeStateUnreadable       = "state_unreadable"
	CodeCredentialFailed      = "credential_failed"
)

type Config struct {
	GOOS     string
	Home     string
	StateDir string
	Root     string
	Binary   string
	Address  string
	Path     string
	Label    string
	UnitName string
	UID      int

	Run          Runner
	LookPath     func(string) (string, error)
	Inspect      func(int) (processidentity.Identity, error)
	EnsureBearer func() (string, error)
	LockHolder   func(string) (int, error)
	Client       *http.Client
	Wait         func(context.Context, time.Duration) error

	ReadyTimeout time.Duration
	StopTimeout  time.Duration
	PollInterval time.Duration
}

// Service implements port.MCPService over a per-user OS supervisor
// (launchd on darwin, systemd --user on linux).
type Service struct {
	cfg        Config
	supervisor supervisor
}

func New(cfg Config) *Service {
	if cfg.LockHolder == nil {
		cfg.LockHolder = LockHolder
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 3 * time.Second}
	}
	if cfg.Wait == nil {
		cfg.Wait = waitContext
	}
	if cfg.ReadyTimeout == 0 {
		cfg.ReadyTimeout = 20 * time.Second
	}
	if cfg.StopTimeout == 0 {
		cfg.StopTimeout = 20 * time.Second
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 100 * time.Millisecond
	}
	service := &Service{cfg: cfg}
	switch cfg.GOOS {
	case "darwin":
		service.supervisor = launchd{run: cfg.Run, label: cfg.Label, uid: cfg.UID, home: cfg.Home}
	case "linux":
		service.supervisor = systemd{run: cfg.Run, unit: cfg.UnitName, home: cfg.Home}
	}
	return service
}

func (s *Service) url() string { return "http://" + s.cfg.Address + s.cfg.Path }

func (s *Service) result(state string, pid int, buildID, code string) mcpservice.Status {
	return mcpservice.Status{OK: code == "", Status: state, PID: pid, BuildID: buildID, URL: s.url(), ErrorCode: code}
}

func (s *Service) fail(status mcpservice.Status, code string, err error) (mcpservice.Status, error) {
	status.OK = false
	status.ErrorCode = code
	return status, err
}

func (s *Service) requireSupervisor() (mcpservice.Status, error) {
	if s.supervisor == nil {
		return s.fail(s.result("", 0, "", ""), CodeSupervisorUnsupported, fmt.Errorf("issueops mcp service has no supported supervisor on %s; run `issueops mcp --http` in the foreground or install with --mcp-transport=stdio", s.cfg.GOOS))
	}
	if _, err := s.cfg.LookPath(s.supervisor.command()); err != nil {
		return s.fail(s.result("", 0, "", ""), CodeSupervisorUnavailable, fmt.Errorf("issueops mcp service supervisor %s is unavailable: %w", s.supervisor.command(), err))
	}
	return mcpservice.Status{}, nil
}

func (s *Service) dir() string        { return httpDir(s.cfg.StateDir) }
func (s *Service) lockPath() string   { return filepath.Join(s.dir(), lockFileName) }
func (s *Service) recordPath() string { return filepath.Join(s.dir(), recordFileName) }

type observation struct {
	status mcpservice.Status
	holder int
	err    error
}

// observe classifies the service without changing it. Running requires the
// lock holder, the published record, the OS process identity and an
// authenticated MCP response to agree on pid, started_at, executable and build.
func (s *Service) observe(ctx context.Context) observation {
	holder, err := s.cfg.LockHolder(s.lockPath())
	if err != nil {
		status, err := s.fail(s.result("", 0, "", ""), CodeStateUnreadable, fmt.Errorf("read mcp http lock: %w", err))
		return observation{status: status, err: err}
	}
	record, recordErr := readRecord(s.recordPath())
	recordMissing := errors.Is(recordErr, os.ErrNotExist)
	if holder == 0 {
		if !recordMissing {
			status, err := s.fail(s.result(mcpservice.StatusStale, record.PID, record.BuildID, ""), CodeStaleRecord, fmt.Errorf("mcp http instance record names a process that no longer holds the service lock"))
			return observation{status: status, err: err}
		}
		if s.portInUse() {
			status, err := s.fail(s.result(mcpservice.StatusConflict, 0, "", ""), CodePortInUse, fmt.Errorf("%s is held by a process outside the issueops service lock (conflict)", s.cfg.Address))
			return observation{status: status, err: err}
		}
		return observation{status: s.result(mcpservice.StatusStopped, 0, "", "")}
	}
	mismatch := func(detail string) observation {
		status, err := s.fail(s.result(mcpservice.StatusConflict, holder, "", ""), CodeIdentityMismatch, fmt.Errorf("mcp http lock holder %d does not match the published instance: %s", holder, detail))
		return observation{status: status, holder: holder, err: err}
	}
	if recordErr != nil {
		return mismatch("instance record unavailable")
	}
	if record.PID != holder {
		return mismatch("record pid " + strconv.Itoa(record.PID))
	}
	identity, err := s.cfg.Inspect(holder)
	if err != nil {
		return mismatch("process identity unavailable")
	}
	if identity.StartTime != record.StartedAt || identity.Executable != record.Executable {
		return mismatch("started_at or executable changed")
	}
	pid, buildID, err := s.probe(ctx)
	if err != nil {
		status, err := s.fail(s.result(mcpservice.StatusStale, holder, record.BuildID, ""), CodeNotReady, fmt.Errorf("mcp http instance %d holds the lock but did not answer an authenticated MCP request: %w", holder, err))
		return observation{status: status, holder: holder, err: err}
	}
	if pid != holder || buildID != record.BuildID {
		return mismatch("authenticated response pid " + strconv.Itoa(pid) + " or build_id differs from the record")
	}
	return observation{status: s.result(mcpservice.StatusRunning, holder, buildID, ""), holder: holder}
}

func (s *Service) portInUse() bool {
	conn, err := net.DialTimeout("tcp", s.cfg.Address, 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (s *Service) probe(ctx context.Context) (int, string, error) {
	bearer, err := ReadBearer(s.cfg.StateDir)
	if err != nil {
		return 0, "", err
	}
	if bearer == "" {
		return 0, "", fmt.Errorf("mcp http bearer is not installed")
	}
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"issueops-service-probe","version":"1"}}}`
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url(), strings.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	request.Header.Set("Authorization", "Bearer "+bearer)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response, err := s.cfg.Client.Do(request)
	if err != nil {
		return 0, "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("initialize returned HTTP %d", response.StatusCode)
	}
	if err := checkInitializeResult(response); err != nil {
		return 0, "", err
	}
	pid, err := strconv.Atoi(response.Header.Get(HeaderPID))
	if err != nil {
		return 0, "", fmt.Errorf("initialize response has no %s header", HeaderPID)
	}
	return pid, response.Header.Get(HeaderBuildID), nil
}

func checkInitializeResult(response *http.Response) error {
	var payload []byte
	if strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 64*1024), 1<<20)
		for scanner.Scan() {
			if data, ok := strings.CutPrefix(scanner.Text(), "data:"); ok {
				payload = []byte(strings.TrimSpace(data))
				break
			}
		}
	} else {
		var buffer bytes.Buffer
		if _, err := buffer.ReadFrom(io.LimitReader(response.Body, 1<<20)); err != nil {
			return err
		}
		payload = buffer.Bytes()
	}
	var message struct {
		Result struct {
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &message); err != nil {
		return fmt.Errorf("decode initialize response: %w", err)
	}
	if message.Result.ServerInfo.Name != "issueops" {
		return fmt.Errorf("initialize response did not come from issueops")
	}
	return nil
}

func (s *Service) Status(ctx context.Context) (mcpservice.Status, error) {
	if status, err := s.requireSupervisor(); err != nil {
		return status, err
	}
	observed := s.observe(ctx)
	return observed.status, observed.err
}

// Prepare installs the credential and the supervisor unit without loading it.
// It returns the bearer so the installer can merge host configs afterwards.
func (s *Service) Prepare(context.Context) (string, error) {
	if _, err := s.requireSupervisor(); err != nil {
		return "", err
	}
	bearer, err := s.cfg.EnsureBearer()
	if err != nil {
		return "", fmt.Errorf("prepare mcp http credential: %w", err)
	}
	logPath := filepath.Join(s.dir(), "server.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", fmt.Errorf("prepare mcp http log: %w", err)
	}
	if err := logFile.Close(); err != nil {
		return "", err
	}
	path := s.supervisor.unitPath()
	unit := s.supervisor.renderUnit(unitSpec{Binary: s.cfg.Binary, Root: s.cfg.Root, StateDir: s.cfg.StateDir, LogPath: logPath})
	if existing, err := os.ReadFile(path); err == nil && string(existing) == unit {
		return bearer, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
		return "", err
	}
	return bearer, nil
}

func (s *Service) Start(ctx context.Context) (mcpservice.Status, error) {
	if status, err := s.requireSupervisor(); err != nil {
		return status, err
	}
	want, err := BuildID(s.cfg.Binary)
	if err != nil {
		return s.fail(s.result("", 0, "", ""), CodeBuildMismatch, fmt.Errorf("read service binary %s: %w", s.cfg.Binary, err))
	}
	observed := s.observe(ctx)
	switch observed.status.Status {
	case mcpservice.StatusRunning:
		if observed.status.BuildID != want {
			return s.fail(observed.status, CodeBuildMismatch, fmt.Errorf("mcp http service runs build %s, binary is %s; stop it first", observed.status.BuildID, want))
		}
		return observed.status, nil
	case mcpservice.StatusConflict:
		return observed.status, observed.err
	case mcpservice.StatusStale:
		if observed.holder != 0 {
			return observed.status, observed.err
		}
		if err := os.Remove(s.recordPath()); err != nil && !os.IsNotExist(err) {
			return s.fail(observed.status, CodeStateUnreadable, err)
		}
	case mcpservice.StatusStopped:
	default:
		return observed.status, observed.err
	}
	if _, err := s.Prepare(ctx); err != nil {
		return s.fail(s.result(mcpservice.StatusStopped, 0, "", ""), CodeCredentialFailed, err)
	}
	if loaded, _, err := s.supervisor.supervised(ctx); err != nil {
		return s.fail(s.result(mcpservice.StatusStopped, 0, "", ""), CodeSupervisorFailed, err)
	} else if loaded {
		if err := s.supervisor.unload(ctx); err != nil {
			return s.fail(s.result(mcpservice.StatusStopped, 0, "", ""), CodeSupervisorFailed, fmt.Errorf("unload idle mcp service job: %w", err))
		}
	}
	if err := s.supervisor.load(ctx); err != nil {
		return s.fail(s.result(mcpservice.StatusStopped, 0, "", ""), CodeSupervisorFailed, fmt.Errorf("load mcp service job: %w", err))
	}
	status, err := s.awaitRunning(ctx, want)
	if err != nil {
		_ = s.supervisor.unload(context.WithoutCancel(ctx))
	}
	return status, err
}

func (s *Service) awaitRunning(ctx context.Context, want string) (mcpservice.Status, error) {
	deadline, cancel := context.WithTimeout(ctx, s.cfg.ReadyTimeout)
	defer cancel()
	last := s.observe(deadline)
	for last.status.Status != mcpservice.StatusRunning {
		if err := s.cfg.Wait(deadline, s.cfg.PollInterval); err != nil {
			return s.fail(last.status, CodeReadinessTimeout, fmt.Errorf("mcp http service did not become ready within %s: %w", s.cfg.ReadyTimeout, errors.Join(err, last.err)))
		}
		last = s.observe(deadline)
	}
	if last.status.BuildID != want {
		return s.fail(last.status, CodeBuildMismatch, fmt.Errorf("mcp http service started build %s, want %s", last.status.BuildID, want))
	}
	return last.status, nil
}

func (s *Service) Stop(ctx context.Context) (mcpservice.Status, error) {
	if status, err := s.requireSupervisor(); err != nil {
		return status, err
	}
	observed := s.observe(ctx)
	switch observed.status.Status {
	case mcpservice.StatusStopped:
		return s.unloadIdle(ctx, observed.status)
	case mcpservice.StatusStale:
		if observed.holder != 0 {
			return observed.status, observed.err
		}
		if err := os.Remove(s.recordPath()); err != nil && !os.IsNotExist(err) {
			return s.fail(observed.status, CodeStateUnreadable, err)
		}
		return s.unloadIdle(ctx, s.result(mcpservice.StatusStopped, 0, "", ""))
	case mcpservice.StatusRunning:
	default:
		return observed.status, observed.err
	}
	loaded, supervisedPID, err := s.supervisor.supervised(ctx)
	if err != nil {
		return s.fail(observed.status, CodeSupervisorFailed, err)
	}
	if !loaded || supervisedPID != observed.status.PID {
		return s.fail(observed.status, CodeNotSupervised, fmt.Errorf("mcp http instance %d is not the process supervised by %s; refusing to stop it", observed.status.PID, s.supervisor.command()))
	}
	if err := s.supervisor.unload(ctx); err != nil {
		return s.fail(observed.status, CodeSupervisorFailed, fmt.Errorf("unload mcp service job: %w", err))
	}
	deadline, cancel := context.WithTimeout(ctx, s.cfg.StopTimeout)
	defer cancel()
	for {
		holder, err := s.cfg.LockHolder(s.lockPath())
		if err != nil {
			return s.fail(observed.status, CodeStateUnreadable, err)
		}
		if holder == 0 {
			break
		}
		if err := s.cfg.Wait(deadline, s.cfg.PollInterval); err != nil {
			return s.fail(observed.status, CodeStopTimeout, fmt.Errorf("mcp http instance %d still holds the service lock: %w", holder, err))
		}
	}
	if record, err := readRecord(s.recordPath()); err == nil && record.PID == observed.status.PID {
		_ = os.Remove(s.recordPath())
	}
	return s.result(mcpservice.StatusStopped, 0, "", ""), nil
}

func (s *Service) unloadIdle(ctx context.Context, status mcpservice.Status) (mcpservice.Status, error) {
	loaded, _, err := s.supervisor.supervised(ctx)
	if err != nil {
		return s.fail(status, CodeSupervisorFailed, err)
	}
	if loaded {
		if err := s.supervisor.unload(ctx); err != nil {
			return s.fail(status, CodeSupervisorFailed, fmt.Errorf("unload idle mcp service job: %w", err))
		}
	}
	return status, nil
}

// EnsureRunning brings the service to the given binary's build: a running
// older build is stopped (identity-checked) and the target build started.
func (s *Service) EnsureRunning(ctx context.Context, binary string) (mcpservice.Status, error) {
	if filepath.Clean(binary) != filepath.Clean(s.cfg.Binary) {
		return s.fail(s.result("", 0, "", ""), CodeBuildMismatch, fmt.Errorf("mcp service binary %s differs from install target %s", s.cfg.Binary, binary))
	}
	status, err := s.Start(ctx)
	if err == nil || status.ErrorCode != CodeBuildMismatch || status.Status != mcpservice.StatusRunning {
		return status, err
	}
	if status, err := s.Stop(ctx); err != nil {
		return status, err
	}
	return s.Start(ctx)
}

func waitContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
