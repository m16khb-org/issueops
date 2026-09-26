package lintdiagnose

import (
	lintdiagnoseapp "issueops/internal/application/lintdiagnose"
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
	"os/exec"
)

func DiagnoseCommand(req lintdiagnosecontract.LintDiagnoseRequest) (lintdiagnosecontract.LintDiagnoseResult, error) {
	return (lintdiagnoseapp.Service{Effects: lintEffects{}}).Diagnose(req)
}

func BuildPrompt(exitCode int, logTail string) string {
	return lintdiagnoseapp.BuildPrompt(exitCode, logTail)
}

type lintEffects struct{}

func (lintEffects) NormalizeRoot(root string) (string, error) { return NormalizeRepoRoot(root) }
func (lintEffects) Run(root string, argv []string) (string, int, bool) {
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0, false
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return string(output), exitError.ExitCode(), true
	}
	return string(output), -1, true
}
