package projectdoc

import (
	"strings"
	"testing"
)

func TestPlanAppendNormalizesKindAndSlug(t *testing.T) {
	plan, err := PlanAppend(AppendInput{Kind: "Architecture Decision", Title: "  Choose SQLite + Go  ", Summary: "why"})
	if err != nil || plan.Kind != "adr" || plan.ModuleDir != "adr" || plan.Slug != "choose-sqlite-go" {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	if _, err := PlanAppend(AppendInput{Kind: "unknown", Title: "x", Summary: "y"}); err == nil || !strings.Contains(err.Error(), "unsupported record kind") {
		t.Fatalf("kind error=%v", err)
	}
	if _, err := PlanAppend(AppendInput{Kind: "caution", Summary: "y"}); err == nil || !strings.Contains(err.Error(), "title is required") {
		t.Fatalf("title error=%v", err)
	}
	if _, err := PlanAppend(AppendInput{Kind: "caution", Title: "x"}); err == nil || !strings.Contains(err.Error(), "summary is required") {
		t.Fatalf("summary error=%v", err)
	}
}
