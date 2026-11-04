package issueopsreview

import (
	"fmt"
	"strings"
)

// ValidateUpdatedDocumentPath checks a doc listed as updated: relative is its
// worktree-relative path (empty means outside the worktree), and changed says whether
// it is in the current change set.
func ValidateUpdatedDocumentPath(input, relative string, changed bool) error {
	if relative == "" {
		return fmt.Errorf("project docs review path %q must be inside the worktree", input)
	}
	if !changed {
		return fmt.Errorf("project docs review lists %s but it is not in the current change set", relative)
	}
	return nil
}

// ValidateReviewedDocumentPath checks a doc listed as reviewed: it must be inside the
// worktree, be AGENTS.md or under .issueops/, not be cycle material, and rootAvailable
// must be true so existence can be verified.
func ValidateReviewedDocumentPath(input, relative string, rootAvailable bool) error {
	if relative == "" {
		return fmt.Errorf("project docs review reviewed path %q must be inside the worktree", input)
	}
	if relative != "AGENTS.md" && !strings.HasPrefix(relative, ".issueops/") {
		// QA: agents in repos without project docs passed README.md.
		return fmt.Errorf("project docs review reviewed path %s is not a project doc (expected .issueops/... or AGENTS.md); when the worktree has no project doc, record no-change without --reviewed-doc", relative)
	}
	// Cycle material is not a project doc (ProjectDocsPresent skips it too).
	// QA: an agent with no project docs passed .issueops/issues/<n>/plan.md.
	if strings.HasPrefix(relative, ".issueops/issues/") || strings.HasPrefix(relative, ".issueops/gates/") {
		return fmt.Errorf("project docs review reviewed path %s is cycle material, not a project doc", relative)
	}
	if !rootAvailable {
		return fmt.Errorf("project docs review cannot verify reviewed path %s without a worktree or repo root", relative)
	}
	return nil
}

// ValidateReviewedDocumentExists rejects a reviewed doc path that is not an existing file.
func ValidateReviewedDocumentExists(relative string, exists bool) error {
	if !exists {
		return fmt.Errorf("project docs review reviewed path %s does not exist as a file", relative)
	}
	return nil
}
