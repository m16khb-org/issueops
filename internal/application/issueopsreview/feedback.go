package issueopsreview

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

func AddFeedback(store reviewport.ReviewMutationStore, stateRoot, id, source, body, classification string) (model.IssueOpsRecord, error) {
	return mutateReviewRecord(store, stateRoot, id, func() error {
		source = strings.TrimSpace(source)
		body = strings.TrimSpace(body)
		classification = strings.ToLower(strings.TrimSpace(classification))
		if source == "" {
			return fmt.Errorf("feedback source is required")
		}
		if body == "" {
			return fmt.Errorf("feedback body is required")
		}
		if !model.KnownFeedbackClassification(classification) {
			return fmt.Errorf("unknown issueops feedback classification %q; use contract_change, defect, question, noise, valid_review, stale_review, rollout_evidence_missing, or environment_debt", classification)
		}
		return nil
	}, func(record *model.IssueOpsRecord) error {
		nextPhase, err := reviewdomain.FeedbackPhaseAfterAdd(string(record.Phase), strings.TrimSpace(record.AISlopCleanAt) != "")
		if err != nil {
			return err
		}
		now := store.Now()
		record.Feedback = append(record.Feedback, model.IssueOpsFeedbackItem{Source: source, Body: body, Classification: classification, CreatedAt: now})
		record.Phase = model.IssueOpsPhase(nextPhase)
		record.UpdatedAt = now
		return nil
	})
}

func MarkContractFeedbackIssueUpdated(store reviewport.ReviewMutationStore, stateRoot, id string) (model.IssueOpsRecord, error) {
	return mutateReviewRecord(store, stateRoot, id, nil, func(record *model.IssueOpsRecord) error {
		now := store.Now()
		marked := false
		for i := range record.Feedback {
			if reviewdomain.FeedbackRequiresIssueUpdate(record.Feedback[i].Classification, record.Feedback[i].IssueUpdatedAt) {
				record.Feedback[i].IssueUpdatedAt = now
				marked = true
			}
		}
		if !marked {
			return fmt.Errorf("no unresolved contract_change feedback requires a remote issue update")
		}
		record.UpdatedAt = now
		return nil
	})
}

func ResolveFeedback(store reviewport.ReviewMutationStore, stateRoot, id string, index int, resolution string) (model.IssueOpsRecord, error) {
	return mutateReviewRecord(store, stateRoot, id, func() error {
		resolution = strings.ToLower(strings.TrimSpace(resolution))
		if !model.KnownFeedbackResolution(resolution) || resolution == "" {
			return fmt.Errorf("unknown feedback resolution %q; use valid-defect, question-answered, or noise-dismissed", resolution)
		}
		return nil
	}, func(record *model.IssueOpsRecord) error {
		if err := reviewdomain.ValidateFeedbackIndex(index, len(record.Feedback)); err != nil {
			return err
		}
		now := store.Now()
		record.Feedback[index].Resolution = resolution
		record.UpdatedAt = now
		return nil
	})
}
