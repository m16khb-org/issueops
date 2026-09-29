package statuscli

import doctorapp "issueops/internal/application/doctor"

type (
	Status                 = HarnessStatus
	SelfVerificationStatus = SelfVerifyStatus
)

func RunStatus(diagnostics doctorapp.Service, args []string) error {
	return runStatus(diagnostics, args)
}

func BuildStatus(diagnostics doctorapp.Service, repo string) Status {
	return buildHarnessStatus(diagnostics, repo)
}
