package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ApplyIssueLink(record model.IssueOpsRecord, issueURL, now string) (model.IssueOpsRecord, error) {
	issueURL = strings.TrimSpace(issueURL)
	if err := ValidateIssueURL(issueURL); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record.IssueURL = issueURL
	if len(PlanReadinessMissing(record)) == 0 && IssueOpsPhaseRank(record.Phase) < IssueOpsPhaseRank(model.IssueOpsPhasePlan) {
		record.Phase = model.IssueOpsPhasePlan
	}
	record.UpdatedAt = now
	return record, nil
}

func ValidateRelatedLinkType(linkType string) error {
	switch linkType {
	case "depends-on", "blocks", "supersedes", "follows-up", "duplicates", "splits-from", "implements":
		return nil
	default:
		return fmt.Errorf("invalid link type %q; must be one of: depends-on, blocks, supersedes, follows-up, duplicates, splits-from, implements", linkType)
	}
}

func ValidateRelationURL(issueURL, field string) error {
	if err := ValidateIssueURL(issueURL); err != nil {
		return fmt.Errorf("%s %s", field, strings.TrimPrefix(err.Error(), "issue_url "))
	}
	return nil
}

func ValidateChildLinkParent(record model.IssueOpsRecord) error {
	if strings.TrimSpace(record.IssueURL) == "" {
		return fmt.Errorf("cannot link child before linked parent issue")
	}
	return nil
}

func AppendIssueRelation(record model.IssueOpsRecord, link model.IssueOpsIssueLink, now string) (model.IssueOpsRecord, error) {
	for _, existing := range record.IssueLinks {
		if existing.Type == link.Type && existing.URL == link.URL {
			if link.Type == "child" {
				return model.IssueOpsRecord{OK: false}, fmt.Errorf("child issue already linked: %s", link.URL)
			}
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("related issue already linked as %s: %s", link.Type, link.URL)
		}
	}
	link.Title = strings.TrimSpace(link.Title)
	link.CreatedAt = now
	record.IssueLinks = append(append([]model.IssueOpsIssueLink(nil), record.IssueLinks...), link)
	record.UpdatedAt = now
	return record, nil
}
