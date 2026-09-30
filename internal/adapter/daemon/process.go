package daemon

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	daemonapp "issueops/internal/application/daemon"
	daemoncontract "issueops/internal/contract/daemon"
	daemondomain "issueops/internal/domain/daemon"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Probe struct{ MaxConnections int }

func NewIdentityToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func ExecutableSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (p Probe) Status(socket string) (daemoncontract.IdentityResponse, error) {
	conn, err := net.DialTimeout("unix", socket, 150*time.Millisecond)
	if err != nil {
		return daemoncontract.IdentityResponse{}, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		return daemoncontract.IdentityResponse{}, err
	}
	if _, err := io.WriteString(conn, daemoncontract.IdentityRequest); err != nil {
		return daemoncontract.IdentityResponse{}, err
	}
	var response daemoncontract.IdentityResponse
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return daemoncontract.IdentityResponse{}, fmt.Errorf("decode daemon identity: %w", err)
	}
	if !response.OK {
		return daemoncontract.IdentityResponse{}, fmt.Errorf("daemon identity probe was rejected")
	}
	if err := daemondomain.ValidateInstance(response.Instance); err != nil {
		return daemoncontract.IdentityResponse{}, fmt.Errorf("invalid daemon identity: %w", err)
	}
	// admission health 이전 daemon은 이 additive 필드들을 생략한다. 그런 응답은
	// 과거의 고정 capacity, accepting 상태로 취급한다.
	if response.MaxConnections == 0 {
		response.MaxConnections = p.MaxConnections
		response.Accepting = true
	}
	return response, nil
}

type Launcher struct {
	Environment []string
	HarnessRoot string
}

func (l Launcher) Start(exe string, paths daemoncontract.Paths) error {
	cmd := exec.Command(exe, "daemon", "--internal")
	cmd.Env = append(append([]string(nil), l.Environment...), "ISSUEOPS_DAEMON_DIR="+paths.Dir, "ISSUEOPS_ROOT="+l.HarnessRoot)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap the owned child so an exited daemon cannot leave a zombie PID.
	go func() { _ = cmd.Wait() }()
	return nil
}

type processHandle struct{ *os.Process }

func (p processHandle) Terminate() error { return p.Signal(syscall.SIGTERM) }
func FindProcess(pid int) (daemonapp.Process, error) {
	p, err := os.FindProcess(pid)
	return processHandle{p}, err
}
func AcquireLock(paths daemoncontract.Paths) (io.Closer, error) {
	return acquireFileLock(paths.Lock, os.Getpid, ProcessAlive)
}
func EnsureDirectory(dir string) error { return os.MkdirAll(dir, 0700) }
func SleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
