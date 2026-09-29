package issueopsapp

import (
	statestore "issueops/internal/adapter/outbound/state"
	statusapp "issueops/internal/application/status"
	"os"
)

func newStatusService() statusapp.Service {
	home, _ := os.UserHomeDir()
	defaultTarget := resolveTarget("")
	return statusapp.Service{
		Home: home, IssueOpsRoot: issueOpsRoot(), Version: version, Inspect: newHarnessInspector(),
		Daemon: newDaemonReader().Run, Doctor: newDoctorService().Run,
		State: newStateService(statestore.StateDir()).List, Workers: newWorkerService().List,
		ResolveTarget: func(target string) string {
			if target != "" {
				return target
			}
			return defaultTarget
		},
	}
}
