package issueops

import (
	"errors"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

// ApplyIssueLink links the issue URL to record after validating it, advances
// a record that now meets the plan readiness gates to the plan phase, and
// stamps now as the update time. On error it returns an empty record.
func ApplyIssueLink(record model.Record, issueURL, now string) (model.Record, error) {
	issueURL = strings.TrimSpace(issueURL)
	if err := ValidateIssueURL(issueURL); err != nil {
		return model.Record{OK: false}, err
	}
	record.IssueURL = issueURL
	if len(PlanReadinessMissing(record)) == 0 && PhaseRank(record.Phase) < PhaseRank(model.PhasePlan) {
		record.Phase = model.PhasePlan
	}
	record.UpdatedAt = now
	return record, nil
}

// ValidateRelatedLinkType accepts the relation types of a related-issue link:
// depends-on, blocks, supersedes, follows-up, duplicates, splits-from, and
// implements. The child type is handled by the child commands.
func ValidateRelatedLinkType(linkType string) error {
	switch linkType {
	case "depends-on", "blocks", "supersedes", "follows-up", "duplicates", "splits-from", "implements":
		return nil
	default:
		return fmt.Errorf("invalid link type %q; must be one of: depends-on, blocks, supersedes, follows-up, duplicates, splits-from, implements", linkType)
	}
}

// ValidateRelationURL validates an issue URL given in the named field, so the
// error message names the flag the caller used.
func ValidateRelationURL(issueURL, field string) error {
	if err := ValidateIssueURL(issueURL); err != nil {
		return fmt.Errorf("%s %s", field, strings.TrimPrefix(err.Error(), "issue_url "))
	}
	return nil
}

// ValidateChildLinkParent requires the parent record to have a linked issue
// before a child can be linked.
func ValidateChildLinkParent(record model.Record) error {
	if strings.TrimSpace(record.IssueURL) == "" {
		return fmt.Errorf("cannot link child before linked parent issue")
	}
	return nil
}

// ErrChildIssueAlreadyLinked reports that the exact child link is already on
// the record; callers that only ensure the link treat it as done.
var ErrChildIssueAlreadyLinked = errors.New("child issue already linked")

// AppendIssueRelation appends link to the record issue links, stamping its
// creation time and the record update time. A link with the same type and URL
// already present is an error; for child links the error wraps
// ErrChildIssueAlreadyLinked. On error it returns an empty record.
func AppendIssueRelation(record model.Record, link model.IssueLink, now string) (model.Record, error) {
	for _, existing := range record.IssueLinks {
		if existing.Type == link.Type && existing.URL == link.URL {
			if link.Type == "child" {
				return model.Record{OK: false}, fmt.Errorf("%w: %s", ErrChildIssueAlreadyLinked, link.URL)
			}
			return model.Record{OK: false}, fmt.Errorf("related issue already linked as %s: %s", link.Type, link.URL)
		}
	}
	link.Title = strings.TrimSpace(link.Title)
	link.CreatedAt = now
	record.IssueLinks = append(append([]model.IssueLink(nil), record.IssueLinks...), link)
	record.UpdatedAt = now
	return record, nil
}
