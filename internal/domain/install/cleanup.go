package install

type MCPProxyIdentity struct {
	PID              int
	ParentPID        int
	Command          string
	StartTime        string
	Executable       string
	IdentityVerified bool
}

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

func SameMCPProxyIdentity(left, right MCPProxyIdentity) bool {
	return left.PID == right.PID && left.ParentPID == right.ParentPID &&
		left.Command == right.Command && left.StartTime == right.StartTime &&
		left.Executable == right.Executable && left.IdentityVerified == right.IdentityVerified
}
