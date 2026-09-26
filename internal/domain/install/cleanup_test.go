package install

import (
	"testing"
)

func TestMCPProxyCleanupDecisionAndIdentity(t *testing.T) {
	valid := MCPProxyIdentity{PID: 33, ParentPID: 1, Command: "/bin/issueops mcp", StartTime: "start", Executable: "/bin/issueops", IdentityVerified: true}
	for _, tc := range []struct {
		name              string
		process           MCPProxyIdentity
		current           int
		supported, dryRun bool
		want              string
	}{
		{name: "current", process: valid, current: 33, supported: true, want: "skip-current"},
		{name: "unverified", process: MCPProxyIdentity{PID: 33}, supported: true, want: "skip-unverified"},
		{name: "wrong command", process: MCPProxyIdentity{PID: 33, ParentPID: 1, Command: "other", StartTime: "start", Executable: "/bin/issueops", IdentityVerified: true}, supported: true, want: "skip-not-exact"},
		{name: "live parent", process: MCPProxyIdentity{PID: 33, ParentPID: 2, Command: valid.Command, StartTime: "start", Executable: valid.Executable, IdentityVerified: true}, supported: true, want: "skip-live-parent"},
		{name: "unsupported", process: valid, want: "skip-unsupported-platform"},
		{name: "dry run", process: valid, supported: true, dryRun: true, want: "would-terminate"},
		{name: "eligible", process: valid, supported: true, want: "terminate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MCPProxyCleanupAction(tc.process, tc.current, tc.supported, tc.dryRun); got != tc.want {
				t.Fatalf("action = %q, want %q", got, tc.want)
			}
		})
	}
	if !SameMCPProxyIdentity(valid, valid) {
		t.Fatal("same process should retain identity")
	}
	reused := valid
	reused.StartTime = "later"
	if SameMCPProxyIdentity(valid, reused) {
		t.Fatal("reused PID must change identity")
	}
}
