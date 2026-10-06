package augmentcatalog

import (
	contract "issueops/internal/contract/selfaugment"
)

// Repository observations are supplied by the composition root.
var (
	CollectSelfAugmentRepoSignals func(string, int, []string, string) contract.SelfAugmentRepoSignals
	DocsContainTerm               func(string, string) bool
	FileContainsTerm              func(string, string, string) bool
	DirContainsTerm               func(string, string, string) bool
)
