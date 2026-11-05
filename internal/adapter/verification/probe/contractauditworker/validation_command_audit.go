// Package contractauditworker holds the self-verify probes that run the built
// binary's contract check, tool conformance baseline, command audit, and worker
// lifecycle smoke tests.
package contractauditworker

import (
	selfverify "issueops/internal/contract/selfverify"

	"path/filepath"
	"strings"
	"time"

	verifydomain "issueops/internal/domain/selfverify"
)

// ValidateCommandAudit runs `policy audit` with a temporary audit log and checks
// that the log has the command_policy_audit fields and no unredacted secret
// fixture. seed is unused.
func ValidateCommandAudit(binary, root string, seed int64) selfverify.StepResult {
	return validateCommandAuditWithDeps(binary, root, seed, validationDeps{})
}

func validateCommandAuditWithDeps(binary, root string, seed int64, deps validationDeps) selfverify.StepResult {
	_ = seed
	deps = deps.withDefaults()
	auditDir, err := deps.MkdirTemp("", "issueops-audit-*")
	if err != nil {
		return verifydomain.FailedStep("command audit smoke", err)
	}
	defer func() { _ = deps.RemoveAll(auditDir) }()
	auditLog := filepath.Join(auditDir, "audit.jsonl")
	step := deps.RunCommandStepEnv(root, "command audit smoke", 30*time.Second, "", []string{"ISSUEOPS_AUDIT_LOG=" + auditLog}, binary, "policy", "audit", "--workspace-root", root, "--cwd", root, "--json", "--", "git", "status", "--short")
	if !step.OK {
		return step
	}
	b, err := deps.ReadFile(auditLog)
	if err != nil {
		return verifydomain.FailedStep("command audit smoke", err)
	}
	errs := []string{}
	text := string(b)
	if !strings.Contains(text, "command_policy_audit") || !strings.Contains(text, "audit_log_id") {
		errs = append(errs, "audit log missing command_policy_audit fields")
	}
	if strings.Contains(strings.ToLower(text), "secret-value") || strings.Contains(text, "sk-123") {
		errs = append(errs, "audit log contains unredacted secret fixture")
	}
	return verifydomain.AssertionStep("command audit smoke", step.DurationMS, errs)
}
