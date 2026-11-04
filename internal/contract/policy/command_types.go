// Package policy는 command policy의 DTO를 소유한다.
//
// 평가 규칙과 redaction은 domain에 남고, 결과를 주고받는 계층은 이 타입만 안다.
package policy

import (
	"strings"
)

type CommandPolicyRequest struct {
	WorkspaceRoot  string   `json:"workspace_root"`
	CWD            string   `json:"cwd"`
	Argv           []string `json:"argv"`
	Timeout        string   `json:"timeout"`
	EnvAllowlist   []string `json:"env_allowlist"`
	NetworkAllowed bool     `json:"network_allowed"`
	WriteAllowed   bool     `json:"write_allowed"`
	ShellAllowed   bool     `json:"shell_allowed"`
	ShellReason    string   `json:"shell_reason,omitempty"`
	AuditLogID     string   `json:"audit_log_id,omitempty"`
}

type CommandPolicyEvaluation struct {
	OK             bool     `json:"ok"`
	Allowed        bool     `json:"allowed"`
	AuditLogID     string   `json:"audit_log_id"`
	WorkspaceRoot  string   `json:"workspace_root"`
	CWD            string   `json:"cwd"`
	Argv           []string `json:"argv"`
	Timeout        string   `json:"timeout"`
	EnvAllowlist   []string `json:"env_allowlist"`
	NetworkAllowed bool     `json:"network_allowed"`
	WriteAllowed   bool     `json:"write_allowed"`
	ShellAllowed   bool     `json:"shell_allowed"`
	ShellReason    string   `json:"shell_reason,omitempty"`
	Tier           Tier     `json:"tier"`
	DenyReasons    []string `json:"deny_reasons"`
	Warnings       []string `json:"warnings"`
	GeneratedAt    string   `json:"generated_at"`
}

type CommandFakeRunResult struct {
	OK         bool                    `json:"ok"`
	Executed   bool                    `json:"executed"`
	ExitCode   int                     `json:"exit_code"`
	Stdout     string                  `json:"stdout,omitempty"`
	Stderr     string                  `json:"stderr,omitempty"`
	StartedAt  string                  `json:"started_at"`
	FinishedAt string                  `json:"finished_at"`
	DurationMS int64                   `json:"duration_ms"`
	Policy     CommandPolicyEvaluation `json:"policy"`
}

type CommandRunResult struct {
	OK         bool                    `json:"ok"`
	Executed   bool                    `json:"executed"`
	ExitCode   int                     `json:"exit_code"`
	Stdout     string                  `json:"stdout,omitempty"`
	Stderr     string                  `json:"stderr,omitempty"`
	StartedAt  string                  `json:"started_at"`
	FinishedAt string                  `json:"finished_at"`
	DurationMS int64                   `json:"duration_ms"`
	TimedOut   bool                    `json:"timed_out"`
	ReadOnly   bool                    `json:"read_only"`
	Policy     CommandPolicyEvaluation `json:"policy"`
}

type DeniedError struct {
	Reasons []string
}

func (e DeniedError) Error() string {
	if len(e.Reasons) == 0 {
		return "command denied by policy"
	}
	return "command denied by policy: " + strings.Join(e.Reasons, "; ")
}

func IsPolicyDenied(err error) bool {
	_, ok := err.(DeniedError) //nolint:errorlint // exit code 3 maps only a direct policy denial; TestIsPolicyDeniedMatchesDirectErrorOnly pins it
	return ok
}
