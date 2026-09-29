package daemon

import (
	"fmt"
	daemoncontract "issueops/internal/contract/daemon"
	daemondomain "issueops/internal/domain/daemon"
	"time"
)

type Reader struct {
	Paths          func() (daemoncontract.Paths, error)
	ReadInstance   func(string) (daemoncontract.InstanceRecord, error)
	ProbeStatus    func(string) (daemoncontract.IdentityResponse, error)
	ProcessAlive   func(int) bool
	InspectProcess func(int) (daemoncontract.ProcessIdentity, error)
	IsNotExist     func(error) bool
	MaxConnections int
	Location       *time.Location
}

func (deps Reader) Run() daemoncontract.Status {
	paths, err := deps.Paths()
	if err != nil {
		return daemoncontract.Status{OK: false, Code: daemoncontract.StatusInstanceUnreadable, MaxConnections: deps.MaxConnections, Message: err.Error()}
	}
	status := daemoncontract.Status{OK: true, Code: daemoncontract.StatusStopped, Paths: paths, MaxConnections: deps.MaxConnections, Message: "daemon is not running"}
	record, readErr := deps.ReadInstance(paths.PID)
	var probe daemoncontract.IdentityResponse
	probed := false
	if readErr != nil {
		var probeErr error
		probe, probeErr = deps.ProbeStatus(paths.Socket)
		if probeErr == nil {
			probed = true
			record, readErr = deps.ReadInstance(paths.PID)
		}
		if readErr != nil && probeErr == nil {
			observed := probe.Instance
			status.Running = true
			status.Reachable = true
			status.PID = observed.PID
			status.Instance = &observed
			applyAdmissionStatus(&status, probe)
			return daemondomain.IdentityMismatchStatus(status, "daemon socket is reachable without a matching instance record")
		}
		if readErr != nil && !deps.IsNotExist(readErr) {
			status.OK = false
			status.Code = daemoncontract.StatusInstanceUnreadable
			status.Message = readErr.Error()
		}
		if readErr != nil {
			return status
		}
	}
	status.PID = record.PID
	status.Instance = &record

	if !probed {
		var probeErr error
		probe, probeErr = deps.ProbeStatus(paths.Socket)
		if probeErr != nil {
			alive := record.PID > 0 && deps.ProcessAlive(record.PID)
			status.Running = alive
			if alive {
				status.OK = false
				status.Code = daemoncontract.StatusSocketUnreachable
				status.Message = "daemon pid exists but socket is not reachable"
			}
			return status
		}
	}
	observed := probe.Instance
	status.Running = true
	status.Reachable = true
	applyAdmissionStatus(&status, probe)
	if observed != record {
		return daemondomain.IdentityMismatchStatus(status, "daemon socket identity does not match instance record")
	}
	if !deps.ProcessAlive(record.PID) {
		return daemondomain.IdentityMismatchStatus(status, "daemon process is not alive")
	}
	process, err := deps.InspectProcess(record.PID)
	if err != nil {
		return daemondomain.IdentityMismatchStatus(status, fmt.Sprintf("inspect daemon process identity: %v", err))
	}
	if !daemondomain.ProcessIdentityMatches(record, process, deps.Location) {
		return daemondomain.IdentityMismatchStatus(status, "daemon OS process identity does not match instance record")
	}
	status.OK = true
	status.IdentityVerified = true
	status.Code = daemoncontract.StatusReady
	status.Message = "daemon is reachable and identity verified"
	return status
}
func applyAdmissionStatus(status *daemoncontract.Status, probe daemoncontract.IdentityResponse) {
	status.ActiveConnections = probe.ActiveConnections
	status.MaxConnections = probe.MaxConnections
	status.Accepting = probe.Accepting
	status.Draining = probe.Draining
}
