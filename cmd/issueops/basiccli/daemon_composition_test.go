package basiccli

import (
	adapter "issueops/internal/adapter/daemon"
	app "issueops/internal/application/daemon"
	domain "issueops/internal/domain/daemon"
	"os"
	"os/exec"
	"time"
)

func testDaemonReader() app.Reader {
	return app.Reader{Paths: adapter.Current, ReadInstance: adapter.ReadInstance, ProbeStatus: (adapter.Probe{MaxConnections: domain.DefaultMaxConnections}).Status, ProcessAlive: adapter.ProcessAlive, InspectProcess: testProcessInspector().Inspect, IsNotExist: os.IsNotExist, MaxConnections: domain.DefaultMaxConnections, Location: time.Local}
}

func testProcessInspector() adapter.ProcessInspector {
	ps, err := exec.LookPath("ps")
	return adapter.ProcessInspector{PSExecutable: ps, PSLookupError: err, Environment: os.Environ()}
}
