package issueopsremote

import (
	"context"
	"errors"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
	"issueops/internal/port"
)

type IssueRecordReader interface {
	Read(context.Context, string) (model.IssueOpsRecord, error)
}

type IssueCandidateSource interface {
	Find(context.Context, string, port.IssueProviderFindIssueCreateCandidatesRequest) (port.IssueProviderFindIssueCreateCandidatesResult, error)
}

type IssueLiveVerifier func(context.Context, model.IssueOpsRemoteArtifactVerificationRequest) error

type IssueReconciler struct {
	records    IssueRecordReader
	candidates IssueCandidateSource
	intents    *IssueCreateIntents
	verify     IssueLiveVerifier
	now        func() time.Time
}

func NewIssueReconciler(records IssueRecordReader, candidates IssueCandidateSource, intents *IssueCreateIntents, verify IssueLiveVerifier, now func() time.Time) *IssueReconciler {
	return &IssueReconciler{records: records, candidates: candidates, intents: intents, verify: verify, now: now}
}

func (s *IssueReconciler) Reconcile(ctx context.Context, id string, confirm bool) (model.IssueOpsIssueCreateReconcileResult, error) {
	var result model.IssueOpsIssueCreateReconcileResult
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return result, err
	}
	if err := domain.ValidateIssueReconcileIntent(record.IssueCreateIntent); err != nil {
		return result, err
	}
	intent := record.IssueCreateIntent
	search, err := s.candidates.Find(ctx, intent.Provider, port.IssueProviderFindIssueCreateCandidatesRequest{Repo: record.Repo, ProjectAuthority: intent.ProjectAuthority, Marker: intent.Marker})
	if err != nil {
		return result, err
	}
	if err := domain.ValidateIssueReconcileSearch(search.Truncated, len(search.Candidates)); err != nil {
		return result, err
	}
	candidate := search.Candidates[0]
	if err := domain.ValidateIssueReconcileCandidate(*intent, remote.ProjectKey(candidate.URL, intent.Provider, "issue"), candidate.Title, candidate.Body); err != nil {
		if confirm {
			err = s.recordFailure(ctx, record.ID, candidate.URL, err)
		}
		return result, err
	}
	if confirm {
		if err := s.verify(ctx, model.IssueOpsRemoteArtifactVerificationRequest{Provider: intent.Provider, Kind: "issue", URL: candidate.URL, Labels: intent.Labels, Assignees: intent.Assignees}); err != nil {
			return result, s.recordFailure(ctx, record.ID, candidate.URL, err)
		}
		record, err = s.intents.Complete(ctx, record.ID, candidate.URL, s.now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return result, err
		}
	}
	return model.IssueOpsIssueCreateReconcileResult{OK: true, CandidateCount: 1, CandidateURL: candidate.URL, WouldAdopt: !confirm, IssueURL: record.IssueURL, IssueCreateIntent: record.IssueCreateIntent}, nil
}

func (s *IssueReconciler) recordFailure(ctx context.Context, id, url string, cause error) error {
	_, err := s.intents.Outcome(ctx, id, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentVerificationFailed, CanonicalURL: url, Failure: IssueCreateFailure(cause), ObservedAt: s.now().UTC().Format(time.RFC3339Nano)})
	return errors.Join(cause, err)
}
