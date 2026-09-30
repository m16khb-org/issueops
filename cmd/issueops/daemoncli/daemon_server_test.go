package daemoncli

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunDaemonServerWithDepsInitializesStateAndExitsOnClosedListener(t *testing.T) {
	dir := t.TempDir()
	paths := daemonPaths{
		Dir:    dir,
		Socket: filepath.Join(dir, "issueops.sock"),
		PID:    filepath.Join(dir, "issueops.pid"),
		Lock:   filepath.Join(dir, "issueops.lock"),
		Log:    filepath.Join(dir, "issueops.log"),
	}
	var log daemonServerFakeLog
	removed := []string{}
	var wroteInstance daemonInstance
	tokenCalls := 0

	err := (Server{MaxConnections: maxConnections, IdleTimeout: 30 * time.Minute,
		Paths: func() (daemonPaths, error) {
			return paths, nil
		},
		MkdirAll: func(path string, perm os.FileMode) error {
			if path != dir || perm != 0o700 {
				t.Fatalf("unexpected mkdir: %s %o", path, perm)
			}
			return nil
		},
		OpenLog: func(path string) (daemonServerLogFile, error) {
			if path != paths.Log {
				t.Fatalf("unexpected log path: %s", path)
			}
			return &log, nil
		},
		Remove: func(path string) error {
			removed = append(removed, path)
			return nil
		},
		Listen: func(network, address string) (net.Listener, error) {
			if network != "unix" || address != paths.Socket {
				t.Fatalf("unexpected listen target: %s %s", network, address)
			}
			return daemonServerClosedListener{}, nil
		},
		Chmod: func(path string, perm os.FileMode) error {
			if path != paths.Socket || perm != 0o600 {
				t.Fatalf("unexpected Chmod: %s %o", path, perm)
			}
			return nil
		},
		WriteInstance: func(path string, instance daemonInstance) error {
			if path != paths.PID {
				t.Fatalf("unexpected instance path: %s", path)
			}
			wroteInstance = instance
			return nil
		},
		PID: func() int {
			return 12345
		},
		InspectProcess: func(pid int) (daemonProcessIdentity, error) {
			if pid != 12345 {
				t.Fatalf("unexpected inspected pid: %d", pid)
			}
			return daemonProcessIdentity{StartTime: "start-a", Executable: "/tmp/issueops"}, nil
		},
		BuildSHA: func(executable string) (string, error) {
			if executable != "/tmp/issueops" {
				t.Fatalf("unexpected executable hash target: %s", executable)
			}
			return "build-a", nil
		},
		NewToken: func() (string, error) {
			tokenCalls++
			if tokenCalls == 1 {
				return "nonce-a", nil
			}
			return "generation-a", nil
		},
		Now: func() time.Time {
			return time.Unix(100, 0).UTC()
		},
		ServeMCPStream: func(context.Context, net.Conn, daemonServerLogFile) error {
			t.Fatal("closed listener should not serve MCP streams")
			return nil
		},
	}).Run()

	if err != nil {
		t.Fatalf("expected closed listener to stop cleanly, got %v", err)
	}
	wantInstance := daemonInstance{
		PID:              12345,
		ProcessStartTime: "start-a",
		Executable:       "/tmp/issueops",
		InstanceNonce:    "nonce-a",
		BuildSHA:         "build-a",
		ProtocolVersion:  daemonProtocolVersion,
		Generation:       "generation-a",
	}
	if wroteInstance != wantInstance {
		t.Fatalf("unexpected instance write: %#v", wroteInstance)
	}
	if !strings.Contains(log.String(), "daemon started pid=12345 socket="+paths.Socket) {
		t.Fatalf("missing start log: %q", log.String())
	}
	if !containsDaemonServerString(removed, paths.Socket) || !containsDaemonServerString(removed, paths.Lock) || !containsDaemonServerString(removed, paths.PID) {
		t.Fatalf("expected socket/lock/pid cleanup, got %v", removed)
	}
}

func TestRunDaemonServerWithDepsReturnsSetupErrors(t *testing.T) {
	setupErr := errors.New("paths failed")
	err := (Server{MaxConnections: maxConnections, IdleTimeout: 30 * time.Minute,
		Paths: func() (daemonPaths, error) {
			return daemonPaths{}, setupErr
		},
	}).Run()
	if !errors.Is(err, setupErr) {
		t.Fatalf("expected paths error, got %v", err)
	}

	listenErr := errors.New("listen failed")
	err = (Server{MaxConnections: maxConnections, IdleTimeout: 30 * time.Minute,
		Paths: func() (daemonPaths, error) {
			return daemonPaths{Dir: t.TempDir(), Socket: "daemon.sock", Log: "daemon.log"}, nil
		},
		MkdirAll: func(string, os.FileMode) error { return nil },
		OpenLog: func(string) (daemonServerLogFile, error) {
			return &daemonServerFakeLog{}, nil
		},
		Remove: func(string) error { return nil },
		Listen: func(string, string) (net.Listener, error) {
			return nil, listenErr
		},
	}).Run()
	if !errors.Is(err, listenErr) {
		t.Fatalf("expected listen error, got %v", err)
	}
}
