package policy

import policycontract "issueops/internal/contract/policy"

import "sort"

// ResolveTier maps the capabilities requested in request to a policy tier. The
// broadest capability wins (shell, then network, then write, else read-only), and
// the granted capabilities are listed in sorted order.
func ResolveTier(request policycontract.Request) policycontract.Tier {
	capabilities := []string{}
	if request.WriteAllowed {
		capabilities = append(capabilities, "write")
	}
	if request.NetworkAllowed {
		capabilities = append(capabilities, "network")
	}
	if request.ShellAllowed {
		capabilities = append(capabilities, "shell")
	}
	sort.Strings(capabilities)
	name := policycontract.TierReadOnly
	switch {
	case request.ShellAllowed:
		name = policycontract.TierShellException
	case request.NetworkAllowed:
		name = policycontract.TierNetworkAccess
	case request.WriteAllowed:
		name = policycontract.TierWorkspaceWrite
	}
	return policycontract.Tier{Name: name, GrantedCapabilities: capabilities, Rationale: Rationale(name)}
}

// Rationale returns the human-readable reason for the named tier. An unknown name
// gets the read-only rationale.
func Rationale(name string) string {
	switch name {
	case policycontract.TierShellException:
		return "shell interpreter exception granted; requires an explicit shell_reason and is audited"
	case policycontract.TierNetworkAccess:
		return "network capability granted; shell interpreters remain denied"
	case policycontract.TierWorkspaceWrite:
		return "write capability granted within the workspace; network and shell remain denied"
	default:
		return "no write, network, or shell capability requested; restricted to the read-only allowlist"
	}
}
