package lintdiagnose

import (
	"fmt"
	"strings"

	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
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
	if len(req.CommandArgv) == 0 {
		return lintdiagnosecontract.LintDiagnoseResult{}, fmt.Errorf("missing command to execute")
	}
	output, exitCode, failed := service.Effects.Run(root, req.CommandArgv)
	result := lintdiagnosecontract.LintDiagnoseResult{
		OK: true, CommandArgv: req.CommandArgv, ExitCode: exitCode, Failed: failed,
	}
	if !failed {
		return result, nil
	}
	lines := strings.Split(output, "\n")
	if len(lines) > 150 {
		lines = lines[len(lines)-150:]
	}
	result.Prompt = BuildPrompt(exitCode, strings.Join(lines, "\n"))
	result.Diagnosis = "command failed; prompt contains the host-agent judgement request"
	return result, nil
}
