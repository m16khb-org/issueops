package remote

import (
	"slices"
	"testing"
)

func TestTemplateSectionsMissingNamesUnfilledRepositorySections(t *testing.T) {
	template := "## What\n\n<!-- one paragraph -->\n\n## Risk\n\n### Checklist ##\n\n- [ ] Tests\n#not-a-heading\n"
	body := "## 요약\n\ntext\n\n## what\n\nfilled\n"
	if got := TemplateSectionsMissing(template, body); !slices.Equal(got, []string{"Risk", "Checklist"}) {
		t.Fatalf("missing = %v, want [Risk Checklist]", got)
	}
	if got := TemplateSectionsMissing(template, body+"## Risk\nlow\n## Checklist\n- [x] Tests\n"); len(got) != 0 {
		t.Fatalf("filled body still missing %v", got)
	}
}
