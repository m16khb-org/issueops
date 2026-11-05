package policy

// Tier names, from the least to the most privileged envelope.
const (
	TierReadOnly       = "read_only"
	TierWorkspaceWrite = "workspace_write"
	TierNetworkAccess  = "network_access"
	TierShellException = "shell_exception"
)

// Request is the capability flags a caller asks for; domain ResolveTier maps it to a Tier.
type Request struct {
	WriteAllowed   bool
	NetworkAllowed bool
	ShellAllowed   bool
}

// Tier is a host-neutral synthesis of the scattered capability flags
// (write/network/shell) into one named privilege envelope, derived from
// Qwen Code's tiered-approval model. It classifies what a request is permitted
// to attempt; it does not decide whether a specific command is allowed (that
// stays in DenyReasons). The YOLO/auto-escalation tiers are deliberately
// excluded so a single approval never raises the whole session's safety level.
type Tier struct {
	Name                string   `json:"name"`
	GrantedCapabilities []string `json:"granted_capabilities"`
	Rationale           string   `json:"rationale"`
}
