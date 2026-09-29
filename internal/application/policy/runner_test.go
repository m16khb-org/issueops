package policy

import (
	"errors"
	"strings"
	"testing"
	"time"

	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

type runnerClock struct{ at time.Time }

func (clock runnerClock) Now() time.Time { return clock.at }

type executeFunc func(policycontract.CommandPolicyRequest, time.Duration) Execution

func (fn executeFunc) Execute(request policycontract.CommandPolicyRequest, timeout time.Duration) Execution {
	return fn(request, timeout)
}

func TestRunPreservesTimeoutExitAndFakeRunNeverExecutes(t *testing.T) {
	service := Service{
		Clock: runnerClock{at: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)},
		Observer: observeFunc(func(policycontract.CommandPolicyRequest) Observation {
			return Observation{Root: "/repo", CWD: "/repo",
				Facts: policydomain.CommandFacts{RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true,
					ReadOnlyAllowed: true}}
		}),
		Executor: executeFunc(func(policycontract.CommandPolicyRequest, time.Duration) Execution {
			return Execution{Stderr: "partial", ExitCode: -1, TimedOut: true, Err: errors.New("deadline")}
		}),
	}
	request := policycontract.CommandPolicyRequest{WorkspaceRoot: "/repo", CWD: "/repo", Argv: []string{"cat", "note"}, Timeout: "1s"}
	result := service.Run(request)
	if result.OK || !result.Executed || !result.TimedOut || result.ExitCode != 124 || result.Stderr != "partial\ncommand timed out\n" {
		t.Fatalf("timeout result=%+v", result)
	}
	service.Executor = executeFunc(func(policycontract.CommandPolicyRequest, time.Duration) Execution {
		t.Fatal("fake run executed the command")
		return Execution{}
	})
	fake := service.FakeRun(request)
	if !fake.OK || fake.Executed || !strings.Contains(fake.Stdout, "command was not executed") {
		t.Fatalf("fake result=%+v", fake)
	}
}

func TestReadOnlyRunDeniesBeforeExecution(t *testing.T) {
	clock := runnerClock{at: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)}
	service := Service{
		Clock: clock,
		Observer: observeFunc(func(request policycontract.CommandPolicyRequest) Observation {
			if request.WriteAllowed || request.NetworkAllowed || request.ShellAllowed {
				t.Fatalf("read-only flags were not cleared: %+v", request)
			}
			return Observation{Root: "/repo", CWD: "/repo",
				Facts: policydomain.CommandFacts{RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true,
					Writes: true}}
		}),
		Executor: executeFunc(func(policycontract.CommandPolicyRequest, time.Duration) Execution {
			t.Fatal("denied request reached executor")
			return Execution{}
		}),
	}
	result := service.RunReadOnly(policycontract.CommandPolicyRequest{WorkspaceRoot: "/repo", CWD: "/repo", Argv: []string{"touch", "marker"}, Timeout: "30s", WriteAllowed: true})
	if result.OK || result.Executed || result.ExitCode != 3 || !result.ReadOnly ||
		!strings.Contains(result.Stderr, "write_not_allowed") {
		t.Fatalf("denied result=%+v", result)
	}
}

func TestAllowedRunUsesBoundedExecutorAndRedactsOutput(t *testing.T) {
	service := Service{
		Clock: runnerClock{at: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)},
		Observer: observeFunc(func(policycontract.CommandPolicyRequest) Observation {
			return Observation{Root: "/repo", CWD: "/repo",
				Facts: policydomain.CommandFacts{RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true,
					ReadOnlyAllowed: true}}
		}),
		Executor: executeFunc(func(request policycontract.CommandPolicyRequest, timeout time.Duration) Execution {
			if timeout != 5*time.Second || request.Argv[0] != "cat" {
				t.Fatalf("executor request=%+v timeout=%v", request, timeout)
			}
			return Execution{Stdout: "token=secret-value\n"}
		}),
	}
	result := service.Run(policycontract.CommandPolicyRequest{WorkspaceRoot: "/repo", CWD: "/repo", Argv: []string{"cat", "note"}, Timeout: "5s"})
	if !result.OK || !result.Executed || result.ExitCode != 0 || strings.Contains(result.Stdout, "secret-value") {
		t.Fatalf("allowed result=%+v", result)
	}
}
