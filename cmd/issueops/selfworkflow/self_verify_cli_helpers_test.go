package selfworkflow

import (
	"issueops/cmd/issueops/selfworkflow/candidatescmd"
	contract "issueops/internal/contract/selfaugment"
)

type SelfVerifyCandidatesDeps struct {
	Export func() contract.SelfVerificationCandidateExportResult
	Save   func(*contract.SelfVerificationCandidateExportResult, string) error
}

func RunSelfVerifyCandidatesWithDeps(args []string, deps SelfVerifyCandidatesDeps) error {
	deps = deps.withDefaults()
	return candidatescmd.Run(args, candidatescmd.Deps{Export: deps.Export, Save: deps.Save, PrintJSON: printJSON})
}
func (deps SelfVerifyCandidatesDeps) withDefaults() SelfVerifyCandidatesDeps {
	if deps.Export == nil {
		deps.Export = ExportSelfVerificationCandidates
	}
	if deps.Save == nil {
		deps.Save = SaveSelfVerificationCandidateExport
	}
	return deps
}
