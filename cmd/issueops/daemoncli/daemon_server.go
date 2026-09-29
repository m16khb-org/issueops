package daemoncli

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	contract "issueops/internal/contract/daemon"
	"issueops/internal/domain/daemonlog"
)

const (
	daemonAdmissionErrorCode    = -32001
	daemonStatusConnectionLimit = "daemon_connection_limit_reached"
)

type LogFile interface {
	io.Writer
	io.Closer
}

type Server struct {
	MaxConnections int
	IdleTimeout    time.Duration
	Paths          func() (contract.Paths, error)
	MkdirAll       func(string, os.FileMode) error
	OpenLog        func(string) (LogFile, error)
	Remove         func(string) error
	Listen         func(network, address string) (net.Listener, error)
	Chmod          func(string, os.FileMode) error
	WriteInstance  func(string, contract.InstanceRecord) error
	PID            func() int
	InspectProcess func(int) (contract.ProcessIdentity, error)
	BuildSHA       func(string) (string, error)
	NewToken       func() (string, error)
	Now            func() time.Time
	ServeMCPStream func(context.Context, net.Conn, LogFile) error
}

func (deps Server) Run() error {
	paths, err := deps.Paths()
	if err != nil {
		return err
	}
	if err := deps.MkdirAll(paths.Dir, 0o700); err != nil {
		return err
	}
	logFile, err := deps.OpenLog(paths.Log)
	if err != nil {
		return err
	}
	defer logFile.Close()
	_ = deps.Remove(paths.Socket)
	listener, err := deps.Listen("unix", paths.Socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer func() { _ = deps.Remove(paths.Socket) }()
	_ = deps.Chmod(paths.Socket, 0o600)
	pid := deps.PID()
	processIdentity, err := deps.InspectProcess(pid)
	if err != nil {
		return fmt.Errorf("inspect daemon process identity: %w", err)
	}
	nonce, err := deps.NewToken()
	if err != nil {
		return fmt.Errorf("create daemon instance nonce: %w", err)
	}
	generation, err := deps.NewToken()
	if err != nil {
		return fmt.Errorf("create daemon generation: %w", err)
	}
	buildSHA, err := deps.BuildSHA(processIdentity.Executable)
	if err != nil {
		return fmt.Errorf("hash daemon executable: %w", err)
	}
	instance := contract.InstanceRecord{
		PID:              pid,
		ProcessStartTime: processIdentity.StartTime,
		Executable:       processIdentity.Executable,
		InstanceNonce:    nonce,
		BuildSHA:         buildSHA,
		ProtocolVersion:  contract.ProtocolVersion,
		Generation:       generation,
	}
	if err := deps.WriteInstance(paths.PID, instance); err != nil {
		return err
	}
	_ = deps.Remove(paths.Lock)
	defer func() {
		_ = deps.Remove(paths.PID)
	}()
	admission := newDaemonAdmission(deps.MaxConnections)
	var activeWG sync.WaitGroup
	fmt.Fprintf(logFile, "%s daemon started pid=%d socket=%s max_connections=%d\n", deps.Now().Format(time.RFC3339), pid, paths.Socket, deps.MaxConnections)
	acceptErr := runDaemonAcceptLoop(listener, logFile, daemonServerLoopDeps{
		now: deps.Now,
		serveConnection: func(conn net.Conn, logFile LogFile) error {
			return serveDaemonConnectionWithAdmission(conn, logFile, instance, admission, func(ctx context.Context, conn net.Conn, logFile LogFile) error {
				return deps.ServeMCPStream(ctx, conn, logFile)
			})
		},
		wrapConn: func(c net.Conn) net.Conn {
			return &idleConn{Conn: c, timeout: deps.IdleTimeout}
		},
		activeWG: &activeWG,
	})
	fmt.Fprintf(logFile, "%s daemon stopping, waiting for active connections\n", deps.Now().Format(time.RFC3339))
	shutdownDone := make(chan struct{})
	go func() {
		activeWG.Wait()
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
		fmt.Fprintf(logFile, "%s daemon stopped cleanly\n", deps.Now().Format(time.RFC3339))
	case <-time.After(30 * time.Second):
		fmt.Fprintf(logFile, "%s daemon stopped with connections still active after 30s timeout\n", deps.Now().Format(time.RFC3339))
	}
	return acceptErr
}

type daemonServerLoopDeps struct {
	now             func() time.Time
	serveConnection func(net.Conn, LogFile) error
	wrapConn        func(net.Conn) net.Conn
	activeWG        *sync.WaitGroup
}

func runDaemonAcceptLoop(listener net.Listener, logFile LogFile, deps daemonServerLoopDeps) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			fmt.Fprintf(logFile, "%s accept error: %v\n", deps.now().Format(time.RFC3339), err)
			continue
		}
		deps.activeWG.Add(1)
		go func(conn net.Conn) {
			defer deps.activeWG.Done()
			defer conn.Close()
			if deps.wrapConn != nil {
				conn = deps.wrapConn(conn)
			}
			if err := deps.serveConnection(conn, logFile); err != nil && !daemonlog.IsRoutineShutdownError(err) {
				fmt.Fprintf(logFile, "%s connection error: %v\n", deps.now().Format(time.RFC3339), err)
			}
		}(conn)
	}
}

// idleConn은 net.Conn을 감싸 매 Read마다 read deadline을 갱신한다. 덕분에 버려진
// MCP 연결(트래픽 없음)은 idle timeout에 걸려 server.Run이 반환되고, 연결이
// 영원히 읽기에서 블록되는 대신 connSlot을 해제한다. 활성 연결은 매 Read마다
// deadline을 갱신하므로 영향받지 않는다.
type idleConn struct {
	net.Conn
	timeout time.Duration
}

func (c *idleConn) Read(p []byte) (int, error) {
	if err := c.Conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(p)
}
