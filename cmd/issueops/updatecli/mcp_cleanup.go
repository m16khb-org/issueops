package updatecli

import (
	"flag"
	"fmt"
	"os"

	"issueops/cmd/issueops/cliflag"
	"issueops/cmd/issueops/jsonout"
	updateapp "issueops/internal/application/update"
)

type CleanupCommand struct{ Effects updateapp.MCPProxyEffects }

var printJSON = jsonout.Print

func (command CleanupCommand) Run(args []string) error {
	fs := flag.NewFlagSet("mcp cleanup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRunFlag := fs.Bool("dry-run", false, "preview MCP proxy cleanup without terminating processes")
	apply := fs.Bool("apply", false, "terminate matching MCP proxy processes")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
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
		if err != nil {
			if printErr := printJSON(jsonout.WithError(result, err)); printErr != nil {
				return printErr
			}
			return jsonout.Reported(err)
		}
		return printJSON(result)
	}
	fmt.Printf("%s: matched=%d terminated=%d dry_run=%v\n", result.Message, result.Matched, result.Terminated, result.DryRun)
	return err
}
