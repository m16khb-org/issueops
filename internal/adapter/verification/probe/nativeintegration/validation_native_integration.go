package nativeintegration

import (
	"fmt"
	"strings"
	"time"

	verifycontract "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024

type StepResult = verifycontract.StepResult

func validateNativeIntegrationWithDeps(root string, deps nativeIntegrationValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	home, err := deps.userHomeDir()
	if err != nil {
		return verifydomain.FailedStep("native integration", fmt.Errorf("user home: %w", err))
	}
	errs := []string{}
	stdoutParts := []string{}
	nativeSkills, err := deps.ListSkillNames(root)
	if err != nil {
		errs = append(errs, "list native skills: "+err.Error())
	}
	codexSkills, _ := deps.SkillNamesForHost(root, nativeSkills, "codex")
	claudeSkills, _ := deps.SkillNamesForHost(root, nativeSkills, "claude")
	omoSkills, _ := deps.SkillNamesForHost(root, nativeSkills, "omo")
	paths := nativeIntegrationRequiredPaths(root, home, codexSkills, claudeSkills, omoSkills)
	errs = append(errs, nativeIntegrationPathErrors(paths, deps)...)
	errs = append(errs, nativeIntegrationCodexConfigErrors(root, home, deps)...)
	errs = append(errs, nativeIntegrationOmoConfigErrors(root, home, deps)...)
	warningErrs, warningOutput := nativeIntegrationDuplicateWarningOutput(deps.duplicateWarningFixture())
	errs = append(errs, warningErrs...)
	stdoutParts = append(stdoutParts, warningOutput)
	if len(errs) > 0 {
		return verifydomain.AssertionStepWithOutput("native integration", time.Since(started).Milliseconds(), errs, stdoutParts, nil, aggregateOutputBudgetBytes)
	}
	stdoutText, stdoutTruncated, stdoutBytes := verifydomain.TailWithBudget(strings.Join(stdoutParts, "\n"), aggregateOutputBudgetBytes)
	return StepResult{
		Label:           "native integration",
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
