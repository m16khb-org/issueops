package status

import (
	"errors"
	doctorcontract "issueops/internal/contract/doctor"
	inspectcontract "issueops/internal/contract/inspect"
	statecontract "issueops/internal/contract/state"
	workercontract "issueops/internal/contract/worker"
	"reflect"
	"testing"
)

func TestRunPreservesObservationOrderAndContinuesAfterErrors(t *testing.T) {
	var calls []string
	service := Service{
		Home: "home", IssueOpsRoot: "harness", Version: "version",
		Inspect: func(repo string) inspectcontract.InspectInfo {
			calls = append(calls, "inspect:"+repo)
			return inspectcontract.InspectInfo{Version: "observed"}
		},
		Doctor: func(req doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error) {
			calls = append(calls, "doctor:"+req.RepoRoot)
			if req.RepoRoot != "raw/../repo" || req.Home != "home" || req.IssueOpsRoot != "harness" || req.Version != "version" {
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
	wantCalls := []string{"inspect:raw/../repo", "doctor:raw/../repo", "state", "workers", "resolve:raw/../repo"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls: %v", calls)
	}
	if got.OK || got.Kind != "harness_status" || got.Repo != "resolved" || got.Version != "version" || got.Inspect.Version != "observed" || !reflect.DeepEqual(got.Warnings, []string{"doctor: ", "state: state failed", "workers: worker failed"}) {
		t.Fatalf("result: %+v", got)
	}
}

func TestRunOKDoesNotRequireHealth(t *testing.T) {
	service := Service{
		Inspect: func(string) inspectcontract.InspectInfo { return inspectcontract.InspectInfo{} },
		Doctor: func(doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error) {
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
