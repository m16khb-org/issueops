// Package selfverify is the application layer of the self-verification loop: it runs the planned QA
// steps in order, summarizes their results into goal scores and failure classification, optionally
// applies the LLM evaluation gate, and saves summaries and candidate exports to state. Process,
// filesystem and state access is injected through the Deps structs.
package selfverify

import (
	"errors"
	"fmt"
	"time"

	selfaugmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
	selfverifydomain "issueops/internal/domain/selfverify"
)

// ErrSelfVerificationGateFailed is wrapped by the error ExecuteLoop returns when a planned step
// fails.
var ErrSelfVerificationGateFailed = errors.New("self-verification quality gate failed")

// ProgressReporter receives structured progress events as the loop starts, runs steps and ends.
type ProgressReporter interface {
	SetStarted(time.Time)
	Emit(selfverifycontract.ProgressEvent)
	EmitStepEnd(string, int, int, int64, int, int, selfverifycontract.StepResult)
}

// LoopRequest configures one ExecuteLoop run: the base ref and seed, the target score, verbose
// printing, an optional progress reporter, and whether to keep running steps after a failure.
type LoopRequest struct {
	BaseRef         string
	BaseSeed        int64
	TargetScore     float64
	Verbose         bool
	Reporter        ProgressReporter
	CollectAllSteps bool
}

// LoopDeps injects the root lookup, step dependencies, printing, summarizing, temp directory
// handling and clock that ExecuteLoop uses.
type LoopDeps struct {
	IssueOpsRoot   func() string
	StepDeps       StepDeps
	FailedStep     func(string, error) selfverifycontract.StepResult
	PrintStep      func(selfverifycontract.StepResult)
	Printf         func(string, ...any) (int, error)
	Summarize      func(selfaugmentcontract.Result, float64) selfaugmentcontract.Summary
	MkdirTemp      func() (string, error)
	RemoveAll      func(string) error
	TempBinaryPath func(string) string
	Now            func() time.Time
}

// NewLoopResult returns the empty result of a self-verification loop with its identity fields and
// the loop contract text filled in.
func NewLoopResult(iterations int, baseSeed int64, targetScore float64, root string) selfaugmentcontract.Result {
	return selfaugmentcontract.Result{
		LoopKind:     "self_verification",
		KoreanName:   selfaugmentcontract.SelfVerificationKoreanName,
		Iterations:   iterations,
		BaseSeed:     baseSeed,
		TargetScore:  targetScore,
		IssueOpsRoot: root,
		InspiredBy:   "/Users/sample/workspace/eye-tracking-scroll/scripts/self-augment.js",
		LoopContract: []string{
			"run one deterministic evidence pass using the supplied seed; no quick/full or repeated-iteration modes",
			"LLM evaluation is opt-in and renders a read-only evaluator prompt without an external request or ingested verdict; gate mode remains non-passing",
			"tests and QA are first-class stages, not optional follow-ups",
			"seeded randomized git preflight fuzz within the single pass",
			"check core invariant, tests, risk-tier QA, build, CLI/MCP schema and response contract golden, CLI, docs, command policy, MCP, state, and native integration smoke checks",
			"terminate only when every concrete goal score is greater than target_score",
			"fail fast by default; collect-all-steps continues gathering evidence after failure without granting termination",
		},
	}
}

