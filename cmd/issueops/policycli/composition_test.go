package policycli

import (
	"issueops/cmd/issueops/pathutil"
	auditadapter "issueops/internal/adapter/audit"
	policyadapter "issueops/internal/adapter/policy"
	auditapp "issueops/internal/application/audit"
	policyapp "issueops/internal/application/policy"
	policy "issueops/internal/contract/policy"
)

func testCommand() Command {
	service := policyapp.Service{Observer: policyadapter.CommandObserver{}, Overrides: policyadapter.OverrideLoader{}, Executor: policyadapter.CommandExecutor{}, Clock: policyadapter.Clock{}}
	return Command{DefaultRoot: pathutil.ResolveTarget(""), Policy: service, Audit: auditapp.Service{Evaluator: service, Writer: auditadapter.NewCommandWriter(), Clock: auditadapter.Clock{}}}
}
func Run(args []string) error            { return testCommand().Run(args) }
func RunCheck(args []string) error       { return testCommand().Check(args) }
func RunFakeRun(args []string) error     { return testCommand().FakeRun(args) }
func RunReadOnly(args []string) error    { return testCommand().RunReadOnly(args) }
func RunAudit(args []string) error       { return testCommand().AuditPolicy(args) }
func runPolicyRun(args []string) error   { return testCommand().RunReadOnly(args) }
func runPolicyCheck(args []string) error { return testCommand().Check(args) }
func parseCommandPolicyFlags(name string, args []string) (policy.CommandPolicyRequest, bool, error) {
	return testCommand().ParseFlags(name, args)
}
func parseCommandPolicyRunFlags(args []string) (policy.CommandPolicyRequest, bool, bool, error) {
	return testCommand().ParseRunFlags(args)
}
