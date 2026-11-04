// Package status is the application service that assembles the harness status
// report from inspect, doctor, state, worker and self-verify observations.
package status

import (
	"context"

	selfaugmentapp "issueops/internal/application/selfaugment"
	doctorcontract "issueops/internal/contract/doctor"
	inspectcontract "issueops/internal/contract/inspect"
	selfaugmentcontract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	statuscontract "issueops/internal/contract/status"
	workercontract "issueops/internal/contract/worker"
	statusdomain "issueops/internal/domain/status"
	"strings"
)

// Service builds the harness status report by combining inspect, doctor, state
// and worker observations with the latest self-verify record. Every function
// field is required.
type Service struct {
	Home, IssueOpsRoot, Version string
	Inspect                     func(string) inspectcontract.Info
	Doctor                      func(context.Context, doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error)
	State                       func() (statecontract.ListResult, error)
	StateRead                   func(string) (statecontract.Result, error)
	Workers                     func() (workercontract.ListResult, error)
	ResolveTarget               func(string) string
}

// Run observes the harness for repo and returns the status result. Failures of
// the individual observations are folded into the result's warnings and OK
// flag by the status domain rather than returned as errors.
func (service Service) Run(ctx context.Context, repo string) statuscontract.Result {
	inspect := service.Inspect(repo)
	doctor, doctorErr := service.Doctor(ctx, doctorcontract.HarnessDoctorRequest{
		RepoRoot: repo, IssueOpsRoot: service.IssueOpsRoot, Home: service.Home, Version: service.Version,
	})
	state, stateErr := service.State()
	workers, workerErr := service.Workers()
	facts := statusdomain.Observation{
		Doctor: statusdomain.Outcome{OK: doctor.OK, Err: doctorErr}, State: statusdomain.Outcome{OK: state.OK, Err: stateErr}, Workers: statusdomain.Outcome{OK: workers.OK, Err: workerErr},
	}
	if stateErr == nil && state.OK {
		history, err := (selfaugmentapp.HistoryService{
			StateDir: func() string { return state.StateDir },
			List:     func() (statecontract.ListResult, error) { return state, nil },
			Read:     service.StateRead,
		}).History(ctx, "", 1, selfaugmentcontract.HistoryRetentionOptions{})
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
	return statuscontract.Result{OK: decision.OK, Kind: "harness_status", Version: service.Version, Repo: service.ResolveTarget(repo), Inspect: inspect, Doctor: doctor, State: state, Workers: workers, SelfVerify: decision.SelfVerify, Warnings: decision.Warnings}
}
