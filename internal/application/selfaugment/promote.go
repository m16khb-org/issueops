package selfaugment

import (
	"fmt"

	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/selfaugment"
)

// PromoteDeps supplies the state directory and snapshot/state accessors
// PromoteBaseline needs.
type PromoteDeps struct {
	StateDir      func() string
	ReadSnapshot  func(string) (contract.StateSnapshot, error)
	WriteSnapshot func(string, string, contract.StateSnapshot) error
	ReadState     func(string) (state.Result, error)
}

// PromoteBaseline copies the summary snapshot at fromKey to baselineKey. Without
// confirm it only previews the promotion; with confirm it writes the baseline and
// reports its path and size. A failed source is refused unless allowFailedSource is set.
func PromoteBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool, deps PromoteDeps) (contract.PromoteResult, error) {
	result, err := domain.PreparePromotion(fromKey, baselineKey, confirm, deps.StateDir())
	if err != nil {
		return result, err
	}

	snapshot, err := deps.ReadSnapshot(fromKey)
	if err != nil {
		return result, fmt.Errorf("read source summary: %w", err)
	}
	result, err = domain.ObservePromotionSource(result, snapshot, allowFailedSource)
	if err != nil || !confirm {
		return result, err
	}

	if err := deps.WriteSnapshot(deps.StateDir(), baselineKey, snapshot); err != nil {
		return result, err
	}
	state, err := deps.ReadState(baselineKey)
	if err != nil {
		return result, err
	}
	result.OK = true
	result.Promoted = true
	result.Path = state.Path
	result.Bytes = state.Record.Bytes
	return result, nil
}
