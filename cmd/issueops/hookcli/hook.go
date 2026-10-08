package hookcli

import (
	"flag"
	"fmt"
	"os"

	"issueops/cmd/issueops/hookcli/hookcatalog"
	"issueops/cmd/issueops/hookcli/hookenv"
)

// hookDisabled reports whether ISSUEOPS_DISABLE_HOOKS turns this invocation into
// a no-op. The switch exists so a single host-level hook registration can stay
// installed while the agent works in repositories the harness does not own.
func hookDisabled() bool {
	return hookenv.Bool("ISSUEOPS_DISABLE_HOOKS")
}

// RunHook dispatches the host lifecycle context hooks. Only SessionStart,
// SubagentStart, and PostCompact exist: each reads the static project-doc
// catalog and emits a host-compatible context payload. They never touch durable
// issueops state, telemetry, or IssueOps authority (ADR 2026-08-10, 2026-08-27,
// 2026-10-08).
func RunHook(args []string, config hookcatalog.Config) error {
	if hookDisabled() {
		return nil
	}
	if len(args) == 0 {
		hookUsage()
		return fmt.Errorf("missing hook subcommand")
	}
	switch args[0] {
	case "--help", "-h", "help":
		hookUsage()
		return flag.ErrHelp
	case "session-start":
		return hookcatalog.RunSessionStart(args[1:], config)
	case "subagent-start":
		return hookcatalog.RunSubagentStart(args[1:], config)
	case "post-compact":
		return hookcatalog.RunPostCompact(args[1:], config)
	default:
		hookUsage()
		return fmt.Errorf("unknown hook subcommand %q", args[0])
	}
}

func hookUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  issueops hook session-start [--repo PATH] [--host codex|claude] [--json]
  issueops hook subagent-start [--repo PATH] [--host codex|claude] [--json]
  issueops hook post-compact [--repo PATH] [--host codex|claude] [--json]

session-start renders the static project-doc catalog for every SessionStart
source, including the post-compaction re-run. subagent-start gives a starting
subagent the same model-facing catalog, except Explore, explorer, and fork
agents. post-compact keeps the catalog reachable for hosts without a
SessionStart re-run (Omo) and for diagnosis. ISSUEOPS_DISABLE_HOOKS=1 turns
every hook into a no-op.
`)
}
