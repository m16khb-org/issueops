// Package install holds the pure install-time rules: MCP proxy cleanup decisions,
// native activation and path-mode validation, and skill name normalization. It performs no I/O.
package install

// MCPProxyIdentity is the observed identity of an MCP proxy process. IdentityVerified is
// true only when StartTime and Executable were read from the OS rather than inferred.
type MCPProxyIdentity struct {
	PID              int
	ParentPID        int
	Command          string
	StartTime        string
	Executable       string
	IdentityVerified bool
}

// MCPProxyCleanupAction decides what to do with a stale MCP proxy process and returns the
// action name: skip-current, skip-unverified, skip-not-exact, skip-live-parent,
// skip-unsupported-platform, would-terminate (dry run), or terminate. Only an orphan
// (parent PID 1) whose verified command is exactly "<executable> mcp" is ever terminated.
func MCPProxyCleanupAction(process MCPProxyIdentity, currentPID int, supported, dryRun bool) string {
	switch {
	case process.PID == currentPID:
		return "skip-current"
	case !process.IdentityVerified || process.StartTime == "" || process.Executable == "":
		return "skip-unverified"
	case process.Command != process.Executable+" mcp":
		return "skip-not-exact"
	case process.ParentPID != 1:
		return "skip-live-parent"
	case !supported:
		return "skip-unsupported-platform"
	case dryRun:
		return "would-terminate"
	default:
		return "terminate"
	}
}

// SameMCPProxyIdentity reports whether left and right describe the same process identity
// in every field, used to recheck a process before terminating it.
func SameMCPProxyIdentity(left, right MCPProxyIdentity) bool {
	return left.PID == right.PID && left.ParentPID == right.ParentPID &&
		left.Command == right.Command && left.StartTime == right.StartTime &&
		left.Executable == right.Executable && left.IdentityVerified == right.IdentityVerified
}
