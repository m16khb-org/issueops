package issueopscli

import (
	"flag"
	"fmt"
	reviewcontract "issueops/internal/contract/issueopsreview"
)

func (cli command) runIssueOpsDevilsAdvocate(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Print(catalogHelp("devils-advocate review"))
		return nil
	}
	if args[0] != "review" {
		return fmt.Errorf("unknown issueops devils-advocate subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("issueops devils-advocate review", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := cli.addIssueOpsActorFlags(fs)
	verdict := fs.String("verdict", "", "devil's-advocate verdict: pass|revise|stop")
	reviewerContext := fs.String("reviewer-context", "", "how the review ran: subagent|inline (audit field, required)")
	reviewerHost := fs.String("reviewer-host", "", "reviewer host (audit only)")
	reviewerModel := fs.String("reviewer-model", "", "model the review ran with (audit only)")
	reviewerEffort := fs.String("reviewer-effort", "", "effort the review ran with, or unknown (audit only)")
	waive := fs.Bool("waive", false, "explicitly waive a stop/revise verdict")
	rationale := fs.String("waiver-rationale", "", "rationale required when --waive is set")
	jsonOut := fs.Bool("json", false, "print JSON")
	var findings repeatedFlag
	fs.Var(&findings, "finding", "surfaced problem (repeatable)")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	if *reviewerContext == "" {
		return fmt.Errorf("--reviewer-context subagent|inline is required: record how the devil's-advocate review ran")
	}
	record, err := cli.Runtime.RecordIssueOpsDevilsAdvocateReviewWithActor(cli.Runtime.IssueOpsStateRoot(), *id, reviewcontract.DevilsAdvocateReviewRequest{
		Verdict:         *verdict,
		Findings:        findings,
		Waived:          *waive,
		WaiverRationale: *rationale,
		ReviewerContext: *reviewerContext,
		ReviewerHost:    *reviewerHost, ReviewerModel: *reviewerModel, ReviewerEffort: *reviewerEffort,
	}, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}
