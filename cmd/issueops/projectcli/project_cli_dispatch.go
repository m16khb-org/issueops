package projectcli

import (
	"flag"
	"fmt"
	"os"

	"issueops/cmd/issueops/cliflag"
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
	case "-h", "--help", "help":
		projectUsage()
		return flag.ErrHelp
	default:
		return cliflag.UnknownSubcommand("project", args[0], "bootstrap", "docs", "route-docs", "append", "commit-suggest", "lint-diagnose")
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
