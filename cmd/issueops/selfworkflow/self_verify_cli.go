package selfworkflow

import "issueops/cmd/issueops/selfworkflow/candidatescmd"

type SelfVerifyCandidatesDeps struct {
	Export func() SelfVerificationCandidateExportResult
	Save   func(*SelfVerificationCandidateExportResult, string) error
}

func RunSelfVerifyCandidates(args []string) error {
	return RunSelfVerifyCandidatesWithDeps(args, SelfVerifyCandidatesDeps{})
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
