package statuscli

import (
	adapter "issueops/internal/adapter/daemon"
	app "issueops/internal/application/daemon"
	domain "issueops/internal/domain/daemon"
	"os"
	"time"
)

func testDaemonReader() app.Reader {
	return app.Reader{Paths: adapter.Current, ReadInstance: adapter.ReadInstance, ProbeStatus: (adapter.Probe{MaxConnections: domain.DefaultMaxConnections}).Status, ProcessAlive: adapter.ProcessAlive, InspectProcess: adapter.InspectProcess, IsNotExist: os.IsNotExist, MaxConnections: domain.DefaultMaxConnections, Location: time.Local}
}
