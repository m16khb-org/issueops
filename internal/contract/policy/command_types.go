// Package policy 패키지는 command policy의 DTO를 소유한다.
//
// 평가 규칙과 redaction은 domain에 남고, 결과를 주고받는 계층은 이 타입만 안다.
package policy

import (
	"strings"
)

// CommandPolicyRequest 타입은 command policy 평가를 요청하는 입력이다.
// 명령은 Argv로만 받고, Network·Write·Shell 허용 플래그가 false인 능력은 허용하지 않는 것으로 평가한다.
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

// CommandPolicyEvaluation 타입은 command policy 평가 결과다.
// Allowed가 false이면 DenyReasons에 거부 사유가 있고, AuditLogID는 감사 로그 항목을 가리킨다.
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

// CommandFakeRunResult 타입은 실제 실행 없이 policy 판정만 검증한 fake run 결과다.
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

// CommandRunResult 타입은 policy를 통과한 명령의 실행 결과다.
// Executed가 false이면 실행되지 않은 것이며, TimedOut은 제한 시간 초과로 끝났음을 뜻한다.
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

// DeniedError 타입은 command policy가 명령을 거부했음을 나타내는 에러다. Reasons에 거부 사유를 담는다.
type DeniedError struct {
	Reasons []string
}

func (e DeniedError) Error() string {
	if len(e.Reasons) == 0 {
		return "command denied by policy"
	}
	return "command denied by policy: " + strings.Join(e.Reasons, "; ")
}

// IsPolicyDenied 함수는 err가 DeniedError 값 자체인지 판별한다.
// 감싼 에러는 인정하지 않는다. 호출 쪽에서 policy 거부를 종료 코드 3으로 대응시키는 데 쓴다.
func IsPolicyDenied(err error) bool {
	_, ok := err.(DeniedError) //nolint:errorlint // exit code 3 maps only a direct policy denial; TestIsPolicyDeniedMatchesDirectErrorOnly pins it
	return ok
}
