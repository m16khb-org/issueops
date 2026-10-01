package riskqa

import "testing"

func TestPlanFromPathsPreservesTierOrderAndDeduplicatesPaths(t *testing.T) {
	plan := PlanFromPaths([]string{"internal/worker/task.go", "internal/worker/task.go", ".issueops/TESTING.md"})
	if plan.Tier != "elevated" || len(plan.ChangedPaths) != 2 || plan.ChangedPaths[0] != ".issueops/TESTING.md" || plan.ChangedPaths[1] != "internal/worker/task.go" {
		t.Fatalf("path normalization and tier=%+v", plan)
	}
	if len(plan.Commands) != 2 || plan.Commands[0] != "go test -race ./... -count=1" || plan.Commands[1] != "go vet ./..." {
		t.Fatalf("command order=%+v", plan.Commands)
	}
}

func TestPlanPreservesWhitespacePathBytes(t *testing.T) {
	values := []string{" x.go", "x.go", "x.go ", "\nx.go\n", " x.go", ""}
	plan := PlanFromPaths(values)
	if len(plan.ChangedPaths) != 4 {
		t.Fatalf("paths collapsed: %#v", plan.ChangedPaths)
	}
	for _, want := range values[:4] {
		found := false
		for _, got := range plan.ChangedPaths {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("lost %q", want)
		}
	}
}
