package issueopsreview

import (
	"fmt"
	"strings"
)

func ValidateUpdatedDocumentPath(input, relative string, changed bool) error {
	if relative == "" {
		return fmt.Errorf("project docs review path %q must be inside the worktree", input)
	}
	if !changed {
		return fmt.Errorf("project docs review lists %s but it is not in the current change set", relative)
	}
	return nil
}

func ValidateReviewedDocumentPath(input, relative string, rootAvailable bool) error {
	if relative == "" {
		return fmt.Errorf("project docs review reviewed path %q must be inside the worktree", input)
	}
	if relative != "AGENTS.md" && !strings.HasPrefix(relative, ".issueops/") {
		return fmt.Errorf("project docs review reviewed path %s is not a project doc (expected .issueops/... or AGENTS.md)", relative)
	}
	if !rootAvailable {
		return fmt.Errorf("project docs review cannot verify reviewed path %s without a worktree or repo root", relative)
	}
	return nil
}

func ValidateReviewedDocumentExists(relative string, exists bool) error {
	if !exists {
		return fmt.Errorf("project docs review reviewed path %s does not exist as a file", relative)
	}
	return nil
}
