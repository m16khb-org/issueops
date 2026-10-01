package selfverify

import (
	"errors"
	"fmt"
	"time"

	selfaugmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
	selfverifydomain "issueops/internal/domain/selfverify"
)

var ErrSelfVerificationGateFailed = errors.New("self-verification quality gate failed")

type ProgressReporter interface {
	SetStarted(time.Time)
	Emit(selfverifycontract.ProgressEvent)
	EmitStepEnd(string, int, int, int64, int, int, selfverifycontract.StepResult)
}

type LoopRequest struct {
	BaseSeed        int64
	TargetScore     float64
	Verbose         bool
	Reporter        ProgressReporter
	CollectAllSteps bool
}

type LoopDeps struct {
	IssueOpsRoot   func() string
	StepDeps       SelfVerifyStepDeps
	FailedStep     func(string, error) selfverifycontract.StepResult
	PrintStep      func(selfverifycontract.StepResult)
	Printf         func(string, ...any) (int, error)
	Summarize      func(selfaugmentcontract.SelfAugmentResult, float64) selfaugmentcontract.SelfAugmentSummary
	MkdirTemp      func() (string, error)
	RemoveAll      func(string) error
	TempBinaryPath func(string) string
	Now            func() time.Time
}

func NewLoopResult(iterations int, baseSeed int64, targetScore float64, root string) selfaugmentcontract.SelfAugmentResult {
	return selfaugmentcontract.SelfAugmentResult{
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

func ExecuteLoop(request LoopRequest, deps LoopDeps) (selfaugmentcontract.SelfAugmentResult, error) {
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
	run := selfaugmentcontract.SelfAugmentIteration{Iteration: iteration, Seed: seed}
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
	plannedSteps := PlannedSteps(result.IssueOpsRoot, tempBin, seed, &goTestStep, deps.StepDeps)
	emit(request.Reporter, selfverifycontract.ProgressEvent{Event: "iteration_start", LoopKind: result.LoopKind, Iteration: iteration, Iterations: iterations, Seed: seed, StepCount: len(plannedSteps)})
	failed := false
	var firstFailure selfverifycontract.StepResult
	for index, plannedStep := range plannedSteps {
		emit(request.Reporter, selfverifycontract.ProgressEvent{Event: "step_start", LoopKind: result.LoopKind, Iteration: iteration, Iterations: iterations, Seed: seed, StepIndex: index + 1, StepCount: len(plannedSteps), Step: plannedStep.Label})
		step := plannedStep.Run()
		run.Steps = append(run.Steps, step)
		if request.Reporter != nil {
			request.Reporter.EmitStepEnd(result.LoopKind, iteration, iterations, seed, index+1, len(plannedSteps), step)
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
	result.Runs = append(result.Runs, run)
	result.ElapsedMS = deps.Now().Sub(started).Milliseconds()
	if failed {
		result.OK = false
		result.Summary = deps.Summarize(result, request.TargetScore)
		message := fmt.Sprintf("%s failed: %s", firstFailure.Label, firstFailure.Error)
		emitEnd(request.Reporter, result.LoopKind, iterations, request.BaseSeed, false, message)
		return result, fmt.Errorf("%w: %s", ErrSelfVerificationGateFailed, message)
	}
	emit(request.Reporter, selfverifycontract.ProgressEvent{Event: "iteration_end", LoopKind: result.LoopKind, Iteration: iteration, Iterations: iterations, Seed: seed, OK: boolPtr(true), StepCount: len(plannedSteps)})
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

func emit(reporter ProgressReporter, event selfverifycontract.ProgressEvent) {
	if reporter != nil {
		reporter.Emit(event)
	}
}

func emitEnd(reporter ProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	event := selfverifycontract.ProgressEvent{Event: "loop_end", LoopKind: loopKind, Iterations: iterations, Seed: seed, OK: boolPtr(ok), Error: errorText}
	emit(reporter, event)
}

func boolPtr(value bool) *bool { return &value }
