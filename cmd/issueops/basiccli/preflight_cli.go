package basiccli

import (
	"flag"
	"fmt"
)

func (command Command) RunPreflight(args []string) error {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	jsonOut := fs.Bool("json", true, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := ""
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}
	if command.Preflight.Observer == nil {
		return fmt.Errorf("git preflight is not configured")
	}
	if target == "" {
		target = command.DefaultTarget
	}
	result := command.Preflight.Check(target, command.IssueOpsRoot)
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(result)
}
