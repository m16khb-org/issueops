package selfaugment

import (
	"fmt"

	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/selfaugment"
)

type PromoteDeps struct {
	StateDir      func() string
	ReadSnapshot  func(string) (contract.SelfAugmentStateSnapshot, error)
	WriteSnapshot func(string, string, contract.SelfAugmentStateSnapshot) error
	ReadState     func(string) (state.StateResult, error)
}

func PromoteBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool, deps PromoteDeps) (contract.SelfAugmentPromoteResult, error) {
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
