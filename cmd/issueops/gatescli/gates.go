// Package gatescli는 태스크 게이트 ledger CLI의 flag 해석과 출력을 소유한다.
// 저장소 조립은 composition root가 하고(Dependencies), 여기서는 게이트 파일
// 형식이나 policy를 모른다.
package gatescli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"issueops/cmd/issueops/cliflag"
	"issueops/cmd/issueops/jsonout"
	gatescontract "issueops/internal/contract/gates"
)

// UnmetError는 게이트가 남아있는 상태의 종료 오류이다(exit 1).
type UnmetError struct {
	Unmet      int
	Unreadable int
}

func (e UnmetError) Error() string {
	if e.Unreadable > 0 {
		// Probe: a missing --file ended with "0 unmet gate(s) remain",
		// which reads as nothing left to do.
		return fmt.Sprintf("%d gate file(s) could not be read (see the error above); %d unmet gate(s) in the rest", e.Unreadable, e.Unmet)
	}
	return fmt.Sprintf("%d unmet gate(s) remain", e.Unmet)
}

func unmetError(result gatescontract.CheckResult) UnmetError {
	failure := UnmetError{Unmet: result.TotalUnmet}
	for _, file := range result.Files {
		if file.Error != "" {
			failure.Unreadable++
		}
	}
	return failure
}

// UsageError는 잘못된 사용법 오류이다(exit 2).
type UsageError struct{ Message string }

func (e UsageError) Error() string { return e.Message }

// Dependencies는 gates CLI가 필요한 연산을 함수로 받는다.
type Dependencies struct {
	Check   func(gatescontract.CheckRequest) (gatescontract.CheckResult, error)
	Init    func(gatescontract.InitRequest) (gatescontract.InitResult, error)
	Abandon func(gatescontract.AbandonRequest) (gatescontract.AbandonResult, error)
	Lint    func(gatescontract.LintRequest) (gatescontract.LintResult, error)
}

// LintError는 lint가 문제를 찾은 상태의 종료 오류이다(exit 1).
type LintError struct {
	Gates    int
	Unparsed int
}

func (e LintError) Error() string {
	return fmt.Sprintf("%d gate(s) cannot be met as written, %d gate-like plan line(s) could not be read", e.Gates, e.Unparsed)
}

func Run(deps Dependencies, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		gatesUsage()
		return nil
	}
	switch args[0] {
	case "init":
		return runInit(deps, args[1:])
	case "check":
		return runCheck(deps, args[1:], false)
	case "status":
		return runCheck(deps, args[1:], true)
	case "report":
		return runReport(deps, args[1:])
	case "abandon":
		return runAbandon(deps, args[1:])
	case "lint":
		return runLint(deps, args[1:])
	default:
		return UsageError{Message: cliflag.UnknownSubcommand("gates", args[0], "init", "check", "status", "report", "abandon", "lint").Error()}
	}
}

func gatesUsage() {
	fmt.Fprint(os.Stderr, `Usage:
  issueops gates init [--file PATH] --scope TEXT --gate "ID: outcome | CHECK: cmd | EXPECT: expect" [--gate SPEC...] [--json]
  issueops gates check [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--timeout-seconds N] [--env NAME,NAME] [--write] [--network] [--json]
  issueops gates status [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--json]
  issueops gates report [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--json]
  issueops gates abandon --file PATH --gate ID --reason TEXT [--json]
  issueops gates lint [--gate SPEC]... [--plan PATH] [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--env NAME,NAME] [--write] [--network] [--json]

Exit codes: 0 all gates met or honestly abandoned (lint: no problems), 1 unmet gates remain (lint: problems found), 2 usage error.
`)
}

type checkFlags struct {
	WorkspaceRoot  string
	CWD            string
	Files          []string
	TimeoutSeconds int
	EnvAllowlist   string
	WriteAllowed   bool
	NetworkAllowed bool
}

