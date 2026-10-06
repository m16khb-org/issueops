package selfworkflow

import (
	augmentation "issueops/internal/adapter/augmentation"
	docs "issueops/internal/adapter/docs"
	docsapp "issueops/internal/application/docs"
	augmentcontract "issueops/internal/contract/selfaugment"
	"time"
)

func collectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) augmentcontract.SelfAugmentRepoSignals {
	return (augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}).CollectSignals(root, docsIndexed, skills, geniusText)
}

func docsContainTerm(root, term string) bool {
	return (augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}).DocsContainTerm(root, term)
}
