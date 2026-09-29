package probe

import (
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

const selfVerifyCommandOutputBudgetBytes = 32 * 1024
const selfVerifyAggregateOutputBudgetBytes = 8 * 1024

type StepResult = verifycontract.StepResult
type SelfAugmentCompareResult = augmentcontract.SelfAugmentCompareResult
type SelfAugmentSlowStepRegression = augmentcontract.SelfAugmentSlowStepRegression
type SelfAugmentStateSnapshot = augmentcontract.SelfAugmentStateSnapshot
type SelfAugmentStateCheckpoint = augmentcontract.SelfAugmentStateCheckpoint
type SelfAugmentStepBudgetRegression = augmentcontract.SelfAugmentStepBudgetRegression
type SelfAugmentSummary = augmentcontract.SelfAugmentSummary
type SelfVerificationCandidateExportResult = augmentcontract.SelfVerificationCandidateExportResult
type SelfVerificationCandidate = verifycontract.SelfVerificationCandidate
type SelfVerificationCandidateExportStateSnapshot = augmentcontract.SelfVerificationCandidateExportStateSnapshot

const selfVerificationCandidateExportKind = augmentcontract.SelfVerificationCandidateExportKind
const selfVerificationKoreanName = augmentcontract.SelfVerificationKoreanName
const selfVerificationSummaryKind = augmentdomain.SelfVerificationSummaryKind
const selfAugmentCandidateStatusSatisfied = augmentcontract.CandidateStatusSatisfied

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return verification.Run(dir, label, timeout, stdin, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
	return verification.RunEnv(dir, label, timeout, stdin, env, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnvWithBudget(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) StepResult {
	return verification.RunEnv(dir, label, timeout, stdin, env, outputBudget, name, args...)
}

func combineFailedStep(label string, started time.Time, child StepResult, stdoutParts []string, commands []string) StepResult {
	return verifydomain.CombineFailedStep(label, time.Since(started).Milliseconds(), child, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) StepResult {
	return verifydomain.AssertionStepWithOutput(label, time.Since(started).Milliseconds(), errs, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func assertionStep(label string, started time.Time, errs []string) StepResult {
	return verifydomain.AssertionStep(label, time.Since(started).Milliseconds(), errs)
}

func failedStep(label string, err error) StepResult {
	return verifydomain.FailedStep(label, err)
}

func writeSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	if WriteSnapshot == nil {
		return fmt.Errorf("self-verification snapshot writer dependency is required")
	}
	return WriteSnapshot(dir, key, snapshot)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return verifydomain.TailWithBudget(s, max)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// WriteSnapshot is installed by the composition root.
var WriteSnapshot func(string, string, SelfAugmentStateSnapshot) error
