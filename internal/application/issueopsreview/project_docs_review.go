package issueopsreview

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordProjectDocsReview stores the project docs review of record id. Paths are
// observed before the lifecycle lock and committed under it, and the commit is
// rejected when the record, actor, phase or local change set moved in between.
// A no-change verdict needs reviewed docs unless the worktree has no project doc.
func RecordProjectDocsReview(store reviewport.ProjectDocsReviewStore, stateRoot, id string, req model.ProjectDocsReviewRequest) (model.Record, error) {
	verdict := strings.ToLower(strings.TrimSpace(req.Verdict))
	docs := cleanReviewValues(req.Docs)
	evidence := cleanReviewValues(req.Evidence)
	reviewedDocs := cleanReviewValues(req.ReviewedDocs)
	// The request alone decides everything but a no-change without reviewed
	// docs, which depends on whether the worktree has any project doc.
	if err := reviewdomain.ValidateProjectDocsReviewRecord(verdict, len(docs), len(evidence), len(reviewedDocs), false); err != nil {
		return model.Record{OK: false}, err
	}
	observed, err := store.Read(stateRoot, id)
	if err != nil {
		return model.Record{OK: false}, err
	}
	if verdict == "no-change" && len(reviewedDocs) == 0 && (store.ProjectDocsPresent == nil || store.ProjectDocsPresent(store.Root(observed))) {
		return model.Record{OK: false}, reviewdomain.ValidateProjectDocsReviewRecord(verdict, len(docs), len(evidence), 0, true)
	}
	plan := projectDocsReviewPlan{
		observed:    observed,
		verdict:     verdict,
		evidence:    evidence,
		fingerprint: store.Fingerprint(observed),
	}
	plan.normalized, plan.normalizeErr = normalizeUpdatedDocuments(store, observed, docs)
	plan.reviewed, plan.reviewedErr = normalizeReviewedDocuments(store, observed, reviewedDocs)
	if plan.reviewedErr != nil && verdict == "no-change" && store.ProjectDocsPresent != nil && !store.ProjectDocsPresent(store.Root(observed)) {
		// QA: with no project doc, agents tried README.md, missing paths,
		// then a cycle plan, before learning no-change needs no reviewed doc.
		plan.reviewedErr = fmt.Errorf("%w; this worktree has no project doc (AGENTS.md or .issueops/*.md outside issues/ and gates/), so record no-change without --reviewed-doc", plan.reviewedErr)
	}
	var result model.Record
	err = store.WithLock(stateRoot, id, func() error {
		var err error
		result, err = plan.commit(store, stateRoot, id)
		return err
	})
	if err != nil {
		return model.Record{OK: false}, err
	}
	return result, nil
}

// projectDocsReviewPlan carries everything observed before the lifecycle lock,
// including the deferred path errors, into the locked commit.
type projectDocsReviewPlan struct {
	observed     model.Record
	verdict      string
	normalized   []string
	normalizeErr error
	reviewed     []string
	reviewedErr  error
	evidence     []string
	fingerprint  string
}

func (p projectDocsReviewPlan) commit(store reviewport.ProjectDocsReviewStore, stateRoot, id string) (model.Record, error) {
	current, err := store.Read(stateRoot, id)
	if err != nil {
		return model.Record{}, err
	}
	if err := store.ValidateMutation(current); err != nil {
		return model.Record{}, err
	}
	if err := issueopsdomain.ValidateEvidenceRecordingPhase(current.Phase, "project docs review"); err != nil {
		return model.Record{}, err
	}
	if err := validateCurrentChangeObservation(p.observed, current); err != nil {
		return model.Record{}, err
	}
	if p.normalizeErr != nil {
		return model.Record{}, p.normalizeErr
	}
	if p.reviewedErr != nil {
		return model.Record{}, p.reviewedErr
	}
	now := store.Now()
	current.ProjectDocsReview = &model.ProjectDocsReview{
		Verdict: p.verdict, Docs: p.normalized, ReviewedDocs: p.reviewed, Evidence: p.evidence,
		ReviewedFingerprint: p.fingerprint, RecordedAt: now,
	}
	current.UpdatedAt = now
	return store.Write(stateRoot, current)
}

// Path observations happen before the lifecycle lock; their errors are returned
// only after the current actor, phase, and observed record identity are checked.
func normalizeUpdatedDocuments(store reviewport.ProjectDocsReviewStore, record model.Record, docs []string) ([]string, error) {
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

func normalizeReviewedDocuments(store reviewport.ProjectDocsReviewStore, record model.Record, docs []string) ([]string, error) {
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
