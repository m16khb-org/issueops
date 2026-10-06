package projectcli

import (
	"fmt"
	"os"
)

func Run(deps Dependencies, args []string) error {
	if len(args) == 0 {
		projectUsage()
		return fmt.Errorf("missing project subcommand")
	}
	switch args[0] {
	case "bootstrap":
		return runProjectBootstrap(deps.Bootstrap, args[1:])
	case "docs":
		return runProjectDocs(deps.Docs, args[1:])
	case "route-docs":
		return runProjectRouteDocs(deps.Docs, args[1:])
	case "append":
		return runProjectAppend(deps.Docs, args[1:])
	case "commit-suggest":
		return runProjectCommitSuggest(deps.Commit, args[1:])
	case "lint-diagnose":
		return runProjectLintDiagnose(deps.Lint, args[1:])
	default:
		projectUsage()
		return fmt.Errorf("unknown project subcommand %q", args[0])
	}
}

func projectUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  issueops project bootstrap [--repo PATH] [--sync] [--dry-run] [--json]
  issueops project docs [--repo PATH] [--json]
  issueops project route-docs [--repo PATH] [--task TEXT] [--json]
  issueops project append --kind caution|adr --title TEXT --summary TEXT [--repo PATH] [--json]
  issueops project commit-suggest [--repo PATH] [--staged] [--json]
  issueops project lint-diagnose [--repo PATH] [--json] -- <command_to_run...>
`)
}
