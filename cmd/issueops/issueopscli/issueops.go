package issueopscli

import (
	"flag"
	"fmt"
	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	"issueops/cmd/issueops/issueopscli/remotecmd"
	provenanceport "issueops/internal/port/issueopsprovenance"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/domain/commandparse"
)

// issueOpsSubcommands는 `issueops <subcommand>`의 디스패치 레지스트리다.
// 라우팅은 단일 map 조회이므로 subcommand 추가는 분기가 많은 switch를 키우는
// 대신 항목 하나와 핸들러 하나를 더하는 것으로 끝난다.
func (cli command) issueOpsSubcommands(deps Dependencies) map[string]func([]string) error {
	return map[string]func([]string) error{
		"start":                 cli.runIssueOpsStart,
		"status":                cli.runIssueOpsStatus,
		"list":                  cli.runIssueOpsList,
		"review-metrics":        cli.runIssueOpsReviewMetrics,
		"next":                  cli.runIssueOpsNext,
		"intent":                cli.runIssueOpsIntent,
		"plan-prep":             cli.runIssueOpsPlanPrep,
		"design":                cli.runIssueOpsDesign,
		"compatibility":         cli.runIssueOpsCompatibility,
		"devils-advocate":       cli.runIssueOpsDevilsAdvocate,
		"domain-review":         cli.runIssueOpsDomainReview,
		"ai-slop-clean":         cli.runIssueOpsAISlopClean,
		"regress":               cli.runIssueOpsRegress,
		"link-issue":            cli.runIssueOpsLinkIssue,
		"link-plan":             cli.runIssueOpsLinkPlan,
		"link-worktree":         cli.runIssueOpsLinkWorktree,
		"link-child":            cli.runIssueOpsLinkChild,
		"link-related":          cli.runIssueOpsLinkRelated,
		"child":                 func(args []string) error { return cli.runIssueOpsChild(args, deps.ChildUsage) },
		"artifact":              cli.runIssueOpsArtifact,
		"implementation-review": cli.runIssueOpsImplementationReview,
		"project-docs-review":   cli.runIssueOpsProjectDocsReview,
		"schema-evidence":       cli.runIssueOpsSchemaEvidence,
		"branch":                cli.runIssueOpsBranch,
		"phase":                 cli.runIssueOpsPhase,
		"record-routing":        cli.runIssueOpsRecordRouting,
		"routing-score":         cli.runIssueOpsRoutingScore,
		"feedback":              func(args []string) error { return cli.runIssueOpsFeedbackWithDependencies(args, deps) },
		"cleanup":               func(args []string) error { return runIssueOpsCleanupWithDependencies(args, deps) },
		"benchmark":             func(args []string) error { return deps.Benchmark.Run(args) },
		"remote": func(args []string) error {
			return remotecmd.Run(args, issueOpsRemoteDepsWithPublication(deps.Publication))
		},
		"remote-score": func(args []string) error {
			return remotecmd.Run(append([]string{"score"}, args...), issueOpsRemoteDepsWithPublication(deps.Publication))
		},
		"prune":        cli.runIssueOpsPrune,
		"pr-readiness": cli.runIssueOpsPRReadiness,
		"decision":     cli.runIssueOpsDecision,
		"execution":    func(args []string) error { return cli.runIssueOpsExecutionWithDependencies(args, deps) },
	}
}

func (cli command) dispatchIssueOps(args []string, deps Dependencies) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		issueOpsUsage(deps.Usage)
		return nil
	}
	handler, ok := cli.issueOpsSubcommands(deps)[args[0]]
	if !ok {
		return fmt.Errorf("unknown issueops subcommand %q%s", args[0], cli.suggestIssueOpsSubcommand(args[0]))
	}
	return handler(args[1:])
}