func parseCheckFlags(name string, args []string) (checkFlags, bool, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	var flags checkFlags
	fs.StringVar(&flags.WorkspaceRoot, "workspace-root", "", "workspace root boundary (defaults to --cwd)")
	fs.StringVar(&flags.CWD, "cwd", "", "gate file directory and CHECK working directory (defaults to cwd)")
	fs.Var(&repeatedFlag{target: &flags.Files}, "file", "gate ledger file (repeatable; defaults to .issueops/issues/<n>/gates.md and .issueops/gates/*.md under --cwd)")
	fs.IntVar(&flags.TimeoutSeconds, "timeout-seconds", gatescontract.TimeoutDefaultSeconds, "per-CHECK timeout")
	fs.StringVar(&flags.EnvAllowlist, "env", "HOME,PATH", "comma-separated environment variable allowlist for CHECK commands")
	fs.BoolVar(&flags.WriteAllowed, "write", true, "allow workspace-write commands for CHECK execution")
	fs.BoolVar(&flags.NetworkAllowed, "network", false, "allow network access for CHECK execution")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return flags, false, err
	}
	if fs.NArg() > 0 {
		return flags, false, UsageError{Message: fmt.Sprintf("unexpected positional argument %q", fs.Arg(0))}
	}
	// The flags document "(defaults to cwd)": without either, discovery and
	// the CHECK policy had no directory and refused ("no gate files found",
	// cwd_required), the same default lint already applies.
	if strings.TrimSpace(flags.CWD) == "" && strings.TrimSpace(flags.WorkspaceRoot) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return flags, false, err
		}
		flags.CWD = wd
	}
	flags.EnvAllowlist = strings.Join(splitCSV(flags.EnvAllowlist), ",")
	return flags, *jsonOut, nil
}

func runCheck(deps Dependencies, args []string, statusOnly bool) error {
	flags, jsonOut, err := parseCheckFlags("gates "+map[bool]string{true: "status", false: "check"}[statusOnly], args)
	if err != nil {
		return err
	}
	req := gatescontract.CheckRequest{
		WorkspaceRoot:  flags.WorkspaceRoot,
		CWD:            flags.CWD,
		Files:          flags.Files,
		TimeoutSeconds: flags.TimeoutSeconds,
		EnvAllowlist:   splitCSV(flags.EnvAllowlist),
		WriteAllowed:   flags.WriteAllowed,
		NetworkAllowed: flags.NetworkAllowed,
		StatusOnly:     statusOnly,
	}
	result, err := deps.Check(req)
	if jsonOut {
		if err != nil {
			return jsonout.PrintResultError(os.Stdout, result, classifyCheckError(err))
		}
		if printErr := printJSON(result); printErr != nil {
			return printErr
		}
	} else if err == nil {
		printCheckText(result)
	}
	if err != nil {
		return classifyCheckError(err)
	}
	if !result.Complete {
		return reportedIf(jsonOut, unmetError(result))
	}
	return nil
}

func runReport(deps Dependencies, args []string) error {
	flags, jsonOut, err := parseCheckFlags("gates report", args)
	if err != nil {
		return err
	}
	result, err := deps.Check(gatescontract.CheckRequest{
		WorkspaceRoot: flags.WorkspaceRoot,
		CWD:           flags.CWD,
		Files:         flags.Files,
		EnvAllowlist:  splitCSV(flags.EnvAllowlist),
		StatusOnly:    true,
	})
	if jsonOut {
		if err != nil {
			return jsonout.PrintResultError(os.Stdout, result, classifyCheckError(err))
		}
		if printErr := printJSON(result); printErr != nil {
			return printErr
		}
	} else if err == nil {
		printReportText(result)
	}
	if err != nil {
		return classifyCheckError(err)
	}
	if !result.Complete {
		return reportedIf(jsonOut, unmetError(result))
	}
	return nil
}

