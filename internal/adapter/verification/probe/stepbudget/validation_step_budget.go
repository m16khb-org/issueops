package stepbudget

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	verification "issueops/internal/adapter/verification"
	augmentcontract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
	augmentdomain "issueops/internal/domain/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024

type StepBudgetCommandRunner func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) verifycontract.StepResult
type StepBudgetSnapshotWriter func(dir, key string, snapshot augmentcontract.SelfAugmentStateSnapshot) error

type StepBudgetValidationDeps struct {
	MakeTempState func(seed int64) (string, error)
	RemoveAll     func(path string) error
	WriteSnapshot StepBudgetSnapshotWriter
	Run           StepBudgetCommandRunner
}

func (deps StepBudgetValidationDeps) withDefaults() StepBudgetValidationDeps {
	if deps.MakeTempState == nil {
		deps.MakeTempState = func(seed int64) (string, error) {
			return os.MkdirTemp("", fmt.Sprintf("issueops-budget-%d-*", seed))
		}
	}
	if deps.RemoveAll == nil {
		deps.RemoveAll = os.RemoveAll
	}
	if deps.Run == nil {
		deps.Run = func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) verifycontract.StepResult {
			return verification.RunEnv(dir, label, timeout, stdin, env, commandOutputBudgetBytes, name, args...)
		}
	}
	return deps
}

func ValidateStepBudgetBaselineWithDeps(binary, root string, seed int64, deps StepBudgetValidationDeps) verifycontract.StepResult {
	deps = deps.withDefaults()
	if deps.WriteSnapshot == nil {
		return verifydomain.FailedStep("step budget baseline", fmt.Errorf("self-verification snapshot writer dependency is required"))
	}
	started := time.Now()
	tempState, err := deps.MakeTempState(seed)
	if err != nil {
		return verifydomain.FailedStep("step budget baseline", err)
	}
	defer func() { _ = deps.RemoveAll(tempState) }()
	baselineKey := fmt.Sprintf("self-verify-budget-baseline-%d", seed)
	candidateKey := fmt.Sprintf("self-verify-budget-candidate-%d", seed)
	baselineSummary, candidateSummary := StepBudgetBaselineSummaries(seed)
	for _, fixture := range []struct {
		key     string
		summary augmentcontract.SelfAugmentSummary
	}{
		{key: baselineKey, summary: baselineSummary},
		{key: candidateKey, summary: candidateSummary},
	} {
		if err := deps.WriteSnapshot(tempState, fixture.key, StepBudgetStateSnapshot(root, seed, fixture.summary)); err != nil {
			return verifydomain.FailedStep("step budget baseline", err)
		}
	}

	env := []string{"ISSUEOPS_STATE_DIR=" + tempState}
	compareStep := deps.Run(root, "step budget baseline", 30*time.Second, "", env, binary, "self-verify", "compare", "--baseline-key", baselineKey, "--candidate-key", candidateKey, "--max-elapsed-regression-pct", "5", "--json")
	stdoutParts := []string{compareStep.Stdout}
	commands := []string{compareStep.Command}
	if !compareStep.OK {
		return verifydomain.CombineFailedStep("step budget baseline", time.Since(started).Milliseconds(), compareStep, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	var result augmentcontract.SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(compareStep.Stdout), &result); err != nil {
		return verifydomain.AssertionStepWithOutput("step budget baseline", time.Since(started).Milliseconds(), []string{err.Error()}, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	errs := StepBudgetValidationErrors(result)
	if len(errs) > 0 {
		return verifydomain.AssertionStepWithOutput("step budget baseline", time.Since(started).Milliseconds(), errs, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	stdoutText, stdoutTruncated, stdoutBytes := verifydomain.TailWithBudget(strings.Join(stdoutParts, "\n"), aggregateOutputBudgetBytes)
	return verifycontract.StepResult{
		Label:           "step budget baseline",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func StepBudgetBaselineSummaries(seed int64) (augmentcontract.SelfAugmentSummary, augmentcontract.SelfAugmentSummary) {
	baselineSummary := augmentcontract.SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "docs index smoke"},
		SlowestSteps: []augmentcontract.SelfAugmentSlowStep{
			{Iteration: 1, Seed: seed, Label: "go test", DurationMS: 2000},
		},
		StepDurationStats: []augmentcontract.SelfAugmentStepDurationStat{
			{Label: "docs index smoke", Count: 10, MinDurationMS: 90, MaxDurationMS: 100, AverageDurationMS: 95, P95DurationMS: 100},
			{Label: "go test", Count: 10, MinDurationMS: 1800, MaxDurationMS: 2000, AverageDurationMS: 1900, P95DurationMS: 2000},
		},
	}
	candidateSummary := baselineSummary
	candidateSummary.StepDurationStats = []augmentcontract.SelfAugmentStepDurationStat{
		{Label: "docs index smoke", Count: 10, MinDurationMS: 90, MaxDurationMS: 130, AverageDurationMS: 105, P95DurationMS: 130},
		{Label: "go test", Count: 10, MinDurationMS: 1800, MaxDurationMS: 2000, AverageDurationMS: 1900, P95DurationMS: 2000},
	}
	return baselineSummary, candidateSummary
}

func StepBudgetStateSnapshot(root string, seed int64, summary augmentcontract.SelfAugmentSummary) augmentcontract.SelfAugmentStateSnapshot {
	return augmentcontract.SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          augmentdomain.SelfVerificationSummaryKind,
		LoopKind:      "self_verification",
		KoreanName:    augmentcontract.SelfVerificationKoreanName,
		OK:            true,
		Iterations:    10,
		BaseSeed:      seed,
		TargetScore:   95,
		ElapsedMS:     1000,
		IssueOpsRoot:  root,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       summary,
	}
}

func StepBudgetValidationErrors(result augmentcontract.SelfAugmentCompareResult) []string {
	errs := []string{}
	if !result.OK || !result.Regressed {
		errs = append(errs, "step budget compare did not report a regression")
	}
	if len(result.SlowStepRegressions) != 0 {
		errs = append(errs, "step budget regression should not depend on slowest_steps top entries")
	}
	if len(result.StepBudgetRegressions) != 1 {
		errs = append(errs, "step budget compare did not report exactly one budget regression")
	} else {
		regression := result.StepBudgetRegressions[0]
		if regression.Label != "docs index smoke" || regression.Metric != "p95_duration_ms" || regression.DeltaMS != 30 || regression.DeltaPct != 30 {
			errs = append(errs, "step budget regression details mismatch")
		}
	}
	if !containsString(result.Regressions, "step_budget:docs index smoke_p95_increased_by_30.00_pct") {
		errs = append(errs, "step budget regression marker missing")
	}
	return errs
}
