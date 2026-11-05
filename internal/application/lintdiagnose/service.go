package lintdiagnose

import (
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
	domain "issueops/internal/domain/lintdiagnose"
)

type effects interface {
	NormalizeRoot(string) (string, error)
	Run(root string, argv []string) (output string, exitCode int, failed bool)
}

// Service diagnoses failing commands using its Effects.
type Service struct{ Effects effects }

// Diagnose validates and runs req.CommandArgv in the normalized repo root. A
// passing command returns without a prompt; a failing one carries the prompt
// built from the output tail, and the host agent produces the actual diagnosis.
func (service Service) Diagnose(req lintdiagnosecontract.Request) (lintdiagnosecontract.Result, error) {
	root, err := service.Effects.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return lintdiagnosecontract.Result{}, err
	}
	if err := domain.ValidateCommand(req.CommandArgv); err != nil {
		return lintdiagnosecontract.Result{}, err
	}
	output, exitCode, failed := service.Effects.Run(root, req.CommandArgv)
	result := lintdiagnosecontract.Result{
		OK: true, CommandArgv: req.CommandArgv, ExitCode: exitCode, Failed: failed,
	}
	if !failed {
		return result, nil
	}
	result.Prompt = BuildPrompt(exitCode, domain.FailureTail(output))
	result.Diagnosis = "command failed; prompt contains the host-agent judgement request"
	return result, nil
}
