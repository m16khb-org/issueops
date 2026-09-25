package issueopsreview

import (
	"strings"

	model "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

func RecordProjectDocsReview(store reviewport.ProjectDocsReviewStore, stateRoot, id string, req model.IssueOpsProjectDocsReviewRequest) (model.IssueOpsRecord, error) {
	verdict := strings.ToLower(strings.TrimSpace(req.Verdict))
	docs := cleanReviewValues(req.Docs)
	evidence := cleanReviewValues(req.Evidence)
	reviewedDocs := cleanReviewValues(req.ReviewedDocs)
	if err := reviewdomain.ValidateProjectDocsReviewRecord(verdict, len(docs), len(evidence), len(reviewedDocs)); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	observed, err := store.Read(stateRoot, id)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	fingerprint := store.Fingerprint(observed)
	normalized, normalizeErr := store.NormalizeDocs(observed, docs)
	reviewed, reviewedErr := store.NormalizeReviewedDocs(observed, reviewedDocs)
	var result model.IssueOpsRecord
	err = store.WithLock(stateRoot, id, func() error {
		current, err := store.Read(stateRoot, id)
		if err != nil {
			return err
		}
		if err := store.ValidateMutation(current); err != nil {
			return err
		}
		if err := issueopsdomain.ValidateEvidenceRecordingPhase(current.Phase, "project docs review"); err != nil {
			return err
		}
		if err := validateCurrentChangeObservation(observed, current); err != nil {
			return err
		}
		if normalizeErr != nil {
			return normalizeErr
		}
		if reviewedErr != nil {
			return reviewedErr
		}
		now := store.Now()
		current.ProjectDocsReview = &model.IssueOpsProjectDocsReview{
			Verdict: verdict, Docs: normalized, ReviewedDocs: reviewed, Evidence: evidence,
			ReviewedFingerprint: fingerprint, RecordedAt: now,
		}
		current.UpdatedAt = now
		result, err = store.Write(stateRoot, current)
		return err
	})
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	return result, nil
}
