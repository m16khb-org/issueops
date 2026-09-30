package status

import (
	daemoncontract "issueops/internal/contract/daemon"
	doctorcontract "issueops/internal/contract/doctor"
	inspectcontract "issueops/internal/contract/inspect"
	statecontract "issueops/internal/contract/state"
	statuscontract "issueops/internal/contract/status"
	workercontract "issueops/internal/contract/worker"
	statusdomain "issueops/internal/domain/status"
)

type Service struct {
	Home, IssueOpsRoot, Version string
	Inspect                     func(string) inspectcontract.InspectInfo
	Daemon                      func() daemoncontract.Status
	Doctor                      func(doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error)
	State                       func() (statecontract.StateListResult, error)
	Workers                     func() (workercontract.WorkerListResult, error)
	ResolveTarget               func(string) string
}

func (service Service) Run(repo string) statuscontract.Result {
	inspect := service.Inspect(repo)
	daemon := service.Daemon()
	doctor, doctorErr := service.Doctor(doctorcontract.HarnessDoctorRequest{
		RepoRoot: repo, IssueOpsRoot: service.IssueOpsRoot, Home: service.Home, Version: service.Version,
		DaemonAdmission: doctorcontract.HarnessDoctorDaemonAdmission{
			Observed:          statusdomain.DaemonAdmissionObserved(daemon.Running, daemon.Reachable, daemon.IdentityVerified),
			ActiveConnections: daemon.ActiveConnections, MaxConnections: daemon.MaxConnections,
			Accepting: daemon.Accepting, Draining: daemon.Draining,
		},
	})
	state, stateErr := service.State()
	workers, workerErr := service.Workers()
	records := make([]statusdomain.Record, 0, len(state.Records))
	for _, record := range state.Records {
		records = append(records, statusdomain.Record{Key: record.Key, UpdatedAt: record.UpdatedAt, Bytes: record.Bytes})
	}
	decision := statusdomain.Evaluate(statusdomain.Observation{
		Doctor: statusdomain.Outcome{OK: doctor.OK, Err: doctorErr}, State: statusdomain.Outcome{OK: state.OK, Err: stateErr}, Workers: statusdomain.Outcome{OK: workers.OK, Err: workerErr}, Records: records,
	})
	return statuscontract.Result{OK: decision.OK, Kind: "harness_status", Version: service.Version, Repo: service.ResolveTarget(repo), Inspect: inspect, Doctor: doctor, Daemon: daemon, State: state, Workers: workers, SelfVerify: decision.SelfVerify, Warnings: decision.Warnings}
}
