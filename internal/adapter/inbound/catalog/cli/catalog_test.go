package cli

import (
	"strings"
	"testing"
)

func TestUsageCatalogKeepsLifecycleAndChildHelpAligned(t *testing.T) {
	full, lifecycle, child := Usage("catalog-probe"), LifecycleUsage(), ChildUsage()
	if !strings.HasPrefix(full, "issueops catalog-probe\n") {
		t.Fatal("version missing from root usage")
	}
	for _, line := range strings.Split(lifecycle, "\n") {
		if strings.HasPrefix(line, "  issueops ") && !strings.Contains(full, line) {
			t.Fatalf("root help dropped %q", line)
		}
		if strings.HasPrefix(line, "  issueops child ") && !strings.Contains(child, line) {
			t.Fatalf("child help dropped %q", line)
		}
	}
	for _, text := range []string{lifecycle, child} {
		if !strings.Contains(text, "RECORD_ACTOR_FLAGS:") || !strings.Contains(text, "ACTOR_FLAGS:") {
			t.Fatal("actor legend missing")
		}
	}
	if strings.Contains(child, "  issueops execution ") {
		t.Fatal("child help contains another namespace")
	}
	first, second := Commands(), Commands()
	first[0].Name = "mutated"
	if second[0].Name == "mutated" {
		t.Fatal("command catalogs share mutable descriptors")
	}
}
