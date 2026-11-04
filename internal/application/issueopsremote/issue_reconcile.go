package issueopsremote

import (
	"context"
	"errors"
	"fmt"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsremote"
	"issueops/internal/port"
)

type IssueRecordReader interface {
	Read(context.Context, string) (model.Record, error)
}

type IssueCandidateSource interface {
	Find(context.Context, string, port.IssueProviderFindIssueCreateCandidatesRequest) (port.IssueProviderFindIssueCreateCandidatesResult, error)
}

type IssueLiveVerifier func(context.Context, model.RemoteArtifactVerificationRequest) error

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
type IssueCreateSettlement func(context.Context, model.Record) (bool, error)

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

func (s *IssueReconciler) Reconcile(ctx context.Context, id string, confirm bool) (model.IssueCreateReconcileResult, error) {
	var result model.IssueCreateReconcileResult
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
	crashSettleable := s.crashSettleable(intent, search)
	var settleErr error
	if crashSettleable {
		// QA (killed create-issue): the intent stayed pending, reconcile
		// needed exactly one candidate, and create-issue refused a pending
		// intent: no exit but abandon.
		var settled bool
		if settled, settleErr = s.settled(ctx, record); settled && settleErr == nil {
			return s.releaseCrashedCreate(ctx, record, confirm)
		}
	}
	if err := domain.ValidateIssueReconcileSearch(search.Truncated, len(search.Candidates)); err != nil {
		if crashSettleable {
			return result, unsettledCrashError(err, intent, settleErr)
		}
		return result, err
	}
	candidate := search.Candidates[0]
	record, err = s.adoptCandidate(ctx, record, candidate, confirm)
	if err != nil {
		return result, err
	}
	result = model.IssueCreateReconcileResult{OK: true, CandidateCount: 1, CandidateURL: candidate.URL, WouldAdopt: !confirm, IssueURL: record.IssueURL, IssueCreateIntent: record.IssueCreateIntent}
	if !confirm {
		// QA: the preview showed would_adopt with no command, and the skill
		// did not name --confirm.
		result.NextCommand = "issueops remote reconcile-issue --id " + record.ID + " --confirm --json"
	}
	return result, nil
}

// crashSettleable reports whether a pending create found no candidate and a
// settlement is wired, so its caller may have died inside the provider call.
func (s *IssueReconciler) crashSettleable(intent *model.IssueCreateIntent, search port.IssueProviderFindIssueCreateCandidatesResult) bool {
	return len(search.Candidates) == 0 && !search.Truncated && intent.Status == model.IssueCreateIntentPending && s.settled != nil
}

// unsettledCrashError appends the settlement hint to the no-candidate search error.
func unsettledCrashError(err error, intent *model.IssueCreateIntent, settleErr error) error {
	// QA: agents read the conditions as requirements they could not meet.
	hint := "not settled yet, wait and re-run this reconcile: a pending create whose caller died is released once it is 2 minutes old (started " + intent.UpdatedAt + "), no issue create for it is running, and the provider lists no issue with its marker"
	if settleErr != nil {
		hint += " (settlement: " + settleErr.Error() + ")"
	}
	return fmt.Errorf("%w: %s", err, hint)
}

// adoptCandidate validates the single candidate against the intent and, on
// confirm, verifies it live and completes the intent. A preview returns the
// record unchanged.
func (s *IssueReconciler) adoptCandidate(ctx context.Context, record model.Record, candidate port.IssueProviderIssueCreateCandidate, confirm bool) (model.Record, error) {
	intent := record.IssueCreateIntent
	if err := domain.ValidateIssueReconcileCandidate(*intent, issueopsremote.ProjectKey(candidate.URL, intent.Provider, "issue"), candidate.Title, candidate.Body); err != nil {
		if confirm {
			err = s.recordFailure(ctx, record.ID, candidate.URL, err)
		}
		return record, err
	}
	if !confirm {
		return record, nil
	}
	if err := s.verify(ctx, model.RemoteArtifactVerificationRequest{Provider: intent.Provider, Kind: "issue", URL: candidate.URL, Labels: intent.Labels, Assignees: intent.Assignees}); err != nil {
		return record, s.recordFailure(ctx, record.ID, candidate.URL, err)
	}
	return s.intents.Complete(ctx, record.ID, candidate.URL, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *IssueReconciler) releaseCrashedCreate(ctx context.Context, record model.Record, confirm bool) (model.IssueCreateReconcileResult, error) {
	result := model.IssueCreateReconcileResult{OK: true, IssueURL: record.IssueURL, IssueCreateIntent: record.IssueCreateIntent}
	if !confirm {
		// QA: ok + candidate_count 0 + would_adopt false read as "nothing to do".
		result.WouldRelease = true
		result.NextCommand = "issueops remote reconcile-issue --id " + record.ID + " --confirm --json"
		return result, nil
	}
	updated, err := s.intents.ReleaseCrashed(ctx, record.ID, *record.IssueCreateIntent, model.IssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked, Failure: issueCreateCrashReleased, ObservedAt: s.now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return result, err
	}
	result.IssueCreateIntent = updated.IssueCreateIntent
	// not_invoked lets the same create-issue run again; next renders it.
	result.NextCommand = "issueops next --id " + record.ID + " --json"
	return result, nil
}

func (s *IssueReconciler) recordFailure(ctx context.Context, id, url string, cause error) error {
	_, err := s.intents.Outcome(ctx, id, model.IssueCreateOutcome{Status: model.IssueCreateIntentVerificationFailed, CanonicalURL: url, Failure: issueopsremote.IssueCreateFailure(cause), ObservedAt: s.now().UTC().Format(time.RFC3339Nano)})
	return errors.Join(cause, err)
}
