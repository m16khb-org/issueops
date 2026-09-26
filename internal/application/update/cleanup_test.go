package update

import (
	"errors"
	"testing"

	updatecontract "issueops/internal/contract/update"
)

type cleanupEffects struct {
	calls        int
	terminated   []int
	process      updatecontract.MCPProxyProcess
	second       updatecontract.MCPProxyProcess
	err          error
	errOnCall    int
	terminateErr error
}

func (fake *cleanupEffects) List() ([]updatecontract.MCPProxyProcess, error) {
	fake.calls++
	if fake.err != nil && (fake.errOnCall == 0 || fake.calls == fake.errOnCall) {
		return nil, fake.err
	}
	if fake.calls > 1 {
		return []updatecontract.MCPProxyProcess{fake.second}, nil
	}
	return []updatecontract.MCPProxyProcess{fake.process}, nil
}
func (fake *cleanupEffects) Terminate(pid int) error {
	fake.terminated = append(fake.terminated, pid)
	return fake.terminateErr
}
func (*cleanupEffects) CurrentPID() int                 { return 1 }
func (*cleanupEffects) SupportsOrphanTermination() bool { return true }

func TestCleanupRevalidatesIdentityBeforeSignal(t *testing.T) {
	valid := updatecontract.MCPProxyProcess{PID: 33, ParentPID: 1, Command: "/bin/issueops mcp", StartTime: "before", Executable: "/bin/issueops", IdentityVerified: true}
	for _, tc := range []struct {
		name              string
		second            updatecontract.MCPProxyProcess
		dryRun            bool
		want              string
		calls, terminated int
	}{
		{name: "dry run", second: valid, dryRun: true, want: "would-terminate", calls: 1},
		{name: "apply", second: valid, want: "terminated", calls: 2, terminated: 1},
		{name: "reused PID", second: updatecontract.MCPProxyProcess{PID: 33, ParentPID: 1, Command: valid.Command, StartTime: "after", Executable: valid.Executable, IdentityVerified: true}, want: "skip-identity-changed", calls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &cleanupEffects{process: valid, second: tc.second}
			result, err := CleanupMCPProxies(fake, tc.dryRun)
			if err != nil || len(result.Processes) != 1 || result.Processes[0].Action != tc.want || fake.calls != tc.calls || len(fake.terminated) != tc.terminated {
				t.Fatalf("result=%+v calls=%d terminated=%v err=%v", result, fake.calls, fake.terminated, err)
			}
		})
	}
	fake := &cleanupEffects{err: errors.New("ps failed")}
	result, err := CleanupMCPProxies(fake, true)
	if err == nil || result.OK {
		t.Fatalf("list failure: result=%+v err=%v", result, err)
	}
}

func TestCleanupStopsOnRevalidationOrTerminationFailure(t *testing.T) {
	valid := updatecontract.MCPProxyProcess{PID: 33, ParentPID: 1, Command: "/bin/issueops mcp", StartTime: "before", Executable: "/bin/issueops", IdentityVerified: true}
	for _, tc := range []struct {
		name           string
		listErrOnCall  int
		terminateErr   error
		want           string
		terminateCalls int
	}{
		{name: "revalidation failed", listErrOnCall: 2, want: "skip-revalidation-error"},
		{name: "signal failed", terminateErr: errors.New("signal failed"), want: "terminate-error", terminateCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &cleanupEffects{process: valid, second: valid, errOnCall: tc.listErrOnCall, terminateErr: tc.terminateErr}
			if tc.listErrOnCall != 0 {
				fake.err = errors.New("ps failed")
			}
			result, err := CleanupMCPProxies(fake, false)
			if err == nil || result.OK || len(result.Processes) != 1 || result.Processes[0].Action != tc.want || len(fake.terminated) != tc.terminateCalls {
				t.Fatalf("result=%+v terminated=%v err=%v", result, fake.terminated, err)
			}
		})
	}
}