func runInit(deps Dependencies, args []string) error {
	fs := flag.NewFlagSet("gates init", flag.ContinueOnError)
	file := fs.String("file", "", "gate ledger file to create (defaults to .issueops/gates/<scope-slug>.md)")
	scope := fs.String("scope", "", "gate scope name (required; used for the default file slug)")
	var gateSpecs repeatedStrings
	fs.Var(&gateSpecs, "gate", `gate spec: "ID: outcome | CHECK: cmd | EXPECT: expect" (repeatable)`)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.ParseNoArgs(fs, args); err != nil {
		return err
	}
	result, err := deps.Init(gatescontract.InitRequest{File: *file, Scope: *scope, Gates: gateSpecs})
	return printSimpleResult(result, err, *jsonOut)
}

func runAbandon(deps Dependencies, args []string) error {
	fs := flag.NewFlagSet("gates abandon", flag.ContinueOnError)
	file := fs.String("file", "", "gate ledger file (required: .issueops/issues/<n>/gates.md or .issueops/gates/<scope>.md)")
	gateID := fs.String("gate", "", "gate id to abandon (e.g. G2)")
	reason := fs.String("reason", "", "honest abandon reason")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.ParseNoArgs(fs, args); err != nil {
		return err
	}
	result, err := deps.Abandon(gatescontract.AbandonRequest{File: *file, GateID: *gateID, Reason: *reason})
	return printSimpleResult(result, err, *jsonOut)
}

func runLint(deps Dependencies, args []string) error {
	fs := flag.NewFlagSet("gates lint", flag.ContinueOnError)
	var gateSpecs, files repeatedStrings
	fs.Var(&gateSpecs, "gate", `gate spec to lint: "ID: outcome | CHECK: cmd | EXPECT: expect" (repeatable)`)
	plan := fs.String("plan", "", "plan file whose gate lines (one-line specs or ledger form) are linted")
	fs.Var(&files, "file", "existing gate ledger file to lint (repeatable)")
	root := fs.String("workspace-root", "", "workspace root boundary for the policy judgment (defaults to --cwd)")
	cwd := fs.String("cwd", "", "CHECK working directory for the policy judgment (defaults to cwd)")
	env := fs.String("env", "HOME,PATH", "environment variable allowlist the CHECKs will run with")
	write := fs.Bool("write", true, "judge CHECKs as gates check --write would run them")
	network := fs.Bool("network", false, "judge CHECKs as gates check --network would run them")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return UsageError{Message: fmt.Sprintf("unexpected positional argument %q", fs.Arg(0))}
	}
	if strings.TrimSpace(*cwd) == "" && strings.TrimSpace(*root) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		*cwd = wd
	}
	result, err := deps.Lint(gatescontract.LintRequest{
		WorkspaceRoot: *root, CWD: *cwd, Gates: gateSpecs, PlanFile: *plan, Files: files,
		EnvAllowlist: splitCSV(*env), WriteAllowed: *write, NetworkAllowed: *network,
	})
	if err != nil {
		if errors.Is(err, gatescontract.ErrNothingToLint) {
			return UsageError{Message: err.Error()}
		}
		return err
	}
	if *jsonOut {
		if printErr := printJSON(result); printErr != nil {
			return printErr
		}
	} else {
		printLintText(result)
	}
	if !result.Clean {
		return reportedIf(*jsonOut, LintError{Gates: result.GatesWithProblems, Unparsed: len(result.UnparsedLines)})
	}
	return nil
}

func printLintText(result gatescontract.LintResult) {
	for _, gate := range result.Gates {
		mark := "ok  "
		if len(gate.Problems) > 0 {
			mark = "FAIL"
		}
		fmt.Printf("%s %s (%s)\n", mark, gate.ID, gate.Source)
		for _, problem := range gate.Problems {
			fmt.Printf("  problem: %s\n", problem)
		}
		for _, warning := range gate.Warnings {
			fmt.Printf("  warning: %s\n", warning)
		}
	}
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	if result.Clean {
		fmt.Printf("LINT CLEAN (%d gates)\n", result.TotalGates)
		return
	}
	fmt.Printf("LINT PROBLEMS: %d of %d gates, %d unreadable gate line(s)\n", result.GatesWithProblems, result.TotalGates, len(result.UnparsedLines))
}

