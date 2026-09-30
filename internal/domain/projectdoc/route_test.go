package projectdoc

import (
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
