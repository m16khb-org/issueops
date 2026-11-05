// Package cli 패키지는 `issueops` CLI의 공개 표면(최상위 명령 목록과 usage 카탈로그)을
// 정의한다. 계약 계층에 속하며 명령 실행 로직은 담지 않는다.
package cli

// Command describes a stable top-level CLI command exposed by the harness.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RootCommands 함수는 하네스가 노출하는 최상위 CLI 명령을 표시 순서대로 돌려준다.
// 호출할 때마다 새 슬라이스를 만든다.
func RootCommands() []Command {
	commands := []Command{
		{Name: "inspect", Description: "inspect harness installation and native integration"},
		{Name: "preflight", Description: "run read-only git preflight checks"},
		{Name: "system-status", Description: "summarize doctor, state, worker, and verification status"},
		{Name: "doctor", Description: "diagnose harness installation, state, hooks, MCP, and project docs"},
		{Name: "docs", Description: "index harness guidance documents"},
		{Name: "policy", Description: "evaluate command policy, fake-run commands, and write audit records"},
		{Name: "guard", Description: "check language-agnostic code and test anti-patterns"},
		{Name: "quality", Description: "inspect quality signals and next improvement candidates"},
		{Name: "verify-work", Description: "run a lightweight evidence matrix for current work"},
		{Name: "trace", Description: "analyze trace-like verification and lifecycle evidence"},
		{Name: "contract", Description: "print or check CLI/MCP response compatibility contracts"},
		{Name: "state", Description: "read and write small agent state checkpoints"},
		{Name: "api-doc", Description: "run API documentation static and agent review gates"},
		{Name: "hook", Description: "run host lifecycle context hooks"},
		{Name: "project", Description: "bootstrap and maintain project operating docs"},
		{Name: "install", Description: "install shared native skills and MCP config"},
		{Name: "update", Description: "rebuild and refresh user-level integrations"},
		{Name: "bootstrap", Description: "set up user-level integrations"},
		{Name: "worker", Description: "manage safe local worker jobs and read-only command evidence"},
		{Name: "loop", Description: "track durable verify-until-done loop contracts"},
		{Name: "gates", Description: "evaluate unlazy-compatible task gate ledgers with policy-gated checks"},
		{Name: "channel", Description: "exchange durable cross-session messages through shared issueops state"},
		{Name: "web-fetch", Description: "fetch public web pages with resilient validation and run deterministic web-fetch benchmarks"},
		{Name: "self-verify", Description: "run harness verification gates"},
		{Name: "self-augment", Description: "plan self-augmentation candidates and lessons"},
		{Name: "mcp", Description: "serve MCP over stdio or shared HTTP, manage the HTTP service, authorize callers, and clean up proxy processes"},
		{Name: "model", Description: "show, set, unset, and resolve per-role agent models for claude, codex, omo, omp, and opencode"},
		{Name: "version", Description: "print issueops version"},
	}
	return commands
}
