package status

import (
	"errors"
	daemoncontract "issueops/internal/contract/daemon"
	doctorcontract "issueops/internal/contract/doctor"
	inspectcontract "issueops/internal/contract/inspect"
	statecontract "issueops/internal/contract/state"
	workercontract "issueops/internal/contract/worker"
	"reflect"
	"testing"
)

func TestRunPreservesObservationOrderAndContinuesAfterErrors(t *testing.T) {
	var calls []string
	daemon := daemoncontract.Status{Running: true, Reachable: true, IdentityVerified: true, ActiveConnections: 7, MaxConnections: 8, Accepting: true}
	service := Service{
		Home: "home", IssueOpsRoot: "harness", Version: "version",
		Inspect: func(repo string) inspectcontract.InspectInfo {
			calls = append(calls, "inspect:"+repo)
			return inspectcontract.InspectInfo{Version: "observed"}
		},
		Daemon: func() daemoncontract.Status { calls = append(calls, "daemon"); return daemon },
		Doctor: func(req doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error) {
			calls = append(calls, "doctor:"+req.RepoRoot)
			if req.RepoRoot != "raw/../repo" || req.Home != "home" || req.IssueOpsRoot != "harness" || req.Version != "version" || !req.DaemonAdmission.Observed || req.DaemonAdmission.ActiveConnections != 7 || req.DaemonAdmission.MaxConnections != 8 || !req.DaemonAdmission.Accepting {
				t.Fatalf("doctor request: %+v", req)
			}
			return doctorcontract.HarnessDoctorResult{OK: true, Healthy: false}, errors.New("")
		},
		State: func() (statecontract.StateListResult, error) {
			calls = append(calls, "state")
			return statecontract.StateListResult{OK: true}, errors.New("state failed")
		},
		Workers: func() (workercontract.WorkerListResult, error) {
			calls = append(calls, "workers")
			return workercontract.WorkerListResult{OK: true}, errors.New("worker failed")
		},
		ResolveTarget: func(repo string) string { calls = append(calls, "resolve:"+repo); return "resolved" },
	}
	got := service.Run("raw/../repo")
	wantCalls := []string{"inspect:raw/../repo", "daemon", "doctor:raw/../repo", "state", "workers", "resolve:raw/../repo"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls: %v", calls)
	}
	if got.OK || got.Kind != "harness_status" || got.Repo != "resolved" || got.Version != "version" || got.Inspect.Version != "observed" || got.Daemon != daemon || !reflect.DeepEqual(got.Warnings, []string{"doctor: ", "state: state failed", "workers: worker failed"}) {
		t.Fatalf("result: %+v", got)
	}
}

func TestRunAdmissionRequiresAllIdentityFactsAndOKDoesNotRequireHealth(t *testing.T) {
	for _, d := range []daemoncontract.Status{{Running: true, Reachable: true}, {Running: true, IdentityVerified: true}, {Reachable: true, IdentityVerified: true}, {Running: true, Reachable: true, IdentityVerified: true}} {
		service := Service{
			Inspect: func(string) inspectcontract.InspectInfo { return inspectcontract.InspectInfo{} },
			Daemon:  func() daemoncontract.Status { return d },
			Doctor: func(req doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error) {
				if req.DaemonAdmission.Observed != (d.Running && d.Reachable && d.IdentityVerified) {
					t.Fatalf("admission: %+v", req.DaemonAdmission)
				}
				return doctorcontract.HarnessDoctorResult{OK: true, Healthy: false}, nil
			},
			State:         func() (statecontract.StateListResult, error) { return statecontract.StateListResult{OK: true}, nil },
			Workers:       func() (workercontract.WorkerListResult, error) { return workercontract.WorkerListResult{OK: true}, nil },
			ResolveTarget: func(s string) string { return s },
		}
		if got := service.Run("repo"); !got.OK || got.Doctor.Healthy {
			t.Fatalf("health replaced observation OK: %+v", got)
		}
	}
}
