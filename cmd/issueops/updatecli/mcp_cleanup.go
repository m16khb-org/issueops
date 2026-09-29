package updatecli

import (
	"encoding/json"
	"flag"
	"fmt"
	updateapp "issueops/internal/application/update"
	"os"
)

type CleanupCommand struct{ Effects updateapp.MCPProxyEffects }

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func (command CleanupCommand) Run(args []string) error {
	fs := flag.NewFlagSet("mcp cleanup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRunFlag := fs.Bool("dry-run", false, "preview MCP proxy cleanup without terminating processes")
	apply := fs.Bool("apply", false, "terminate matching MCP proxy processes")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected mcp cleanup argument %q", fs.Arg(0))
	}
	if *dryRunFlag && *apply {
		return fmt.Errorf("--dry-run and --apply cannot be used together")
	}

	result, err := updateapp.CleanupMCPProxies(command.Effects, !*apply)
	if *jsonOut {
		if printErr := printJSON(result); printErr != nil {
			return printErr
		}
		return err
	}
	fmt.Printf("%s: matched=%d terminated=%d dry_run=%v\n", result.Message, result.Matched, result.Terminated, result.DryRun)
	return err
}
