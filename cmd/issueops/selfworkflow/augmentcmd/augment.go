// Package augmentcmd implements the `issueops self-augment` command: it plans
// improvement cycles, optionally saves the plan, and routes the lesson, verify,
// history, compare, and promote subcommands to their runners.
package augmentcmd

import (
	"flag"
	"fmt"
	"io"
	"os"

	"issueops/cmd/issueops/cliflag"
	app "issueops/internal/application/selfaugment"
	augmentcontract "issueops/internal/contract/selfaugment"
	augmentdomain "issueops/internal/domain/selfaugment"
)

// Deps injects the planning, persistence, and output behavior of Run. Nil
// fields fall back to defaults that either write to stdout or fail with a
// "dependency is required" error.
type Deps struct {
	Output              io.Writer
	RunLesson           func([]string) error
	RunVerify           func([]string) error
	Plan                func(augmentcontract.PlanRequest) augmentcontract.PlanResult
	SavePlan            func(*augmentcontract.PlanResult, string) error
	PrintJSON           func(any) error
	SelectedCandidateID func(*augmentcontract.Candidate) string
	DefaultTargetScore  float64
}

// Run executes `issueops self-augment`. The lesson, verify, history, compare,
// and promote subcommands are forwarded to their runners; otherwise it plans
// --cycles cycles against --target-score, optionally saving with --save-state.
func Run(args []string, deps Deps) error {
	deps = deps.withDefaults()
	if len(args) > 0 {
		switch args[0] {
		case "lesson":
			return deps.RunLesson(args[1:])
		case "verify":
			return deps.RunVerify(args[1:])
		case "history", "compare", "promote":
			return deps.RunVerify(args)
		}
	}
	fs := flag.NewFlagSet("self-augment", flag.ContinueOnError)
	cycles := fs.Int("cycles", 1, "number of autonomous improvement cycles to plan")
	targetScore := fs.Float64("target-score", deps.DefaultTargetScore, "exclusive per-goal score threshold; every concrete goal must score above this value before termination")
	saveState := fs.Bool("save-state", false, "save compact self-augmentation plan to issueops state")
	stateKey := fs.String("state-key", "self-augment-latest", "state key for --save-state")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.ParseNoArgs(fs, args); err != nil {
		return err
	}
	if *cycles < 1 {
		return fmt.Errorf("cycles must be positive")
	}
	if *targetScore < 0 || *targetScore >= 100 {
		return fmt.Errorf("target-score must be >= 0 and < 100")
	}
	result, err := app.PlanAndSave(augmentcontract.PlanRequest{Cycles: *cycles, TargetScore: *targetScore}, *saveState, *stateKey, app.PlanAndSaveDeps{Plan: deps.Plan, Save: deps.SavePlan})
	if err != nil {
		return err
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	printPlanText(deps, result)
	return nil
}

func printPlanText(deps Deps, result augmentcontract.PlanResult) {
	_, _ = fmt.Fprintf(deps.Output, "%s plan: %d candidate(s), selected=%s, termination_eligible=%v\n", result.KoreanName, len(result.Candidates), deps.SelectedCandidateID(result.SelectedCandidate), result.TerminationEligible)
	for _, goal := range result.Goals {
		_, _ = fmt.Fprintf(deps.Output, "- %s score=%.1f target>%.1f passed=%v\n", goal.KoreanName, goal.Score, goal.TargetScore, goal.Passed)
	}
	if result.SelectedCandidate != nil {
		_, _ = fmt.Fprintf(deps.Output, "selected: %s \u2014 %s\n", result.SelectedCandidate.ID, result.SelectedCandidate.Title)
	}
}

func (deps Deps) withDefaults() Deps {
	if deps.Output == nil {
		deps.Output = os.Stdout
	}
	if deps.RunLesson == nil {
		deps.RunLesson = func([]string) error {
			return fmt.Errorf("self-augment lesson runner dependency is required")
		}
	}
	if deps.RunVerify == nil {
		deps.RunVerify = func([]string) error {
			return fmt.Errorf("self-verify runner dependency is required")
		}
	}
	if deps.Plan == nil {
		deps.Plan = func(augmentcontract.PlanRequest) augmentcontract.PlanResult {
			return augmentcontract.PlanResult{OK: false}
		}
	}
	if deps.SavePlan == nil {
		deps.SavePlan = func(*augmentcontract.PlanResult, string) error {
			return fmt.Errorf("self-augment state dependency is required")
		}
	}
	if deps.PrintJSON == nil {
		deps.PrintJSON = func(any) error {
			return fmt.Errorf("JSON printer dependency is required")
		}
	}
	if deps.SelectedCandidateID == nil {
		deps.SelectedCandidateID = augmentdomain.SelectedCandidateID
	}
	if deps.DefaultTargetScore == 0 {
		deps.DefaultTargetScore = 95.0
	}
	return deps
}
