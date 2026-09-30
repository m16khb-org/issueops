package daemoncli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	daemoncontract "issueops/internal/contract/daemon"
	"net"
)

func serveDaemonConnectionWithAdmission(conn net.Conn, logFile LogFile, instance daemoncontract.InstanceRecord, admission *daemonAdmission, serveMCPStream func(context.Context, net.Conn, LogFile) error) error {
	session, admitted := admission.acquire()
	if admitted {
		defer func() {
			if session != nil {
				session.close()
			}
		}()
	} else if admission.reserveOverflowClassifier() {
		defer admission.releaseOverflowClassifier()
	} else {
		return rejectDaemonConnection(conn, logFile, admission.snapshot())
	}

	var first [1]byte
	if _, err := io.ReadFull(conn, first[:]); err != nil {
		return err
	}
	if first[0] != daemoncontract.IdentityRequest[0] {
		if !admitted {
			return rejectDaemonConnection(conn, logFile, admission.snapshot())
		}
		return serveMCPStream(session.Context, &daemonReplayConn{
			Conn:   conn,
			reader: io.MultiReader(bytes.NewReader(first[:]), conn),
		}, logFile)
	}
	rest := make([]byte, len(daemoncontract.IdentityRequest)-1)
	if _, err := io.ReadFull(conn, rest); err != nil {
		return err
	}
	if string(append(first[:], rest...)) != daemoncontract.IdentityRequest {
		return fmt.Errorf("invalid daemon identity request")
	}
	if session != nil {
		session.close()
		session = nil
	}
	snapshot := admission.snapshot()
	return json.NewEncoder(conn).Encode(daemoncontract.IdentityResponse{
		OK:                true,
		Instance:          instance,
		ActiveConnections: snapshot.ActiveConnections,
		MaxConnections:    snapshot.MaxConnections,
		Accepting:         snapshot.Accepting,
		Draining:          snapshot.Draining,
	})
}

func rejectDaemonConnection(conn net.Conn, logFile LogFile, status daemonAdmissionStatus) error {
	fmt.Fprintf(logFile, "daemon admission rejected code=%s active_connections=%d max_connections=%d draining=%t\n", daemonStatusConnectionLimit, status.ActiveConnections, status.MaxConnections, status.Draining)
	return writeDaemonAdmissionError(conn, status)
}

type daemonReplayConn struct {
	net.Conn
	reader io.Reader
}

func (c *daemonReplayConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}
