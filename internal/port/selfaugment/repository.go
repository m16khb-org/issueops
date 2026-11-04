// Package selfaugment defines the outbound port the self-augmentation planner
// uses to read repository evidence.
package selfaugment

import contract "issueops/internal/contract/selfaugment"

// PlanRepository is the outbound port that supplies repository evidence for
// self-augmentation planning: the GENIUS_THINK.md text, whether the work tree
// holds an implementation delta, and the collected repository signals.
type PlanRepository interface {
	ReadGeniusThink(root string) (path, text string, err error)
	HasImplementationDelta(root string) bool
	CollectSignals(root string, docsIndexed int, skills []string, geniusText string) contract.RepoSignals
}
