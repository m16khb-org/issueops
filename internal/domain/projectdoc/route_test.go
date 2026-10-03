package projectdoc

import (
	"reflect"
	"strings"
	"testing"
)

func TestRouteDocsForTaskPreservesSpecificAndDefaultGuidance(t *testing.T) {
	for _, test := range []struct {
		task string
		want []string
	}{
		{"OpenAPI controller DTO test", []string{"AGENTS.md", ".issueops/OPEN_API_SPEC.md", ".issueops/TESTING.md"}},
		{"gitlab pull request", []string{"AGENTS.md", ".issueops/VCS.md"}},
		{"구현", []string{"AGENTS.md", ".issueops/CONSTITUTION.md", ".issueops/AGENT_WORKFLOW.md"}},
		{"", []string{"AGENTS.md", ".issueops/CONSTITUTION.md", ".issueops/TESTING.md"}},
	} {
		t.Run(test.task, func(t *testing.T) {
			routed := RouteDocsForTask(strings.ToLower(test.task))
			for _, want := range test.want {
				count := 0
				for _, doc := range routed {
					if doc.Rel == want {
						count++
					}
				}
				if count != 1 {
					t.Fatalf("%s occurs %d times in %+v", want, count, routed)
				}
			}
		})
	}
}

func TestRouteDocsForTaskTokenBoundaries(t *testing.T) {
	for _, task := range []string{"ci", "(ci)", "문서/ci", "ci-ci"} {
		t.Run(task, func(t *testing.T) {
			var paths []string
			for _, doc := range RouteDocsForTask(task) {
				paths = append(paths, doc.Rel)
			}
			want := []string{
				"AGENTS.md", ".issueops/TESTING.md", ".issueops/TECH_STACK.md",
				".issueops/AGENT_WORKFLOW.md", ".issueops/CAUTIONS.md",
			}
			if !reflect.DeepEqual(paths, want) {
				t.Fatalf("paths = %v, want %v", paths, want)
			}
		})
	}
	for _, task := range []string{"문서ci", "ci2", "xcix"} {
		t.Run(task, func(t *testing.T) {
			docs := RouteDocsForTask(task)
			if len(docs) < 2 || docs[1].Rel != ".issueops/CONSTITUTION.md" {
				t.Fatalf("expected general guidance for embedded token: %+v", docs)
			}
		})
	}
}

func TestRouteDocsForTaskPreservesFirstMatchOrder(t *testing.T) {
	var paths []string
	for _, doc := range RouteDocsForTask("implement ci pr") {
		paths = append(paths, doc.Rel)
	}
	want := []string{
		"AGENTS.md", ".issueops/CONSTITUTION.md", ".issueops/AGENT_WORKFLOW.md",
		".issueops/CONVENTIONS.md", ".issueops/CAUTIONS.md", ".issueops/TESTING.md",
		".issueops/COMMIT_POLICY.md", ".issueops/TECH_STACK.md",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}
