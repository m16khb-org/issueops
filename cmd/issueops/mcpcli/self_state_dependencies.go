package mcpcli

import (
	"context"

	contract "issueops/internal/contract/selfaugment"
)

// SelfStateDependencies binds persistence use cases to this server's state root.
type SelfStateDependencies struct {
	SavePlan    func(context.Context, *contract.SelfAugmentPlanResult, string) error
	SaveSummary func(context.Context, *contract.SelfAugmentResult, string) error
	Promote     func(context.Context, string, string, bool, bool) (contract.SelfAugmentPromoteResult, error)
}
