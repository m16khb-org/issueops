package basiccli

import (
	"flag"
	"fmt"

	"issueops/cmd/issueops/cliflag"
)

func (command Command) RunPreflight(args []string) error {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	jsonOut := fs.Bool("json", true, "print JSON")
	target, err := cliflag.ParseOptionalPositional(fs, args)
	if err != nil {
		return err
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