// ExecuteLoop runs one deterministic self-verification evidence pass: it builds the planned steps
// against a temporary binary, runs them in order, and summarizes the outcome. A failed step returns
// the result together with an error wrapping ErrSelfVerificationGateFailed; later steps still run
// when CollectAllSteps is set, but termination is not granted.
func ExecuteLoop(request LoopRequest, deps LoopDeps) (selfaugmentcontract.Result, error) {
	const iterations = 1
	started := deps.Now()
	result := NewLoopResult(iterations, request.BaseSeed, request.TargetScore, deps.IssueOpsRoot())
	if request.Reporter != nil {
		request.Reporter.SetStarted(started)
		emit(request.Reporter, selfverifycontract.ProgressEvent{Event: "loop_start", LoopKind: result.LoopKind, Iterations: iterations, Seed: request.BaseSeed})
	}

	const iteration = 1
	seed := request.BaseSeed
	if request.Verbose {
		_, _ = deps.Printf("\n=== Self-verification evidence pass seed=%d ===\n", seed)
	}
	run := selfaugmentcontract.Iteration{Iteration: iteration, Seed: seed}
	tempDir, err := deps.MkdirTemp()
	if err != nil {
		run.Steps = append(run.Steps, deps.FailedStep("create temp workspace", err))
		result.Runs = append(result.Runs, run)
		result.ElapsedMS = deps.Now().Sub(started).Milliseconds()
		result.Summary = deps.Summarize(result, request.TargetScore)
		emitEnd(request.Reporter, result.LoopKind, iterations, request.BaseSeed, false, err.Error())
		return result, err
	}
	defer func() { _ = deps.RemoveAll(tempDir) }()
	tempBin := deps.TempBinaryPath(tempDir)

	var goTestStep selfverifycontract.StepResult
	stepDeps := deps.StepDeps
	if stepDeps.ValidateRiskQATierWithScope != nil {
		stepDeps.ValidateRiskQATier = func(root string) RiskQAEvidence {
			return deps.StepDeps.ValidateRiskQATierWithScope(root, request.BaseRef)
		}
	}
	plannedSteps := PlannedSteps(result.IssueOpsRoot, tempBin, seed, &goTestStep, stepDeps)
	emit(request.Reporter, selfverifycontract.ProgressEvent{Event: "iteration_start", LoopKind: result.LoopKind, Iteration: iteration, Iterations: iterations, Seed: seed, StepCount: len(plannedSteps)})
	failed, firstFailure := runPlannedSteps(request, deps, result.LoopKind, plannedSteps, seed, iterations, iteration, &run)
	result.Runs = append(result.Runs, run)
	result.ElapsedMS = deps.Now().Sub(started).Milliseconds()
	if failed {
		result.OK = false
		result.Summary = deps.Summarize(result, request.TargetScore)
		message := fmt.Sprintf("%s failed: %s", firstFailure.Label, firstFailure.Error)
		emitEnd(request.Reporter, result.LoopKind, iterations, request.BaseSeed, false, message)
		return result, fmt.Errorf("%w: %s", ErrSelfVerificationGateFailed, message)
	}
	emit(request.Reporter, selfverifycontract.ProgressEvent{Event: "iteration_end", LoopKind: result.LoopKind, Iteration: iteration, Iterations: iterations, Seed: seed, OK: new(true), StepCount: len(plannedSteps)})
	result.OK = true
	result.Summary = deps.Summarize(result, request.TargetScore)
	result.TerminationEligible = result.Summary.TerminationEligible
	result.OK = result.TerminationEligible
	if request.Verbose {
		_, _ = deps.Printf("\nSelf-verification pipeline passed one evidence pass in %.1fs.\n", float64(result.ElapsedMS)/1000)
	}
	emitEnd(request.Reporter, result.LoopKind, iterations, request.BaseSeed, result.OK, "")
	return result, nil
}

// runPlannedSteps executes the planned steps in order, appending each result to run.Steps, and reports whether any step failed and the first failure.
func runPlannedSteps(request LoopRequest, deps LoopDeps, loopKind string, plannedSteps []PlannedStep, seed int64, iterations, iteration int, run *selfaugmentcontract.Iteration) (bool, selfverifycontract.StepResult) {
	failed := false
	var firstFailure selfverifycontract.StepResult
	for index, plannedStep := range plannedSteps {
		emit(request.Reporter, selfverifycontract.ProgressEvent{Event: "step_start", LoopKind: loopKind, Iteration: iteration, Iterations: iterations, Seed: seed, StepIndex: index + 1, StepCount: len(plannedSteps), Step: plannedStep.Label})
		step := plannedStep.Run()
		run.Steps = append(run.Steps, step)
		if request.Reporter != nil {
			request.Reporter.EmitStepEnd(loopKind, iteration, iterations, seed, index+1, len(plannedSteps), step)
		}
		if request.Verbose {
			deps.PrintStep(step)
		}
		if !step.OK {
			if !failed {
				firstFailure = step
			}
			failed = true
			if !selfverifydomain.ContinueAfterFailure(request.CollectAllSteps) {
				break
			}
		}
	}
	return failed, firstFailure
}

func emit(reporter ProgressReporter, event selfverifycontract.ProgressEvent) {
	if reporter != nil {
		reporter.Emit(event)
	}
}

func emitEnd(reporter ProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	event := selfverifycontract.ProgressEvent{Event: "loop_end", LoopKind: loopKind, Iterations: iterations, Seed: seed, OK: new(ok), Error: errorText}
	emit(reporter, event)
}
