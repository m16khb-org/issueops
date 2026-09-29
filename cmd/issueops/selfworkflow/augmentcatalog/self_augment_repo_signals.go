package augmentcatalog

// Repository observations are supplied by the composition root.
var (
	CollectSelfAugmentRepoSignals func(string, int, []string, string) SelfAugmentRepoSignals
	DocsContainTerm               func(string, string) bool
	FileContainsTerm              func(string, string, string) bool
	DirContainsTerm               func(string, string, string) bool
)
