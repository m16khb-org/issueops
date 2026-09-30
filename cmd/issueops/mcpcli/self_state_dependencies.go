package mcpcli

import contract "issueops/internal/contract/selfaugment"

// SelfStateDependencies binds persistence use cases to this server's state root.
type SelfStateDependencies struct {
	SavePlan    func(*contract.SelfAugmentPlanResult, string) error
	SaveSummary func(*contract.SelfAugmentResult, string) error
	Promote     func(string, string, bool, bool) (contract.SelfAugmentPromoteResult, error)
}
