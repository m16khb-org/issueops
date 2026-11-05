package issueops

import (
	"fmt"
	"slices"
	"strings"

	model "issueops/internal/contract/issueops"
)

// BeginIssueCreateIntent seals the intent to create an issue before the
// provider call. It fails when the record already links an issue. A first
// call stores a pending intent with a deterministic marker; a retry is
// allowed only for a not-invoked intent with an identical request and
// increments the attempt. Any other intent status requires reconcile.
func BeginIssueCreateIntent(record model.Record, request model.IssueCreateIntentRequest) (model.Record, error) {
	if strings.TrimSpace(record.IssueURL) != "" {
		return record, fmt.Errorf("issueops record already links issue %s", record.IssueURL)
	}
	if err := validateIssueCreateIntentRequest(request); err != nil {
		return record, err
	}
	if record.IssueCreateIntent != nil {
		intent := *record.IssueCreateIntent
		if intent.Status != model.IssueCreateIntentNotInvoked {
			return record, fmt.Errorf("issue create intent %s is %s; reconcile before retry", intent.OperationID, intent.Status)
		}
		if !sameIssueCreateRequest(intent, request) {
			return record, fmt.Errorf("retry request does not match sealed issue create intent")
		}
		intent.Status = model.IssueCreateIntentPending
		intent.Attempt++
		intent.Failure = ""
		intent.UpdatedAt = request.StartedAt
		record.IssueCreateIntent = &intent
		return record, nil
	}
	record.IssueCreateIntent = &model.IssueCreateIntent{
		OperationID: strings.TrimSpace(request.OperationID), Marker: "<!-- issueops:issue-create:" + strings.TrimSpace(request.OperationID) + " -->",
		Provider: strings.TrimSpace(request.Provider), ProjectAuthority: strings.TrimSpace(request.ProjectAuthority),
		Title: strings.TrimSpace(request.Title), BodySHA256: strings.ToLower(strings.TrimSpace(request.BodySHA256)),
		Labels: append([]string(nil), request.Labels...), Assignees: append([]string(nil), request.Assignees...),
		Status: model.IssueCreateIntentPending, Attempt: 1,
		StartedAt: strings.TrimSpace(request.StartedAt), UpdatedAt: strings.TrimSpace(request.StartedAt),
	}
	return record, nil
}

// RecordIssueCreateOutcome records the observed outcome of an issue create
// call on the sealed intent. The transition must be legal, and an observed
// canonical URL must belong to the sealed project authority. A completed
// intent cannot be changed.
func RecordIssueCreateOutcome(record model.Record, outcome model.IssueCreateOutcome, projectAuthority string) (model.Record, error) {
	if record.IssueCreateIntent == nil {
		return record, fmt.Errorf("no pending issue create intent")
	}
	if record.IssueCreateIntent.Status == model.IssueCreateIntentCompleted {
		return record, fmt.Errorf("issue create intent is already completed")
	}
	if err := ValidateIssueCreateTransition(record.IssueCreateIntent.Status, outcome.Status); err != nil {
		return record, err
	}
	next := *record.IssueCreateIntent
	next.Status = outcome.Status
	next.CanonicalURL = strings.TrimSpace(outcome.CanonicalURL)
	next.Failure = strings.TrimSpace(outcome.Failure)
	next.UpdatedAt = strings.TrimSpace(outcome.ObservedAt)
	if next.CanonicalURL != "" && (projectAuthority == "" || projectAuthority != next.ProjectAuthority) {
		return record, fmt.Errorf("observed issue project authority %q does not match sealed authority %q", projectAuthority, next.ProjectAuthority)
	}
	if err := model.ValidateIssueCreateIntent(next); err != nil {
		return record, err
	}
	if err := validateIssueCreateIntentInvariants(next); err != nil {
		return record, err
	}
	record.IssueCreateIntent = &next
	return record, nil
}

// CompleteIssueCreateIntent finishes the intent with the created issue URL:
// it validates the URL and the project authority, links the issue to the
// record, and advances a record that now meets the plan readiness gates to
// the plan phase. It fails for a not-invoked or already completed intent.
func CompleteIssueCreateIntent(record model.Record, issueURL, completedAt, projectAuthority string) (model.Record, error) {
	if record.IssueCreateIntent == nil {
		return record, fmt.Errorf("no pending issue create intent")
	}
	if record.IssueCreateIntent.Status == model.IssueCreateIntentNotInvoked {
		return record, fmt.Errorf("issue create intent was not invoked; begin a retry before completion")
	}
	if record.IssueCreateIntent.Status == model.IssueCreateIntentCompleted {
		return record, fmt.Errorf("issue create intent is already completed")
	}
	if err := ValidateIssueCreateTransition(record.IssueCreateIntent.Status, model.IssueCreateIntentCompleted); err != nil {
		return record, err
	}
	canonicalURL := strings.TrimSpace(issueURL)
	if err := ValidateIssueURL(canonicalURL); err != nil {
		return record, err
	}
	if projectAuthority == "" || projectAuthority != record.IssueCreateIntent.ProjectAuthority {
		return record, fmt.Errorf("created issue project authority %q does not match sealed authority %q", projectAuthority, record.IssueCreateIntent.ProjectAuthority)
	}
	next := *record.IssueCreateIntent
	next.Status = model.IssueCreateIntentCompleted
	next.CanonicalURL = canonicalURL
	next.Failure = ""
	next.UpdatedAt = strings.TrimSpace(completedAt)
	record.IssueURL = canonicalURL
	record.IssueCreateIntent = &next
	if len(PlanReadinessMissing(record)) == 0 && PhaseRank(record.Phase) < PhaseRank(model.PhasePlan) {
		record.Phase = model.PhasePlan
	}
	return record, nil
}

func validateIssueCreateIntentRequest(request model.IssueCreateIntentRequest) error {
	for _, field := range []struct{ name, value string }{
		{"operation_id", request.OperationID},
		{"provider", request.Provider},
		{"project_authority", request.ProjectAuthority},
		{"title", request.Title},
		{"body_sha256", request.BodySHA256},
		{"started_at", request.StartedAt},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	if len(strings.TrimSpace(request.BodySHA256)) != 64 {
		return fmt.Errorf("body_sha256 must be 64 hex characters")
	}
	return nil
}

func sameIssueCreateRequest(intent model.IssueCreateIntent, request model.IssueCreateIntentRequest) bool {
	return intent.OperationID == strings.TrimSpace(request.OperationID) &&
		intent.Provider == strings.TrimSpace(request.Provider) &&
		intent.ProjectAuthority == strings.TrimSpace(request.ProjectAuthority) &&
		intent.Title == strings.TrimSpace(request.Title) &&
		intent.BodySHA256 == strings.ToLower(strings.TrimSpace(request.BodySHA256)) &&
		slices.Equal(intent.Labels, request.Labels) && slices.Equal(intent.Assignees, request.Assignees)
}
