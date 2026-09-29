package lintdiagnose

import (
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
	domain "issueops/internal/domain/lintdiagnose"
)

type Effects interface {
	NormalizeRoot(string) (string, error)
	Run(root string, argv []string) (output string, exitCode int, failed bool)
}

type Service struct{ Effects Effects }

func (service Service) Diagnose(req lintdiagnosecontract.LintDiagnoseRequest) (lintdiagnosecontract.LintDiagnoseResult, error) {
	root, err := service.Effects.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return lintdiagnosecontract.LintDiagnoseResult{}, err
	}
	if err := domain.ValidateCommand(req.CommandArgv); err != nil {
		return lintdiagnosecontract.LintDiagnoseResult{}, err
	}
	output, exitCode, failed := service.Effects.Run(root, req.CommandArgv)
	result := lintdiagnosecontract.LintDiagnoseResult{
		OK: true, CommandArgv: req.CommandArgv, ExitCode: exitCode, Failed: failed,
	}
	if !failed {
		return result, nil
	}
	result.Prompt = BuildPrompt(exitCode, domain.FailureTail(output))
	result.Diagnosis = "command failed; prompt contains the host-agent judgement request"
	return result, nil
}
