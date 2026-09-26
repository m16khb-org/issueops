package projectdoc

import (
	"fmt"
	"strings"
)

type RevisionInput struct {
	Content        string
	ExpectedSHA256 string
	Summary        string
	Evidence       []string
	Confirm        bool
}

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
