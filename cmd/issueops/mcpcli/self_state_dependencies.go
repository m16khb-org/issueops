package mcpcli

import (
	"context"

	contract "issueops/internal/contract/selfaugment"
)

// SelfStateDependencies binds persistence use cases to this server's state root.
type SelfStateDependencies struct {
	SavePlan    func(context.Context, *contract.PlanResult, string) error
	SaveSummary func(context.Context, *contract.Result, string) error
	Promote     func(context.Context, string, string, bool, bool) (contract.PromoteResult, error)
}
