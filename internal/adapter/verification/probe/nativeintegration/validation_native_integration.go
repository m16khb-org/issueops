package nativeintegration

import (
	"fmt"
	"strings"
	"time"

	verifycontract "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024

func validateNativeIntegrationWithDeps(root string, deps nativeIntegrationValidationDeps) verifycontract.StepResult {
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
	ompSkills, _ := deps.SkillNamesForHost(root, nativeSkills, "omp")
	opencodeSkills, _ := deps.SkillNamesForHost(root, nativeSkills, "opencode")
	opencodeConfigRoot := ""
	if deps.OpencodeConfigRoot != nil {
		opencodeConfigRoot = deps.OpencodeConfigRoot(home)
	} else {
		errs = append(errs, "opencode config root resolver is unavailable")
	}
	paths := nativeIntegrationRequiredPaths(root, home, opencodeConfigRoot, codexSkills, claudeSkills, omoSkills, ompSkills, opencodeSkills)
	errs = append(errs, nativeIntegrationPathErrors(paths, deps)...)
	errs = append(errs, nativeIntegrationCodexConfigErrors(root, home, deps)...)
	errs = append(errs, nativeIntegrationOmoConfigErrors(root, home, deps)...)
	errs = append(errs, nativeIntegrationOmpConfigErrors(root, home, deps)...)
	errs = append(errs, nativeIntegrationOpencodeConfigErrors(root, opencodeConfigRoot, deps)...)
	warningErrs, warningOutput := nativeIntegrationDuplicateWarningOutput(deps.duplicateWarningFixture())
	errs = append(errs, warningErrs...)
	stdoutParts = append(stdoutParts, warningOutput)
	if len(errs) > 0 {
		return verifydomain.AssertionStepWithOutput("native integration", time.Since(started).Milliseconds(), errs, stdoutParts, nil, aggregateOutputBudgetBytes)
	}
	stdoutText, stdoutTruncated, stdoutBytes := verifydomain.TailWithBudget(strings.Join(stdoutParts, "\n"), aggregateOutputBudgetBytes)
	return verifycontract.StepResult{
		Label:           "native integration",
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