func classifyCheckError(err error) error {
	if errors.Is(err, gatescontract.ErrNoGateFiles) {
		return UsageError{Message: err.Error()}
	}
	return err
}

func printSimpleResult(result any, err error, jsonOut bool) error {
	if jsonOut {
		return jsonout.PrintResultError(os.Stdout, result, err)
	}
	if err == nil {
		switch value := result.(type) {
		case gatescontract.InitResult:
			fmt.Printf("created %s with %d gates\n", value.File, value.GateCount)
		case gatescontract.AbandonResult:
			fmt.Printf("recorded ABANDON for %s in %s\n", value.GateID, value.File)
		}
	}
	return err
}

// reportedIf marks a verdict error as already shown when its JSON result is on
// stdout: the result carries the verdict, so the root adds nothing.
func reportedIf(jsonOut bool, err error) error {
	if jsonOut {
		return jsonout.Reported(err)
	}
	return err
}

func printCheckText(result gatescontract.CheckResult) {
	for _, file := range result.Files {
		if file.Error != "" {
			fmt.Printf("%s: %s\n", file.File, file.Error)
			continue
		}
		for _, gate := range file.Gates {
			switch gate.State {
			case "met":
				fmt.Printf("  PASS %s: %s\n", gate.ID, gate.Title)
			case "abandoned":
				fmt.Printf("  ABANDONED %s: %s (%s)\n", gate.ID, gate.Title, gate.AbandonReason)
			case "unchecked", "evidence_pending":
				if !result.StatusOnly && gate.CheckError != "" {
					fmt.Printf("  FAIL %s: %s\n       %s\n", gate.ID, gate.Title, gate.CheckError)
				} else {
					fmt.Printf("  UNMET %s (%s): %s\n", gate.ID, gate.State, gate.Title)
				}
			}
		}
		fmt.Printf("%s: %d gates\n", file.File, file.GateCount)
	}
	printSummaryText(result)
}

func printReportText(result gatescontract.CheckResult) {
	for _, file := range result.Files {
		if file.Error != "" {
			fmt.Printf("%s: %s\n", file.File, file.Error)
			continue
		}
		fmt.Printf("%s: %d/%d gates met", file.File, file.Met, file.GateCount)
		if file.Abandoned > 0 {
			fmt.Printf(", %d abandoned", file.Abandoned)
		}
		fmt.Println()
		for _, gate := range file.Gates {
			switch gate.State {
			case "met":
				fmt.Printf("  [x] %s met — evidence: %s\n", gate.ID, gate.Evidence)
			case "abandoned":
				fmt.Printf("  [~] %s abandoned — reason: %s\n", gate.ID, gate.AbandonReason)
			case "unchecked":
				fmt.Printf("  [ ] %s unchecked — %s\n", gate.ID, gate.Title)
			case "evidence_pending":
				fmt.Printf("  [!] %s checked but EVIDENCE pending — %s\n", gate.ID, gate.Title)
			}
		}
	}
	printSummaryText(result)
}

func printSummaryText(result gatescontract.CheckResult) {
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	suffix := ""
	if result.TotalAbandoned > 0 {
		suffix = fmt.Sprintf(", %d abandoned", result.TotalAbandoned)
	}
	if result.Complete {
		fmt.Printf("ALL MET (%d met%s)\n", result.TotalMet, suffix)
		return
	}
	fmt.Printf("UNMET: %d (met: %d%s)\n", result.TotalUnmet, result.TotalMet, suffix)
}

type repeatedStrings []string

func (r *repeatedStrings) String() string { return strings.Join(*r, ", ") }

func (r *repeatedStrings) Set(value string) error {
	*r = append(*r, value)
	return nil
}

type repeatedFlag struct {
	target *[]string
}

func (r repeatedFlag) String() string {
	if r.target == nil {
		return ""
	}
	return strings.Join(*r.target, ", ")
}

func (r repeatedFlag) Set(value string) error {
	*r.target = append(*r.target, value)
	return nil
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

var printJSON = jsonout.Print
