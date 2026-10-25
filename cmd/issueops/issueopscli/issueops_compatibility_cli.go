package issueopscli

import (
	"flag"
	"fmt"
	"issueops/cmd/issueops/cliflag"
	reviewcontract "issueops/internal/contract/issueopsreview"
)

func (cli command) runIssueOpsCompatibility(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Print(catalogHelp("compatibility review"))
		return nil
	}
	if args[0] != "review" {
		return cliflag.UnknownSubcommand("issueops compatibility", args[0], "review")
	}
	fs := flag.NewFlagSet("issueops compatibility review", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := cli.addIssueOpsActorFlags(fs)
	rollbackPlan := fs.String("rollback-plan", "", "rollback plan if compatibility or side effects break")
	approved := fs.Bool("approved", false, "approve compatibility and side-effect review")
	jsonOut := fs.Bool("json", false, "print JSON")
	var backwardCompatibility repeatedFlag
	var sideEffects repeatedFlag
	var verification repeatedFlag
	var blockers repeatedFlag
	fs.Var(&backwardCompatibility, "backward-compatibility", "backward compatibility finding (repeatable)")
	fs.Var(&sideEffects, "side-effect", "side-effect finding (repeatable)")
	fs.Var(&verification, "verification", "verification evidence (repeatable)")
	fs.Var(&blockers, "blocker", "unresolved compatibility blocker (repeatable)")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := cli.Runtime.RecordIssueOpsCompatibilityReviewWithActor(cli.Runtime.IssueOpsStateRoot(), *id, reviewcontract.CompatibilityReviewRequest{
		BackwardCompatibility: backwardCompatibility,
		SideEffects:           sideEffects,
		RollbackPlan:          *rollbackPlan,
		Verification:          verification,
		Blockers:              blockers,
		Approved:              *approved,
	}, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}
