package policy

import (
	"fmt"
	"strings"
	"time"

	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

type clock interface{ Now() time.Time }

// Execution 타입은 명령 실행기가 돌려주는 원시 결과다. 표준 출력과 오류, 종료 코드, 시간 초과 여부, 출력이 잘렸는지 여부를 담는다.
type Execution struct {
	StdoutTruncated bool
	StderrTruncated bool
	Stdout          string
	Stderr          string
	ExitCode        int
	TimedOut        bool
	Err             error
}

type executor interface {
	Execute(policycontract.CommandPolicyRequest, time.Duration) Execution
}

// FakeRun 메서드는 정책만 평가하고 명령은 실행하지 않는다. 허용되면 수락 메시지를, 거부되면 종료 코드 3과 거부 사유를 결과에 담는다.
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

// RunReadOnly 메서드는 쓰기, 네트워크, 셸 권한을 모두 끈 뒤 읽기 전용 등급으로 명령을 실행한다. 요청이 더 넓은 권한을 달라고 해도 경고와 함께 낮춰서 평가한다.
func (service Service) RunReadOnly(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	request.WriteAllowed = false
	request.NetworkAllowed = false
	request.ShellAllowed = false
	result := service.runWithTier(request, "read_only")
	result.Policy.Warnings = policydomain.ForcedReadOnlyWarnings(result.Policy.Warnings)
	return result
}

// Run 메서드는 셸을 끄고 요청한 권한에 맞는 등급으로 명령을 실행한다. 쓰기나 네트워크가 필요하면 privileged, 아니면 read_only 등급이다. 정책이 거부하면 실행하지 않고 종료 코드 3을 돌려준다.
func (service Service) Run(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	request.ShellAllowed = false
	if request.WriteAllowed || request.NetworkAllowed {
		return service.runWithTier(request, "privileged")
	}
	return service.runWithTier(request, "read_only")
}

func (service Service) runWithTier(request policycontract.CommandPolicyRequest, tier string) policycontract.CommandRunResult {
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
	result.Stdout = capturedOutput(execution.Stdout, execution.StdoutTruncated)
	result.Stderr = capturedOutput(execution.Stderr, execution.StderrTruncated)
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

func capturedOutput(text string, truncated bool) string {
	text = policydomain.RedactFreeform(text)
	if len(text) > 32*1024 {
		return budgetOutput(text)
	}
	// The command executor marks the omitted middle itself; other executors
	// get the marker appended.
	if truncated && !strings.Contains("\n"+text, "\n<truncated>\n") {
		return text + "\n<truncated>\n"
	}
	return text
}
