package stateroundtrip

import (
	selfaugment "issueops/internal/contract/selfaugment"
	selfverify "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"

	"strings"
	"time"
)

func validateStateRoundtripSelfVerifyDeps(input validateStateRoundtripSelfVerifyInput) selfverify.StepResult {
	session := newStateRoundtripSelfVerifySession(input)
	key := input.key
	baselineCompareKey := key + "-compare-base"
	candidateCompareKey := key + "-compare-candidate"
	if step := session.writeCompareSnapshots(input, baselineCompareKey, candidateCompareKey); !step.OK {
		return step
	}
	if step := session.validateCompare(baselineCompareKey, candidateCompareKey); !step.OK {
		return step
	}

	promotedBaselineKey := key + "-promoted-baseline"
	if step := session.validatePromote(candidateCompareKey, promotedBaselineKey); !step.OK {
		return step
	}
	if step := session.validatePromotedCompare(candidateCompareKey, promotedBaselineKey); !step.OK {
		return step
	}

	if step := session.validateHistoryAndRetention(baselineCompareKey, candidateCompareKey, promotedBaselineKey); !step.OK {
		return step
	}

	stdoutText, stdoutTruncated, stdoutBytes := verifydomain.TailWithBudget(strings.Join(session.stdoutParts, "\n"), aggregateOutputBudgetBytes)
	return selfverify.StepResult{
		Label:           "state roundtrip",
		Command:         strings.Join(session.commands, " && "),
		OK:              true,
		DurationMS:      time.Since(input.started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

// validateCompare는 회귀 없는 비교와 경과 시간 임계값 초과 비교를 순서대로 검증한다.
func (s *stateRoundtripSelfVerifySession) validateCompare(baselineCompareKey, candidateCompareKey string) selfverify.StepResult {
	compareOK, failed := runSelfVerifyJSON[selfaugment.SelfAugmentCompareResult](s, "self verify compare ok", "self-verify", "compare", "--baseline-key", baselineCompareKey, "--candidate-key", candidateCompareKey, "--json")
	if failed != nil {
		return *failed
	}
	if !compareOK.OK || compareOK.Regressed || compareOK.ElapsedDeltaMS != 100 {
		return s.fail("self-verify compare reported unexpected non-regression result")
	}

	compareRegression, failed := runSelfVerifyJSON[selfaugment.SelfAugmentCompareResult](s, "self verify compare regression", "self-verify", "compare", "--baseline-key", baselineCompareKey, "--candidate-key", candidateCompareKey, "--max-elapsed-regression-pct", "5", "--json")
	if failed != nil {
		return *failed
	}
	if !compareRegression.OK || !compareRegression.Regressed || len(compareRegression.Regressions) == 0 {
		return s.fail("self-verify compare did not report expected elapsed regression")
	}
	return selfverify.StepResult{OK: true}
}

// validatePromote는 dry-run이 상태를 바꾸지 않고, confirm이 기준선을 기록하는지 검증한다.
func (s *stateRoundtripSelfVerifySession) validatePromote(candidateCompareKey, promotedBaselineKey string) selfverify.StepResult {
	input := s.input
	promoteDry, failed := runSelfVerifyJSON[selfaugment.SelfAugmentPromoteResult](s, "self verify promote dry-run", "self-verify", "promote", "--from-key", candidateCompareKey, "--baseline-key", promotedBaselineKey, "--json")
	if failed != nil {
		return *failed
	}
	if !promoteDry.OK || !promoteDry.DryRun || promoteDry.Promoted {
		return s.fail("self-verify promote dry-run mutated state or did not report dry-run")
	}
	if _, err := input.deps.stateRead(input.tempState, promotedBaselineKey); err == nil {
		return s.fail("self-verify promote dry-run wrote baseline unexpectedly")
	}

	promoteConfirm, failed := runSelfVerifyJSON[selfaugment.SelfAugmentPromoteResult](s, "self verify promote confirm", "self-verify", "promote", "--from-key", candidateCompareKey, "--baseline-key", promotedBaselineKey, "--allow-failed-source", "--confirm", "--json")
	if failed != nil {
		return *failed
	}
	if !promoteConfirm.OK || promoteConfirm.DryRun || !promoteConfirm.Promoted {
		return s.fail("self-verify promote confirm did not write baseline")
	}
	return selfverify.StepResult{OK: true}
}

// validatePromotedCompare는 승격된 기준선이 후보와 깨끗하게 비교되는지 검증한다.
func (s *stateRoundtripSelfVerifySession) validatePromotedCompare(candidateCompareKey, promotedBaselineKey string) selfverify.StepResult {
	comparePromoted, failed := runSelfVerifyJSON[selfaugment.SelfAugmentCompareResult](s, "self verify compare promoted", "self-verify", "compare", "--baseline-key", promotedBaselineKey, "--candidate-key", candidateCompareKey, "--json")
	if failed != nil {
		return *failed
	}
	if !comparePromoted.OK || comparePromoted.Regressed || comparePromoted.ElapsedDeltaMS != 0 {
		return s.fail("promoted baseline did not compare cleanly with candidate")
	}
	return selfverify.StepResult{OK: true}
}
