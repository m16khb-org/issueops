package selfaugment

import contract "issueops/internal/contract/selfaugment"

type PlanRepository interface {
	ReadGeniusThink(root string) (path, text string, err error)
	HasImplementationDelta(root string) bool
	CollectSignals(root string, docsIndexed int, skills []string, geniusText string) contract.SelfAugmentRepoSignals
}
