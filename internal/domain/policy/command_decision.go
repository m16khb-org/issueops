package policy

import (
	"sort"
	"strings"
	"time"

	policycontract "issueops/internal/contract/policy"
)

// CommandFacts are observations made at the filesystem and catalog boundary.
// The policy decision below has no access to either source of facts.
type CommandFacts struct {
	RootDirectory    bool
	CWDDirectory     bool
	CWDWithinRoot    bool
	PathOutsideRoot  bool
	Timeout          time.Duration
	TimeoutValid     bool
	ShellCommand     bool
	UsesNetwork      bool
	Writes           bool
	ReadOnlyAllowed  bool
	PRTargetDeny     string
	PRTargetExpected string
	Warnings         []string
}

type CommandDecision struct {
	Allowed     bool
	DenyReasons []string
	Warnings    []string
}

func EvaluateCommandDecision(req policycontract.CommandPolicyRequest, facts CommandFacts) CommandDecision {
	denies := []string{}
	warnings := append([]string{}, facts.Warnings...)
	if req.WorkspaceRoot == "" {
		denies = append(denies, "workspace_root_required")
	} else if !facts.RootDirectory {
		denies = append(denies, "workspace_root_not_directory")
	}
	if req.CWD == "" {
		denies = append(denies, "cwd_required")
	} else if !facts.CWDDirectory {
		denies = append(denies, "cwd_not_directory")
	}
	if req.WorkspaceRoot != "" && req.CWD != "" && !facts.CWDWithinRoot {
		denies = append(denies, "cwd_outside_workspace")
	}
	if len(req.Argv) == 0 {
		denies = append(denies, "argv_required")
	}
	if !facts.TimeoutValid || facts.Timeout <= 0 {
		denies = append(denies, "invalid_timeout")
	} else if facts.Timeout > 15*time.Minute {
		denies = append(denies, "timeout_exceeds_15m")
	}
	for _, envName := range req.EnvAllowlist {
		if !ValidEnvName(envName) {
			denies = append(denies, "invalid_env_allowlist_name")
			break
		}
	}
	for _, arg := range req.Argv {
		if SecretLikeArg(arg) {
			denies = append(denies, "secret_like_argument")
			break
		}
	}
	if len(req.Argv) > 0 {
		if facts.PathOutsideRoot {
			denies = append(denies, "path_outside_workspace")
		}
		if facts.ShellCommand {
			if !req.ShellAllowed {
				denies = append(denies, "shell_interpreter_not_allowed")
			} else if strings.TrimSpace(req.ShellReason) == "" {
				denies = append(denies, "shell_reason_required")
			} else {
				warnings = append(warnings, "shell_interpreter_exception")
			}
		}
		if facts.UsesNetwork && !req.NetworkAllowed {
			denies = append(denies, "network_not_allowed")
		}
		if facts.Writes && !req.WriteAllowed {
			denies = append(denies, "write_not_allowed")
		}
		if !req.WriteAllowed && !facts.ReadOnlyAllowed {
			denies = append(denies, "command_not_in_read_only_allowlist")
		}
		if facts.PRTargetDeny != "" {
			denies = append(denies, facts.PRTargetDeny)
			warnings = append(warnings, "pr_target_branch_expected="+facts.PRTargetExpected)
		}
	}
	denies = uniqueSorted(denies)
	warnings = uniqueSorted(warnings)
	return CommandDecision{Allowed: len(denies) == 0, DenyReasons: denies, Warnings: warnings}
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value != "" {
			seen[value] = true
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
