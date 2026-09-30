package projectdoc

import (
	"strings"
	"testing"
)

func TestPlanRevisionRequiresCurrentDigestAndPreservesDryRun(t *testing.T) {
	current := "# Testing\n"
	request := RevisionInput{Content: "# Changed\n\n", Summary: "  updated  ", Evidence: []string{" ", "test"}}
	if _, err := PlanRevision(request, "TESTING.md", current, true); err == nil || !strings.Contains(err.Error(), "expected_sha256 is required") {
		t.Fatalf("missing digest error = %v", err)
	}
	request.ExpectedSHA256 = "wrong"
	if _, err := PlanRevision(request, "TESTING.md", current, true); err == nil || !strings.Contains(err.Error(), "expected_sha256 mismatch") {
		t.Fatalf("mismatch error = %v", err)
	}
	request.ExpectedSHA256 = SHA256Hex(current)
	plan, err := PlanRevision(request, "TESTING.md", current, true)
	if err != nil || plan.Action != "update" || plan.Write || plan.Content != "# Changed\n" || plan.Summary != "updated" || len(plan.Evidence) != 1 || len(plan.Warnings) != 1 {
		t.Fatalf("dry-run plan=%+v err=%v", plan, err)
	}
	request.Confirm = true
	plan, err = PlanRevision(request, "TESTING.md", current, true)
	if err != nil || !plan.Write || len(plan.Warnings) != 0 {
		t.Fatalf("confirmed plan=%+v err=%v", plan, err)
	}
	request.Content = current
	plan, err = PlanRevision(request, "TESTING.md", current, true)
	if err != nil || plan.Action != "unchanged" || plan.Write {
		t.Fatalf("unchanged plan=%+v err=%v", plan, err)
	}
}

func TestPlanRevisionTreatsExistingEmptyFileAsSHAProtected(t *testing.T) {
	request := RevisionInput{Content: "# New", Summary: "fill empty file"}
	if _, err := PlanRevision(request, "TESTING.md", "", true); err == nil || !strings.Contains(err.Error(), "expected_sha256 is required") {
		t.Fatalf("empty existing file must require digest: %v", err)
	}
	request.ExpectedSHA256 = SHA256Hex("")
	plan, err := PlanRevision(request, "TESTING.md", "", true)
	if err != nil || plan.Action != "create" || plan.CurrentSHA256 != request.ExpectedSHA256 {
		t.Fatalf("empty existing file plan=%+v err=%v", plan, err)
	}
}
