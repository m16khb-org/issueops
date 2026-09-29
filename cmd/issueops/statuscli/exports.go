package statuscli

import doctorapp "issueops/internal/application/doctor"

type (
	Status                 = HarnessStatus
	SelfVerificationStatus = SelfVerifyStatus
	WorkResult             = VerifyWorkResult
	WorkEvidenceItem       = VerifyWorkEvidenceItem
	WorkSuggestedCommand   = VerifyWorkSuggestedCommand
)

func RunStatus(diagnostics doctorapp.Service, args []string) error {
	return runStatus(diagnostics, args)
}

func BuildStatus(diagnostics doctorapp.Service, repo string) Status {
	return buildHarnessStatus(diagnostics, repo)
}

func RunVerifyWork(args []string) error {
	return runVerifyWork(args)
}

func BuildVerifyWork(repo string, all bool, argv []string) WorkResult {
	return buildVerifyWork(repo, all, argv)
}