func (cli command) runIssueOpsWithDependencies(args []string, deps Dependencies) error {
	clean, generated, err := cli.prepareGeneratedCommandInvocation(args, deps)
	if err == nil && generated {
		err = requireGeneratedOwnerProcessCWD(clean)
	}
	if err != nil {
		if issueOpsJSONRequested(args) {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	args = clean
	return cli.dispatchIssueOps(args, deps)
}

func requireGeneratedOwnerProcessCWD(args []string) error {
	command, ok := commandparse.ParseExactIssueOpsArgs(args)
	if !ok {
		return nil
	}
	flags, ok := commandparse.ExactIssueOpsOwnerMutation(command)
	if !ok {
		return nil
	}
	values := flags["--cwd"]
	if len(values) != 1 {
		return fmt.Errorf("generated owner mutation requires one exact --cwd")
	}
	processCWD, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("observe generated owner mutation actual process cwd: %w", err)
	}
	if !sameExistingIssueOpsPath(processCWD, values[0]) {
		return fmt.Errorf("generated owner mutation actual process cwd must match --cwd before mutation")
	}
	return nil
}

func sameExistingIssueOpsPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(strings.TrimSpace(left))
	rightAbs, rightErr := filepath.Abs(strings.TrimSpace(right))
	if leftErr != nil || rightErr != nil || strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	leftResolved, leftErr := filepath.EvalSymlinks(leftAbs)
	rightResolved, rightErr := filepath.EvalSymlinks(rightAbs)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftResolved) == filepath.Clean(rightResolved)
}

// issueOpsConceptHints는 에이전트가 CLI subcommand으로 자주 오인하는 IssueOps
// 도메인 어휘(lifecycle phase 이름, 결정 동사, ledger artifact 이름)를 매핑한다.
// skill 문서는 생생한 명사(grill, split, domain)를 쓰지만 CLI는 일반 동사(phase,
// remote, link-related)를 쓴다. 이 힌트가 그 이름 간극을 메워, 잘못 추측해도 맨
// "unknown subcommand" 대신 실행 가능한 안내를 내놓는다.
var issueOpsConceptHints = map[string]string{
	"grill":     "did you mean `issueops phase --to grill`? (grill is a lifecycle phase, not a subcommand)",
	"problem":   "did you mean `issueops phase --to problem`? (problem is a lifecycle phase, not a subcommand)",
	"implement": "did you mean `issueops phase --to implement`? (implement is a lifecycle phase, not a subcommand)",
	"split":     "did you mean `issueops remote create-child` or `issueops link-related --type splits-from`? (split is a breakdown decision, not a subcommand)",
}

// suggestIssueOpsSubcommand는 알 수 없는 subcommand에 대한 제안 접미사를 돌려준다.
// 알려진 phase/decision 단어에는 concept hint를, 그 외에는 실제 subcommand
// 레지스트리에 대한 prefix 일치를 쓴다. 쓸 만한 제안이 없으면 ""를 돌려준다.
func (cli command) suggestIssueOpsSubcommand(input string) string {
	if hint, ok := issueOpsConceptHints[input]; ok {
		return "; " + hint
	}
	var matches []string
	for name := range cli.issueOpsSubcommands(Dependencies{}) {
		if strings.HasPrefix(name, input) {
			matches = append(matches, name)
		}
	}
	if len(matches) == 1 {
		return fmt.Sprintf("; did you mean `%s`?", matches[0])
	}
	return ""
}

func issueOpsRemoteDepsWithPublication(publication remotecmd.PublicationHandlers) remotecmd.Deps {
	return remotecmd.Deps{
		PrintJSON:         printJSON,
		PrintResult:       printIssueOpsResult,
		PrintError:        printIssueOpsErrorJSON,
		VerifyLive:        verifyIssueOpsRemoteArtifactLive,
		VerifyLiveContext: verifyIssueOpsRemoteArtifactLiveContext,
		VerifyMerged:      verifyIssueOpsRemoteArtifactMergedLive,
		Publication:       publication,
	}
}

func parseIssueOpsFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func (cli command) runIssueOpsFeedbackWithDependencies(args []string, deps Dependencies) error {
	if len(args) > 0 && args[0] == "resolve" {
		return cli.runIssueOpsFeedbackResolve(args[1:])
	}
	return deps.Cleanup.RunFeedback(args, cleanupTransport(deps.CleanupRuntime, deps.Provenance))
}

func runIssueOpsCleanupWithDependencies(args []string, deps Dependencies) error {
	return deps.Cleanup.RunCleanup(args, cleanupTransport(deps.CleanupRuntime, deps.Provenance))
}

func cleanupTransport(runtime feedbackcleanup.Deps, provenance provenanceport.Observer) feedbackcleanup.Deps {
	runtime.Provenance = provenance
	runtime.ParseFlags = parseIssueOpsFlags
	runtime.PrintResult = printIssueOpsResult
	runtime.PrintJSON = printJSON
	runtime.PrintError = printIssueOpsErrorJSON
	return runtime
}
