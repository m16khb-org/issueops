package cli

// Command describes a stable top-level CLI command exposed by the harness.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

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
		{Name: "mcp", Description: "serve the MCP stdio proxy and clean up proxy processes"},
		{Name: "model", Description: "show, set, and resolve per-role agent models for claude and codex"},
		{Name: "version", Description: "print issueops version"},
	}
	return commands
}
