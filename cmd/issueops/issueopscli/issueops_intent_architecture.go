package issueopscli

import (
	"flag"
	"fmt"
	reviewcontract "issueops/internal/contract/issueopsreview"

	issueopscontract "issueops/internal/contract/issueops"
)

func (cli command) runIssueOpsIntent(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: issueops intent record --id ID --raw-request TEXT --interpreted-intent TEXT --success-criteria TEXT [--constraint TEXT] [--ambiguity TEXT] [--non-goal TEXT] [--intent-class CLASS] [--json]")
		return nil
	}
	if args[0] != "record" {
		return fmt.Errorf("unknown issueops intent subcommand")
	}
	fs := flag.NewFlagSet("issueops intent record", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := cli.addIssueOpsActorFlags(fs)
	rawRequest := fs.String("raw-request", "", "raw user request")
	interpretedIntent := fs.String("interpreted-intent", "", "agent interpretation of user intent")
	var successCriteria repeatedFlag
	var constraints repeatedFlag
	var ambiguities repeatedFlag
	var nonGoals repeatedFlag
	fs.Var(&successCriteria, "success-criteria", "success criterion; repeatable")
	fs.Var(&constraints, "constraint", "constraint; repeatable")
	fs.Var(&ambiguities, "ambiguity", "resolved, deferred, or blocking ambiguity; repeatable")
	fs.Var(&nonGoals, "non-goal", "non-goal; repeatable")
	intentClass := fs.String("intent-class", "", "intent class controlling plan-prep gate strictness: trivial, standard, refactoring, architecture, research")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := cli.Runtime.RecordIssueOpsIntentWithActor(cli.Runtime.IssueOpsStateRoot(), *id, issueopscontract.IssueOpsIntentRecordRequest{
		RawRequest:        *rawRequest,
		InterpretedIntent: *interpretedIntent,
		SuccessCriteria:   []string(successCriteria),
		Constraints:       []string(constraints),
		Ambiguities:       []string(ambiguities),
		NonGoals:          []string(nonGoals),
		IntentClass:       *intentClass,
	}, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func (cli command) runIssueOpsArchitecture(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printIssueOpsArchitectureReviewUsage()
		return nil
	}
	if args[0] != "review" {
		return fmt.Errorf("unknown issueops architecture subcommand")
	}
	if len(args) > 1 && (args[1] == "--help" || args[1] == "-h" || args[1] == "help") {
		printIssueOpsArchitectureReviewUsage()
		return nil
	}
	fs := flag.NewFlagSet("issueops architecture review", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := cli.addIssueOpsActorFlags(fs)
	problemSummary := fs.String("problem-summary", "", "reviewed problem summary")
	proposedArchitecture := fs.String("proposed-architecture", "", "reviewed architecture")
	refactorPlan := fs.String("refactor-plan", "", "refactor plan or boundary decision")
	var alternatives repeatedFlag
	var risks repeatedFlag
	var verification repeatedFlag
	var openQuestions repeatedFlag
	fs.Var(&alternatives, "alternative", "alternative considered; repeatable")
	fs.Var(&risks, "risk", "architecture risk; repeatable")
	fs.Var(&verification, "verification", "verification step; repeatable")
	fs.Var(&openQuestions, "open-question", "open architecture question; repeatable")
	reviewerContext := fs.String("reviewer-context", "", "reviewer context: subagent|inline")
	approved := fs.Bool("approved", false, "mark architecture reviewed and approved for implementation")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := cli.Runtime.RecordIssueOpsArchitectureReviewWithActor(cli.Runtime.IssueOpsStateRoot(), *id, reviewcontract.ArchitectureReviewRequest{
		ProblemSummary:       *problemSummary,
		ProposedArchitecture: *proposedArchitecture,
		RefactorPlan:         *refactorPlan,
		Alternatives:         []string(alternatives),
		Risks:                []string(risks),
		Verification:         []string(verification),
		OpenQuestions:        []string(openQuestions),
		Approved:             *approved,
		ReviewerContext:      *reviewerContext,
	}, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func printIssueOpsArchitectureReviewUsage() {
	fmt.Println("Usage: issueops architecture review --id ID --problem-summary TEXT --proposed-architecture TEXT --verification TEXT --reviewer-context subagent|inline [--refactor-plan TEXT] [--alternative TEXT] [--risk TEXT] [--open-question TEXT] [--approved] [--json]")
	fmt.Println()
	fmt.Println("Approved reviews require --refactor-plan, at least one --alternative, at least one --risk, no --open-question, and one architecture-review evidence verification item.")
	fmt.Printf("Use a verification item such as --verification %q alongside test commands.\n", issueopscontract.IssueOpsArchitectureReviewEvidenceExample)
	fmt.Println("The review binds to the staged architecture artifact (issueops artifact stage --name architecture): its required sections must have content, and editing the document afterwards makes the approval stale.")
	fmt.Println("Approval is recorded with the full architecture review payload; there is no approve-only merge step.")
}
