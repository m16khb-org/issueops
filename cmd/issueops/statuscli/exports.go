package statuscli

import (
	doctorapp "issueops/internal/application/doctor"
	workerapp "issueops/internal/application/worker"
)

type (
	Status                 = HarnessStatus
	SelfVerificationStatus = SelfVerifyStatus
)

func RunStatus(diagnostics doctorapp.Service, worker workerapp.Service, args []string) error {
	return runStatus(diagnostics, worker, args)
}

func BuildStatus(diagnostics doctorapp.Service, worker workerapp.Service, repo string) Status {
	return buildHarnessStatus(diagnostics, worker, repo)
}
