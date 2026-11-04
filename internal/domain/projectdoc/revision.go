package projectdoc

import (
	"fmt"
	"strings"
)

// RevisionInput is a request to replace one project document: the full new Content, the ExpectedSHA256 of the current file (required when it exists), a Summary of the change, supporting Evidence, and Confirm to actually write.
type RevisionInput struct {
	Content        string
	ExpectedSHA256 string
	Summary        string
	Evidence       []string
	Confirm        bool
}

// RevisionPlan is the planned outcome of a revision: the normalized Content, the digests before and after, the Action ("create", "update", or "unchanged"), and whether the caller should Write. Warnings include the dry-run notice when Confirm was not set.
type RevisionPlan struct {
	Content       string
	Summary       string
	CurrentSHA256 string
	NextSHA256    string
	Action        string
	Evidence      []string
	Warnings      []string
	Write         bool
}

// ValidateRevisionInput reports an error when the revision has no content or no summary.
func ValidateRevisionInput(request RevisionInput) error {
	content := strings.TrimRight(request.Content, "\n") + "\n"
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("content is required")
	}
	if strings.TrimSpace(request.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	return nil
}

// PlanRevision decides what a revision does to one document without writing it.
// When the document exists, request.ExpectedSHA256 must be present and match
// the SHA-256 of current, otherwise it returns an error. Write is set only when
// request.Confirm is true and the content changes.
func PlanRevision(request RevisionInput, rel, current string, exists bool) (RevisionPlan, error) {
	if err := ValidateRevisionInput(request); err != nil {
		return RevisionPlan{}, err
	}
	content := strings.TrimRight(request.Content, "\n") + "\n"
	summary := strings.TrimSpace(request.Summary)
	currentSHA := ""
	if exists {
		currentSHA = SHA256Hex(current)
	}
	if currentSHA != "" {
		expected := strings.TrimSpace(request.ExpectedSHA256)
		if expected == "" {
			return RevisionPlan{}, fmt.Errorf("expected_sha256 is required when updating an existing project doc; call project_docs_read first")
		}
		if expected != currentSHA {
			return RevisionPlan{}, fmt.Errorf("expected_sha256 mismatch for %s: current %s", rel, currentSHA)
		}
	}
	action := "create"
	if current != "" {
		if current == content {
			action = "unchanged"
		} else {
			action = "update"
		}
	}
	plan := RevisionPlan{
		Content: content, Summary: summary, CurrentSHA256: currentSHA, NextSHA256: SHA256Hex(content),
		Action: action, Evidence: NonEmptyStrings(request.Evidence), Write: request.Confirm && action != "unchanged",
		Warnings: []string{},
	}
	if !request.Confirm {
		plan.Warnings = append(plan.Warnings, "dry_run_only: pass confirm=true to write the updated .issueops document")
	}
	return plan, nil
}
