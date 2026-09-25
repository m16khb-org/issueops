package issueops

import (
	"fmt"
	"strings"
	"time"

	"context"

	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
)

// RecordIssueOpsDomainReview persists the grill-phase domain review
// (terminology, model fit, risks, uncertainties) — the source of truth backing
// the grill domain_review artifact.
func RecordIssueOpsDomainReview(stateRoot, id string, req issueops.IssueOpsDomainReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDomainReview(stateRoot, id, req, nil)
}

func RecordIssueOpsDomainReviewWithActor(stateRoot, id string, req issueops.IssueOpsDomainReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDomainReview(stateRoot, id, req, &actor)
}

func recordIssueOpsDomainReview(stateRoot, id string, req issueops.IssueOpsDomainReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	store := reviewMutationStore(actor)
	store.ValidateMutation = func(record issueops.IssueOpsRecord) error {
		return validateWorkspacePreparationMutation(record, actor)
	}
	return reviewapp.RecordDomainReview(store, stateRoot, id, req)
}

// RecordIssueOpsAISlopCleanEvidence persists which cleanup categories were
// checked/cleaned and which verifications were rerun — the source of truth
// backing the ai-slop-clean cleanup_evidence and verification_evidence artifacts.
func RecordIssueOpsAISlopCleanEvidence(stateRoot, id string, categories, verification []string) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsAISlopCleanEvidence(stateRoot, id, categories, verification, nil)
}

func RecordIssueOpsAISlopCleanEvidenceWithActor(stateRoot, id string, categories, verification []string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsAISlopCleanEvidence(stateRoot, id, categories, verification, &actor)
}

func recordIssueOpsAISlopCleanEvidence(stateRoot, id string, categories, verification []string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if err := validatePostTransferMutation(record, actor); err != nil {
			return err
		}
		var e error
		rec, e = recordIssueOpsAISlopCleanEvidenceLocked(stateRoot, id, categories, verification)
		return e
	})
	return rec, err
}

func recordIssueOpsAISlopCleanEvidenceLocked(stateRoot, id string, categories, verification []string) (issueops.IssueOpsRecord, error) {
	cats := cleanIssueOpsTextValues(categories)
	ver := cleanIssueOpsTextValues(verification)
	if len(cats) == 0 {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("ai-slop-clean evidence requires at least one cleanup category")
	}
	if len(ver) == 0 {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("ai-slop-clean evidence requires at least one verification entry")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.AISlopCleanCategories = cats
	record.AISlopCleanVerification = ver
	if strings.TrimSpace(record.AISlopCleanAt) != "" && issueOpsPhaseRank(record.Phase) >= issueOpsPhaseRank(IssueOpsPhaseAISlopClean) {
		return refreshIssueOpsAISlopClean(stateRoot, record)
	}
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}

// ResolveIssueOpsFeedback records the outcome of a feedback item by index — the
// source of truth backing the feedback feedback_resolution artifact.
func ResolveIssueOpsFeedback(stateRoot, id string, index int, resolution string) (issueops.IssueOpsRecord, error) {
	return resolveIssueOpsFeedback(stateRoot, id, index, resolution, nil)
}

func ResolveIssueOpsFeedbackWithActor(stateRoot, id string, index int, resolution string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return resolveIssueOpsFeedback(stateRoot, id, index, resolution, &actor)
}

func resolveIssueOpsFeedback(stateRoot, id string, index int, resolution string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.ResolveFeedback(reviewMutationStore(actor), stateRoot, id, index, resolution)
}
