package policy

import (
	"fmt"
	"strings"
	"time"

	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

type Clock interface{ Now() time.Time }

type Execution struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
	Err      error
}

type Executor interface {
	Execute(policycontract.CommandPolicyRequest, time.Duration) Execution
}

func (service Service) FakeRun(request policycontract.CommandPolicyRequest) policycontract.CommandFakeRunResult {
	started := service.Clock.Now()
	policy := service.Evaluate(request)
	finished := service.Clock.Now()
	result := policycontract.CommandFakeRunResult{
		OK: policy.Allowed, Executed: false, ExitCode: 0,
		StartedAt:  started.UTC().Format(time.RFC3339Nano),
		FinishedAt: finished.UTC().Format(time.RFC3339Nano),
		DurationMS: finished.Sub(started).Milliseconds(), Policy: policy,
	}
	if !policy.Allowed {
		result.ExitCode = 3
		result.Stderr = "fake-run denied by policy: " + strings.Join(policy.DenyReasons, "; ") + "\n"
		return result
	}
	result.Stdout = fmt.Sprintf("fake-run accepted by policy; command was not executed\nargv: %s\naudit_log_id: %s\n", strings.Join(policy.Argv, " "), policy.AuditLogID)
	return result
}

func (service Service) RunReadOnly(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	request.WriteAllowed = false
	request.NetworkAllowed = false
	request.ShellAllowed = false
	return service.run(request, "read_only")
}

func (service Service) Run(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	request.ShellAllowed = false
	if request.WriteAllowed || request.NetworkAllowed {
		return service.run(request, "privileged")
	}
	return service.run(request, "read_only")
}

func (service Service) run(request policycontract.CommandPolicyRequest, tier string) policycontract.CommandRunResult {
	started := service.Clock.Now()
	policy := service.Evaluate(request)
	result := policycontract.CommandRunResult{
		OK: policy.Allowed, Executed: false, ExitCode: 0,
		StartedAt: started.UTC().Format(time.RFC3339Nano),
		ReadOnly:  tier == "read_only", Policy: policy,
	}
	if !policy.Allowed {
		finished := service.Clock.Now()
		result.ExitCode = 3
		result.FinishedAt = finished.UTC().Format(time.RFC3339Nano)
		result.DurationMS = finished.Sub(started).Milliseconds()
		result.Stderr = "run denied by policy: " + strings.Join(policy.DenyReasons, "; ") + "\n"
		return result
	}
	timeout, _ := policydomain.CommandTimeout(request.Timeout)
	execution := service.Executor.Execute(request, timeout)
	finished := service.Clock.Now()
	result.Executed = true
	result.FinishedAt = finished.UTC().Format(time.RFC3339Nano)
	result.DurationMS = finished.Sub(started).Milliseconds()
	result.TimedOut = execution.TimedOut
	result.Stdout = budgetOutput(policydomain.RedactFreeform(execution.Stdout))
	result.Stderr = budgetOutput(policydomain.RedactFreeform(execution.Stderr))
	if execution.Err != nil {
		result.OK = false
		result.ExitCode = execution.ExitCode
		if result.ExitCode == 0 {
			result.ExitCode = 1
		}
		if result.TimedOut {
			result.ExitCode = 124
			if result.Stderr != "" && !strings.HasSuffix(result.Stderr, "\n") {
				result.Stderr += "\n"
			}
			result.Stderr += "command timed out\n"
		}
		return result
	}
	result.OK = true
	return result
}

func budgetOutput(text string) string {
	const limit = 32 * 1024
	if len(text) <= limit {
		return text
	}
	return policydomain.TruncateBytes(text, limit) + "\n<truncated>\n"
}
