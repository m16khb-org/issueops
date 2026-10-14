// Package gates는 태스크 게이트 ledger capability의 DTO를 소유한다.
//
// CLI와 MCP가 같은 요청/응답 계약을 공유하며, ledger 파일 형식 자체는
// internal/domain/gates가 소유한다.
package gates

import "errors"

// ErrNoGateFiles는 게이트 파일을 못 찾았을 때의 사용법 오류이다(unlazy exit 2).
var ErrNoGateFiles = errors.New("no gate files found (.issueops/issues/<n>/gates.md or .issueops/gates/*.md)")

// ErrNothingToLint는 lint할 게이트 출처가 하나도 없을 때의 사용법 오류다(exit 2).
var ErrNothingToLint = errors.New("give at least one --gate spec, --plan file, or --file ledger to lint")

// SchemaVersion는 gates 응답 계약의 현재 버전이다.
const SchemaVersion = 1

// 실행 권한 상수. policy tier와 같은 어휘를 쓴다.
const (
	TimeoutDefaultSeconds = 120
)

// CheckRequest는 gates check/status 실행 요청이다. StatusOnly가 true면 명령을
// 실행하지 않고 상태만 계산한다(unlazy의 --status).
type CheckRequest struct {
	WorkspaceRoot  string   `json:"workspace_root"`
	CWD            string   `json:"cwd"`
	Files          []string `json:"files,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	EnvAllowlist   []string `json:"env_allowlist,omitempty"`
	WriteAllowed   bool     `json:"write_allowed"`
	NetworkAllowed bool     `json:"network_allowed"`
	StatusOnly     bool     `json:"status_only"`
}

// GateResult는 게이트 하나의 평가 결과다.
type GateResult struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	State         string `json:"state"` // met | unchecked | evidence_pending | abandoned
	Checked       bool   `json:"checked"`
	HasCheck      bool   `json:"has_check"`
	Evidence      string `json:"evidence,omitempty"`
	AbandonReason string `json:"abandon_reason,omitempty"`
	// CheckError는 CHECK 실행이 실패했거나 정책이 거부한 이유다.
	CheckError string `json:"check_error,omitempty"`
	// PolicyDenied는 CHECK 명령이 command policy에 거부되었는지 나타낸다.
	PolicyDenied bool `json:"policy_denied,omitempty"`
	// AuditLogID는 실행된 CHECK의 audit log id다.
	AuditLogID string `json:"audit_log_id,omitempty"`
}

// FileResult는 게이트 파일 하나의 평가 결과다.
type FileResult struct {
	File      string       `json:"file"`
	GateCount int          `json:"gate_count"`
	Met       int          `json:"met"`
	Unmet     int          `json:"unmet"`
	Abandoned int          `json:"abandoned"`
	Complete  bool         `json:"complete"`
	Gates     []GateResult `json:"gates"`
	// Error는 파일 읽기/파싱 실패 원문이다.
	Error string `json:"error,omitempty"`
}

// CheckResult는 gates check/status의 응답이다.
type CheckResult struct {
	OK             bool         `json:"ok"`
	SchemaVersion  int          `json:"schema_version"`
	StatusOnly     bool         `json:"status_only"`
	Complete       bool         `json:"complete"`
	TotalGates     int          `json:"total_gates"`
	TotalMet       int          `json:"total_met"`
	TotalUnmet     int          `json:"total_unmet"`
	TotalAbandoned int          `json:"total_abandoned"`
	Files          []FileResult `json:"files"`
	Warnings       []string     `json:"warnings,omitempty"`
	GeneratedAt    string       `json:"generated_at"`
}

// InitRequest는 게이트 파일 스캐폴드 요청이다.
type InitRequest struct {
	File  string   `json:"file"`
	Scope string   `json:"scope"`
	Gates []string `json:"gates"` // "G1: outcome | CHECK: cmd | EXPECT: expect" 형식
}

// InitResult는 gates init의 응답이다.
type InitResult struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	File          string `json:"file"`
	GateCount     int    `json:"gate_count"`
	Created       bool   `json:"created"`
	Error         string `json:"error,omitempty"`
}

// AbandonRequest는 게이트 포기 기록 요청이다.
type AbandonRequest struct {
	File   string `json:"file"`
	GateID string `json:"gate_id"`
	Reason string `json:"reason"`
}

// AbandonResult는 gates abandon의 응답이다.
type AbandonResult struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	File          string `json:"file"`
	GateID        string `json:"gate_id"`
	Recorded      bool   `json:"recorded"`
	Error         string `json:"error,omitempty"`
}

// LintRequest는 게이트를 실행하지 않고 원장 규칙·command policy·EXPECT 형식으로
// 검사하는 요청이다. Gates(init과 같은 spec), PlanFile(계획 문서 안의 게이트 줄),
// Files(기존 원장) 중 하나 이상을 준다. 정책 옵션은 gates check와 같은 의미다.
type LintRequest struct {
	WorkspaceRoot  string   `json:"workspace_root"`
	CWD            string   `json:"cwd"`
	Gates          []string `json:"gates,omitempty"`
	PlanFile       string   `json:"plan_file,omitempty"`
	Files          []string `json:"files,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	EnvAllowlist   []string `json:"env_allowlist,omitempty"`
	WriteAllowed   bool     `json:"write_allowed"`
	NetworkAllowed bool     `json:"network_allowed"`
}

// LintGate는 게이트 하나의 검사 결과다. Problems가 있으면 gates check에서
// 구현과 무관하게 met이 될 수 없다. Warnings는 의도와 다르게 읽힐 수 있는 형식이다.
type LintGate struct {
	Source       string   `json:"source"` // "spec", "<plan file>:<line>", 또는 원장 파일
	ID           string   `json:"id"`
	Check        string   `json:"check"`
	Argv         []string `json:"argv,omitempty"`
	Expect       string   `json:"expect"`
	PolicyDenied bool     `json:"policy_denied,omitempty"`
	Problems     []string `json:"problems,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

// LintResult는 gates lint의 응답이다. Clean은 모든 게이트에 Problems가 없고, 계획에서
// 게이트처럼 보이지만 읽지 못한 줄(UnparsedLines)도 없다는 뜻이다. 읽지 못한 게이트는
// gates init으로 옮겨 적을 수 없고 검사되지도 않았다.
type LintResult struct {
	OK                bool       `json:"ok"`
	SchemaVersion     int        `json:"schema_version"`
	Clean             bool       `json:"clean"`
	TotalGates        int        `json:"total_gates"`
	GatesWithProblems int        `json:"gates_with_problems"`
	Gates             []LintGate `json:"gates"`
	UnparsedLines     []string   `json:"unparsed_lines,omitempty"` // "<plan file>:<line>"
	Warnings          []string   `json:"warnings,omitempty"`
}
