package stateroundtrip

import (
	selfaugment "issueops/internal/contract/selfaugment"
	selfverify "issueops/internal/contract/selfverify"
	"slices"

	statecontract "issueops/internal/contract/state"
)

func (s *stateRoundtripSelfVerifySession) validateHistoryAndRetention(baselineCompareKey, candidateCompareKey, promotedBaselineKey string) selfverify.StepResult {
	if step := s.validateHistoryListing(baselineCompareKey, candidateCompareKey, promotedBaselineKey); !step.OK {
		return step
	}
	if step := s.validateRetentionPrune(); !step.OK {
		return step
	}
	return s.validateStateDoctorCorruption()
}

// validateHistoryListing은 history가 저장된 기준선/후보/승격 요약을 모두 나열하는지 검증한다.
func (s *stateRoundtripSelfVerifySession) validateHistoryListing(baselineCompareKey, candidateCompareKey, promotedBaselineKey string) selfverify.StepResult {
	historyResult, failed := runSelfVerifyJSON[selfaugment.SelfAugmentHistoryResult](s, "self verify history", "self-verify", "history", "--prefix", s.input.key+"-", "--json")
	if failed != nil {
		return *failed
	}
	historyKeys := []string{}
	for _, entry := range historyResult.Entries {
		historyKeys = append(historyKeys, entry.Key)
	}
	if !historyResult.OK || historyResult.TotalMatches < 3 || !historyListsAll(historyKeys, baselineCompareKey, candidateCompareKey, promotedBaselineKey) {
		return s.fail("self-verify history did not list saved baseline/candidate/promoted summaries")
	}
	return selfverify.StepResult{OK: true}
}

// validateRetentionPrune은 retention dry-run, confirm 삭제, 삭제 후 잔여 개수를 순서대로 검증한다.
func (s *stateRoundtripSelfVerifySession) validateRetentionPrune() selfverify.StepResult {
	prefix := s.input.key + "-"
	retentionDry, failed := runSelfVerifyJSON[selfaugment.SelfAugmentHistoryResult](s, "self verify history retention dry-run", "self-verify", "history", "--prefix", prefix, "--retention-limit", "1", "--prune-retention", "--json")
	if failed != nil {
		return *failed
	}
	if !retentionDryRunIsSafe(retentionDry) {
		return s.fail("self-verify history retention dry-run did not classify prune candidates safely")
	}

	retentionConfirm, failed := runSelfVerifyJSON[selfaugment.SelfAugmentHistoryResult](s, "self verify history retention confirm", "self-verify", "history", "--prefix", prefix, "--retention-limit", "1", "--prune-retention", "--confirm", "--json")
	if failed != nil {
		return *failed
	}
	if !retentionConfirmDeleted(retentionConfirm) {
		return s.fail("self-verify history retention confirm did not delete prune candidates")
	}

	historyAfterRetention, failed := runSelfVerifyJSON[selfaugment.SelfAugmentHistoryResult](s, "self verify history after retention", "self-verify", "history", "--prefix", prefix, "--json")
	if failed != nil {
		return *failed
	}
	if historyAfterRetention.TotalMatches > 1 {
		return s.fail("self-verify history retention confirm left too many matching summaries")
	}
	return selfverify.StepResult{OK: true}
}

// validateStateDoctorCorruption은 손상된 상태 파일을 심고 doctor가 이를 보고하면서 유효 키는 보존하는지 검증한다.
func (s *stateRoundtripSelfVerifySession) validateStateDoctorCorruption() selfverify.StepResult {
	input := s.input
	key := input.key
	db, err := input.deps.openDatabase(input.tempState)
	if err != nil {
		return s.fail(err.Error())
	}
	if err := db.Put("state", key+"-corrupt", []byte("{not json\n")); err != nil {
		return s.fail(err.Error())
	}
	doctorResult, failed := runSelfVerifyJSON[statecontract.StateDoctorResult](s, "state doctor", "state", "doctor", "--json")
	if failed != nil {
		return *failed
	}
	if !doctorResult.OK || doctorResult.Healthy || !slices.Contains(doctorResult.ValidKeys, key) || !stateDoctorHasIssueCode(doctorResult.Issues, "invalid_state") {
		return s.fail("state doctor did not report corrupt fixture and preserve valid key")
	}
	return selfverify.StepResult{OK: true}
}

func historyListsAll(historyKeys []string, wantKeys ...string) bool {
	for _, want := range wantKeys {
		if !slices.Contains(historyKeys, want) {
			return false
		}
	}
	return true
}

func retentionDryRunIsSafe(result selfaugment.SelfAugmentHistoryResult) bool {
	retention := result.Retention
	return retention != nil && retention.DryRun && !retention.Confirm && retention.Limit == 1 && len(retention.CandidateKeys) != 0 && len(retention.DeletedKeys) == 0
}

func retentionConfirmDeleted(result selfaugment.SelfAugmentHistoryResult) bool {
	retention := result.Retention
	return retention != nil && !retention.DryRun && retention.Confirm && len(retention.DeletedKeys) != 0
}

func stateDoctorHasIssueCode(issues []statecontract.StateDoctorIssue, want string) bool {
	for _, issue := range issues {
		if issue.Code == want {
			return true
		}
	}
	return false
}
