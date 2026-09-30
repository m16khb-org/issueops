package issueopsapp

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/daemoncli"
	adapter "issueops/internal/adapter/daemon"
	contract "issueops/internal/contract/daemon"
)

func TestDaemonServersKeepCapturedMCPAndAdmission(t *testing.T) {
	var servers [2]daemoncli.Server
	var dirs [2]string
	for i := range servers {
		root, err := os.MkdirTemp("/tmp", "io-daemon-instance-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(root) })
		dirs[i] = filepath.Join(root, "daemon")
		t.Setenv("HOME", root)
		t.Setenv("ISSUEOPS_ROOT", root)
		t.Setenv("ISSUEOPS_STATE_DIR", filepath.Join(root, "state"))
		t.Setenv("ISSUEOPS_WORKER_DIR", filepath.Join(root, "worker"))
		t.Setenv("ISSUEOPS_DAEMON_DIR", dirs[i])
		t.Setenv("ISSUEOPS_DAEMON_MAX_CONNECTIONS", strconv.Itoa(i+1))
		t.Setenv("ISSUEOPS_MCP_IDLE_TIMEOUT", strconv.Itoa(i+1)+"m")
		servers[i] = newDaemonServer(newDaemonReader(), issueOpsMCPDependencies())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i, server := range servers {
		if server.IdleTimeout != time.Duration(i+1)*time.Minute {
			t.Fatalf("idle configuration leaked: %v", server.IdleTimeout)
		}
		listening := make(chan net.Listener, 1)
		server.Listen = func(network, address string) (net.Listener, error) {
			listener, err := net.Listen(network, address)
			if err == nil {
				listening <- listener
			}
			return listener, err
		}
		done := make(chan error, 1)
		go func() { done <- server.Run() }()
		var listener net.Listener
		select {
		case listener = <-listening:
		case err := <-done:
			t.Fatalf("server setup failed: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		t.Cleanup(func() {
			_ = listener.Close()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("server exit: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Error("server did not exit")
			}
		})
		conn, err := net.Dial("unix", filepath.Join(dirs[i], "issueops.sock"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		session, err := mcp.NewClient(&mcp.Implementation{Name: "daemon-instance-test", Version: "1"}, nil).Connect(ctx, &mcp.IOTransport{Reader: conn, Writer: conn}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "daemon_status", Arguments: map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			t.Fatalf("daemon status failed: %+v", result)
		}
		var status contract.Status
		if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &status); err != nil {
			t.Fatal(err)
		}
		if status.Paths.Dir != dirs[i] || status.MaxConnections != i+1 || !status.IdentityVerified || status.ActiveConnections != 1 || status.Accepting != (i > 0) {
			t.Fatalf("server %d lost its MCP/admission context: %+v", i, status)
		}
		// The private probe remains usable even when the first server's single slot is occupied.
		probe, err := (adapter.Probe{MaxConnections: 99}).Status(status.Paths.Socket)
		if err != nil || probe.MaxConnections != i+1 || probe.ActiveConnections != 1 || probe.Accepting != (i > 0) {
			t.Fatalf("server %d admission: %+v %v", i, probe, err)
		}
	}
}

func TestDaemonCommandKeepsCapturedStatusContext(t *testing.T) {
	var commands [2]daemoncli.Command
	var dirs [2]string
	for i := range commands {
		dirs[i] = filepath.Join(t.TempDir(), "daemon")
		t.Setenv("ISSUEOPS_DAEMON_DIR", dirs[i])
		t.Setenv("ISSUEOPS_DAEMON_MAX_CONNECTIONS", strconv.Itoa(i+2))
		commands[i] = newDaemonCommand()
	}
	for i, command := range commands {
		status := command.Status()
		if status.Paths.Dir != dirs[i] || status.MaxConnections != i+2 {
			t.Fatalf("command %d leaked: %+v", i, status)
		}
		if _, err := os.Stat(dirs[i]); !os.IsNotExist(err) {
			t.Fatalf("status materialized directory: %v", err)
		}
	}
}

func TestDaemonServerForwardsMCPContextCancellation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
	t.Setenv("ISSUEOPS_STATE_DIR", filepath.Join(root, "state"))
	t.Setenv("ISSUEOPS_DAEMON_DIR", filepath.Join(root, "daemon"))
	daemon := newDaemonServer(newDaemonReader(), issueOpsMCPDependencies())
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	log, err := os.CreateTemp(root, "daemon-log-")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- daemon.ServeMCPStream(ctx, server, log) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("root-composed daemon MCP callback ignored session cancellation")
	}
}
