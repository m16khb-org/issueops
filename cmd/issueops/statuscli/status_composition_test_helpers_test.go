package statuscli

import (
	statestore "issueops/internal/adapter/outbound/state"
	doctorapp "issueops/internal/application/doctor"
	statusapp "issueops/internal/application/status"
	workerapp "issueops/internal/application/worker"
	statuscontract "issueops/internal/contract/status"
	"os"
)

func testStatusService(diagnostics doctorapp.Service, worker workerapp.Service) statusapp.Service {
	home, _ := os.UserHomeDir()
	return statusapp.Service{Home: home, IssueOpsRoot: deps.IssueOpsRoot(), Version: deps.Version, Inspect: deps.InspectHarness, Doctor: diagnostics.Run, State: statestore.NewService().List, StateRead: statestore.NewService().Read, Workers: worker.List, ResolveTarget: deps.ResolveTarget}
}
func BuildStatus(diagnostics doctorapp.Service, worker workerapp.Service, repo string) statuscontract.Result {
	return testStatusService(diagnostics, worker).Run(repo)
}
func RunStatus(diagnostics doctorapp.Service, worker workerapp.Service, args []string) error {
	return (Command{Service: testStatusService(diagnostics, worker)}).Run(args)
}
