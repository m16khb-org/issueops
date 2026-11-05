// Package promotecmd implements the `issueops self-verify promote` command,
// which promotes a saved summary snapshot to a baseline state key.
package promotecmd

import (
	"flag"
	"fmt"

	"issueops/cmd/issueops/cliflag"
	contract "issueops/internal/contract/selfaugment"
)

// Deps injects the promote operation and JSON output used by Run.
type Deps struct {
	Promote   func(fromKey, baselineKey string, confirm, allowFailedSource bool) (contract.PromoteResult, error)
	PrintJSON func(any) error
}

// Run parses the promote flags and promotes --from-key to --baseline-key. It
// only writes with --confirm; otherwise it reports what would be promoted.
func Run(args []string, deps Deps) error {
	fs := flag.NewFlagSet("self-verify promote", flag.ContinueOnError)
	fromKey := fs.String("from-key", "", "state key containing the candidate self-verification summary snapshot")
	baselineKey := fs.String("baseline-key", "", "state key to write as the promoted baseline")
	confirm := fs.Bool("confirm", false, "write baseline-key; omitted means dry-run")
	allowFailedSource := fs.Bool("allow-failed-source", false, "promote even when the source snapshot did not pass the gate (baseline-poisoning override; off by default)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.ParseNoArgs(fs, args); err != nil {
		return err
	}
	result, err := deps.Promote(*fromKey, *baselineKey, *confirm, *allowFailedSource)
	if err != nil {
		return err
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	action := "would promote"
	if result.Confirm {
		action = "promoted"
	}
	fmt.Printf("%s self-verification summary %q to baseline %q (source_passed=%v)\n", action, result.FromKey, result.BaselineKey, result.SourcePassed)
	return nil
}
