package status

import (
	selfaugmentapp "issueops/internal/application/selfaugment"
	daemoncontract "issueops/internal/contract/daemon"
	doctorcontract "issueops/internal/contract/doctor"
	inspectcontract "issueops/internal/contract/inspect"
	selfaugmentcontract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	statuscontract "issueops/internal/contract/status"
	workercontract "issueops/internal/contract/worker"
	statusdomain "issueops/internal/domain/status"
	"strings"
)

type Service struct {
	Home, IssueOpsRoot, Version string
	Inspect                     func(string) inspectcontract.InspectInfo
	Daemon                      func() daemoncontract.Status
	Doctor                      func(doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error)
	State                       func() (statecontract.StateListResult, error)
	StateRead                   func(string) (statecontract.StateResult, error)
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
	facts := statusdomain.Observation{
		Doctor: statusdomain.Outcome{OK: doctor.OK, Err: doctorErr}, State: statusdomain.Outcome{OK: state.OK, Err: stateErr}, Workers: statusdomain.Outcome{OK: workers.OK, Err: workerErr},
	}
	if stateErr == nil && state.OK {
		history, err := (selfaugmentapp.HistoryService{
			StateDir: func() string { return state.StateDir },
			List:     func() (statecontract.StateListResult, error) { return state, nil },
			Read:     service.StateRead,
		}).History("", 1, selfaugmentcontract.SelfAugmentHistoryRetentionOptions{})
		if err != nil {
			facts.SelfVerifyReadFailed = true
			facts.SelfVerifyWarnings = append(facts.SelfVerifyWarnings, err.Error())
		}
		facts.SelfVerifyWarnings = append(facts.SelfVerifyWarnings, history.Warnings...)
		for _, skipped := range history.Skipped {
			if strings.HasPrefix(skipped.Reason, "state_read:") {
				facts.SelfVerifyReadFailed = true
				facts.SelfVerifyWarnings = append(facts.SelfVerifyWarnings, skipped.Key+": "+skipped.Reason)
			}
		}
		if len(history.Entries) > 0 {
			entry := history.Entries[0]
			facts.LatestSelfVerify = &statusdomain.Record{Key: entry.Key, UpdatedAt: entry.UpdatedAt, Bytes: entry.Bytes}
		}
	}
	decision := statusdomain.Evaluate(facts)
	return statuscontract.Result{OK: decision.OK, Kind: "harness_status", Version: service.Version, Repo: service.ResolveTarget(repo), Inspect: inspect, Doctor: doctor, Daemon: daemon, State: state, Workers: workers, SelfVerify: decision.SelfVerify, Warnings: decision.Warnings}
}
