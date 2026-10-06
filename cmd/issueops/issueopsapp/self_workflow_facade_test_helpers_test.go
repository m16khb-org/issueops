package issueopsapp

import (
	"context"
	docsapp "issueops/internal/application/docs"
	"time"

	augmentation "issueops/internal/adapter/augmentation"
	docs "issueops/internal/adapter/docs"
	statestore "issueops/internal/adapter/outbound/state"
	verifyapp "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func applySelfAugmentHistoryRetention(result *augmentcontract.SelfAugmentHistoryResult, options augmentcontract.SelfAugmentHistoryRetentionOptions) error {
	return newSelfWorkflowHistory(statestore.StateDir()).ApplyRetention(context.Background(), result, options)
}

func nonNilStringSlice(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func nonNilSlowStepSlice(items []augmentcontract.SelfAugmentSlowStep) []augmentcontract.SelfAugmentSlowStep {
	if items == nil {
		return []augmentcontract.SelfAugmentSlowStep{}
	}
	return items
}

func collectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) augmentcontract.SelfAugmentRepoSignals {
	return (augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}).CollectSignals(root, docsIndexed, skills, geniusText)
}

func docsContainTerm(root, term string) bool {
	return (augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}).DocsContainTerm(root, term)
}

func summarizeSelfAugment(result augmentcontract.SelfAugmentResult) augmentcontract.SelfAugmentSummary {
	return verifyapp.SummarizeSelfVerification(result, 95)
}
