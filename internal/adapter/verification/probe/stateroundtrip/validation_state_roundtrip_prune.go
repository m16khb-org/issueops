package stateroundtrip

import (
	selfverify "issueops/internal/contract/selfverify"
	"slices"

	"encoding/json"

	statecontract "issueops/internal/contract/state"
)

// runStateJSON runs a CLI step and decodes its JSON stdout; on failure it returns the failing step result and false.
func runStateJSON[T any](s *stateRoundtripStateSession, label string, command ...string) (T, selfverify.StepResult, bool) {
	var result T
	step := s.run(label, command...)
	if !step.OK {
		return result, s.combineFailed(step), false
	}
	if err := json.Unmarshal([]byte(step.Stdout), &result); err != nil {
		return result, s.fail(err.Error()), false
	}
	return result, selfverify.StepResult{}, true
}

func (s *stateRoundtripStateSession) validatePrune() selfverify.StepResult {
	oldKey := s.input.key + "-old"
	oldWriteResult, failed, ok := runStateJSON[statecontract.StateResult](s, "state old write", s.input.binary, "state", "write", "--key", oldKey, "--value", "old state", "--json")
	if !ok {
		return failed
	}
	oldWriteResult.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	if _, err := s.input.deps.writeRecord(s.input.tempState, oldKey, oldWriteResult.Record); err != nil {
		return s.fail(err.Error())
	}

	pruneDryResult, failed, ok := runStateJSON[statecontract.StatePruneResult](s, "state prune dry-run", s.input.binary, "state", "prune", "--max-age", "1h", "--json")
	if !ok {
		return failed
	}
	if !pruneDryRunClassifiedKeys(pruneDryResult, oldKey, s.input.key) {
		return s.fail("state prune dry-run did not classify old/fresh keys")
	}

	pruneConfirmResult, failed, ok := runStateJSON[statecontract.StatePruneResult](s, "state prune confirm", s.input.binary, "state", "prune", "--max-age", "1h", "--confirm", "--json")
	if !ok {
		return failed
	}
	if !pruneConfirmDeletedKey(pruneConfirmResult, oldKey) {
		return s.fail("state prune confirm did not delete old key")
	}

	listAfterPruneResult, failed, ok := runStateJSON[statecontract.StateListResult](s, "state list after prune", s.input.binary, "state", "list", "--json")
	if !ok {
		return failed
	}
	if !slices.Contains(listAfterPruneResult.Keys, s.input.key) || slices.Contains(listAfterPruneResult.Keys, oldKey) {
		return s.fail("state prune did not preserve fresh key and remove old key")
	}
	return selfverify.StepResult{OK: true}
}

// pruneDryRunClassifiedKeys reports whether a dry-run prune marked oldKey for deletion and kept freshKey.
func pruneDryRunClassifiedKeys(result statecontract.StatePruneResult, oldKey, freshKey string) bool {
	return result.OK && result.DryRun && slices.Contains(result.DeletedKeys, oldKey) && slices.Contains(result.KeptKeys, freshKey)
}

// pruneConfirmDeletedKey reports whether a confirmed prune actually deleted oldKey.
func pruneConfirmDeletedKey(result statecontract.StatePruneResult, oldKey string) bool {
	return result.OK && !result.DryRun && result.Confirm && slices.Contains(result.DeletedKeys, oldKey)
}
