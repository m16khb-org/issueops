package installdryrun

import (
	"encoding/json"
	"path/filepath"
	"time"

	verifycontract "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024
const skillName = "atomic-commit-push"

func Validate(binary, root string, seed int64) verifycontract.StepResult {
	return validateInstallDryRunSmokeWithDeps(binary, root, seed, installDryRunValidationDeps{})
}

func validateInstallDryRunSmokeWithDeps(binary, root string, seed int64, deps installDryRunValidationDeps) verifycontract.StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempHome, err := deps.makeTempDir("home", seed)
	if err != nil {
		return verifydomain.FailedStep("install dry-run smoke", err)
	}
	defer func() { _ = deps.removeAll(tempHome) }()
	tempRoot, err := deps.makeTempDir("root", seed)
	if err != nil {
		return verifydomain.FailedStep("install dry-run smoke", err)
	}
	defer func() { _ = deps.removeAll(tempRoot) }()
	skillDir := filepath.Join(tempRoot, "skills", skillName)
	if err := deps.makeDirAll(skillDir, 0o755); err != nil {
		return verifydomain.FailedStep("install dry-run smoke", err)
	}
	if err := deps.writeFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+skillName+"\ndescription: install dry-run smoke\n---\n"), 0o644); err != nil {
		return verifydomain.FailedStep("install dry-run smoke", err)
	}
	env := []string{
		"HOME=" + tempHome,
		"CODEX_HOME=" + filepath.Join(tempHome, ".codex"),
		"ISSUEOPS_ROOT=" + tempRoot,
	}
	step := deps.run(root, "install dry-run smoke", 30*time.Second, "", env, binary, "install", "--dry-run", "--project-local", "--json")
	if !step.OK {
		return step
	}
	var result installDryRunSmokeResult
	if err := json.Unmarshal([]byte(step.Stdout), &result); err != nil {
		return verifydomain.AssertionStepWithOutput("install dry-run smoke", time.Since(started).Milliseconds(), []string{err.Error()}, []string{step.Stdout}, []string{step.Command}, aggregateOutputBudgetBytes)
	}
	errs := installDryRunValidationErrors(result, tempHome, tempRoot, deps.exists)
	if len(errs) > 0 {
		return verifydomain.AssertionStepWithOutput("install dry-run smoke", time.Since(started).Milliseconds(), errs, []string{step.Stdout}, []string{step.Command}, aggregateOutputBudgetBytes)
	}
	step.DurationMS = time.Since(started).Milliseconds()
	return step
}
