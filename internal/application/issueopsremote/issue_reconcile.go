package issueopsremote

import (
	"context"
	"errors"
	"fmt"
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
	settled    IssueCreateSettlement
}

// IssueCreateSettlement proves that a pending issue create whose caller died
// created nothing and can no longer create anything.
type IssueCreateSettlement func(context.Context, model.IssueOpsRecord) (bool, error)

// WithCrashSettlement lets a reconcile that finds no candidate for a pending
// create (its caller died inside the provider call) release the intent for a
// retry once the settlement proves nothing was or will be created.
func (s *IssueReconciler) WithCrashSettlement(settled IssueCreateSettlement) *IssueReconciler {
	s.settled = settled
	return s
}

// issueCreateCrashReleased is the failure a settled crash records: the
// not_invoked status lets the same create-issue run again.
const issueCreateCrashReleased = "the create-issue caller died before the provider created the issue; settled: no candidate, no create running, quiet period passed"

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
	var settleErr error
	if len(search.Candidates) == 0 && !search.Truncated && intent.Status == model.IssueCreateIntentPending && s.settled != nil {
		// QA (killed create-issue): the intent stayed pending, reconcile
		// needed exactly one candidate, and create-issue refused a pending
		// intent: no exit but abandon.
		var settled bool
		if settled, settleErr = s.settled(ctx, record); settled && settleErr == nil {
			return s.releaseCrashedCreate(ctx, record, confirm)
		}
	}
	if err := domain.ValidateIssueReconcileSearch(search.Truncated, len(search.Candidates)); err != nil {
		if len(search.Candidates) == 0 && !search.Truncated && intent.Status == model.IssueCreateIntentPending && s.settled != nil {
			hint := "a pending create whose caller died is released once it is 2 minutes old, no issue create for it is running, and the provider lists no issue with its marker; re-run this reconcile then"
			if settleErr != nil {
				hint += " (settlement: " + settleErr.Error() + ")"
			}
			return result, fmt.Errorf("%w: %s", err, hint)
		}
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
	result = model.IssueOpsIssueCreateReconcileResult{OK: true, CandidateCount: 1, CandidateURL: candidate.URL, WouldAdopt: !confirm, IssueURL: record.IssueURL, IssueCreateIntent: record.IssueCreateIntent}
	if !confirm {
		// QA: the preview showed would_adopt with no command, and the skill
		// did not name --confirm.
		result.NextCommand = "issueops remote reconcile-issue --id " + record.ID + " --confirm --json"
	}
	return result, nil
}

func (s *IssueReconciler) releaseCrashedCreate(ctx context.Context, record model.IssueOpsRecord, confirm bool) (model.IssueOpsIssueCreateReconcileResult, error) {
	result := model.IssueOpsIssueCreateReconcileResult{OK: true, IssueURL: record.IssueURL, IssueCreateIntent: record.IssueCreateIntent}
	if !confirm {
		result.NextCommand = "issueops remote reconcile-issue --id " + record.ID + " --confirm --json"
		return result, nil
	}
	updated, err := s.intents.ReleaseCrashed(ctx, record.ID, *record.IssueCreateIntent, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked, Failure: issueCreateCrashReleased, ObservedAt: s.now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return result, err
	}
	result.IssueCreateIntent = updated.IssueCreateIntent
	// not_invoked lets the same create-issue run again; next renders it.
	result.NextCommand = "issueops next --id " + record.ID + " --json"
	return result, nil
}

func (s *IssueReconciler) recordFailure(ctx context.Context, id, url string, cause error) error {
	_, err := s.intents.Outcome(ctx, id, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentVerificationFailed, CanonicalURL: url, Failure: remote.IssueCreateFailure(cause), ObservedAt: s.now().UTC().Format(time.RFC3339Nano)})
	return errors.Join(cause, err)
}
