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
	normalized, normalizeErr := normalizeUpdatedDocuments(store, observed, docs)
	reviewed, reviewedErr := normalizeReviewedDocuments(store, observed, reviewedDocs)
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

// Path observations happen before the lifecycle lock; their errors are returned
// only after the current actor, phase, and observed record identity are checked.
func normalizeUpdatedDocuments(store reviewport.ProjectDocsReviewStore, record model.IssueOpsRecord, docs []string) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	root := store.Root(record)
	changed := map[string]bool{}
	for _, path := range store.ChangedPaths(record) {
		changed[path] = true
	}
	out := make([]string, 0, len(docs))
	for _, doc := range docs {
		rel := store.RelativePath(root, doc)
		if err := reviewdomain.ValidateUpdatedDocumentPath(doc, rel, changed[rel]); err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	return out, nil
}

func normalizeReviewedDocuments(store reviewport.ProjectDocsReviewStore, record model.IssueOpsRecord, docs []string) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	root := store.Root(record)
	out := make([]string, 0, len(docs))
	for _, doc := range docs {
		rel := store.RelativePath(root, doc)
		if err := reviewdomain.ValidateReviewedDocumentPath(doc, rel, root != ""); err != nil {
			return nil, err
		}
		if err := reviewdomain.ValidateReviewedDocumentExists(rel, store.FileExists(root, rel)); err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	return out, nil
}
