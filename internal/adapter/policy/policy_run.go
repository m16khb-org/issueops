package policy

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	policyapp "issueops/internal/application/policy"
	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

func FakeRunCommand(request policycontract.CommandPolicyRequest) policycontract.CommandFakeRunResult {
	return (Evaluator{}).FakeRun(request)
}

func RunReadOnlyCommand(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	return (Evaluator{}).RunReadOnly(request)
}

// RunCommand executes argv under the requested write/network permissions.
// The application always clears shell permission before evaluating the request.
func RunCommand(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	return (Evaluator{}).Run(request)
}

func (e Evaluator) FakeRun(request policycontract.CommandPolicyRequest) policycontract.CommandFakeRunResult {
	return e.service().FakeRun(request)
}

func (e Evaluator) RunReadOnly(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	return e.service().RunReadOnly(request)
}

func (e Evaluator) Run(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	return e.service().Run(request)
}

func (e Evaluator) service() policyapp.Service {
	return policyapp.Service{Observer: commandObserver{lookup: e.lookup}, Executor: commandExecutor{}, Clock: systemClock{}}
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type commandExecutor struct{}

func (commandExecutor) Execute(request policycontract.CommandPolicyRequest, timeout time.Duration) policyapp.Execution {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, request.Argv[0], request.Argv[1:]...)
	cmd.Dir = request.CWD
	cmd.Env = commandEnv(request.EnvAllowlist)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := policyapp.Execution{Stdout: stdout.String(), Stderr: stderr.String(), TimedOut: ctx.Err() == context.DeadlineExceeded, Err: err}
	if err != nil {
		result.ExitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	}
	return result
}

func commandEnv(allowlist []string) []string {
	allowed := map[string]bool{}
	for _, name := range policydomain.CleanEnvAllowlist(allowlist) {
		allowed[name] = true
	}
	env := []string{}
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if ok && allowed[name] {
			env = append(env, entry)
		}
	}
	return env
}
