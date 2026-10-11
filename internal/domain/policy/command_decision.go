package policy

import (
	"slices"
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
	denies = append(denies, workspaceDenies(req, facts)...)
	if len(req.Argv) == 0 {
		denies = append(denies, "argv_required")
	}
	denies = append(denies, timeoutDenies(facts)...)
	denies = append(denies, argumentDenies(req)...)
	if len(req.Argv) > 0 {
		denies, warnings = appendCommandDenies(denies, warnings, req, facts)
	}
	denies = uniqueSorted(denies)
	warnings = uniqueSorted(warnings)
	return CommandDecision{Allowed: len(denies) == 0, DenyReasons: denies, Warnings: warnings}
}

// workspaceDenies는 작업 공간 루트와 CWD에 대한 거부 사유를 모은다.
func workspaceDenies(req policycontract.CommandPolicyRequest, facts CommandFacts) []string {
	var denies []string
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
	return denies
}

// timeoutDenies는 타임아웃 값이 없거나 15분을 넘는 경우의 거부 사유를 모은다.
func timeoutDenies(facts CommandFacts) []string {
	if !facts.TimeoutValid || facts.Timeout <= 0 {
		return []string{"invalid_timeout"}
	}
	if facts.Timeout > 15*time.Minute {
		return []string{"timeout_exceeds_15m"}
	}
	return nil
}

// argumentDenies는 env 허용 목록 이름과 비밀처럼 보이는 인자를 검사한다.
func argumentDenies(req policycontract.CommandPolicyRequest) []string {
	var denies []string
	for _, envName := range req.EnvAllowlist {
		if !ValidEnvName(envName) {
			denies = append(denies, "invalid_env_allowlist_name")
			break
		}
	}
	if slices.ContainsFunc(req.Argv, SecretLikeArg) {
		denies = append(denies, "secret_like_argument")
	}
	return denies
}

// appendCommandDenies는 argv가 있을 때만 적용되는 경로·셸·네트워크·쓰기·PR 대상 규칙을 적용한다.
func appendCommandDenies(denies, warnings []string, req policycontract.CommandPolicyRequest, facts CommandFacts) ([]string, []string) {
	if facts.PathOutsideRoot {
		denies = append(denies, "path_outside_workspace")
	}
	if facts.ShellCommand {
		denies, warnings = appendShellDecision(denies, warnings, req)
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
	return denies, warnings
}

// appendShellDecision은 셸 인터프리터 허용 여부와 사유 필요 여부를 판정한다.
func appendShellDecision(denies, warnings []string, req policycontract.CommandPolicyRequest) ([]string, []string) {
	if !req.ShellAllowed {
		return append(denies, "shell_interpreter_not_allowed"), warnings
	}
	if strings.TrimSpace(req.ShellReason) == "" {
		return append(denies, "shell_reason_required"), warnings
	}
	return denies, append(warnings, "shell_interpreter_exception")
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
