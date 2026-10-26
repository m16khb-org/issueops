package cli

import (
	"strings"
	"testing"
)

// The index is what `issueops --help` prints; every command path in the full
// usage must be findable in it, or an agent searching the index misses it.
func TestUsageIndexNamesEveryCommandPath(t *testing.T) {
	index := UsageIndex("test")
	lines := map[string]string{}
	for line := range strings.SplitSeq(index, "\n") {
		name, subs, _ := strings.Cut(strings.TrimSpace(line), ": ")
		lines[name] = ", " + subs + ","
	}
	for line := range strings.SplitSeq(Usage("test"), "\n") {
		rest, ok := strings.CutPrefix(line, "  issueops ")
		if !ok {
			continue
		}
		words := strings.Fields(rest)
		subs, found := lines[words[0]]
		if !found {
			t.Errorf("index lacks command %q", words[0])
			continue
		}
		if len(words) > 1 && !strings.ContainsAny(words[1][:1], "-[(<") && strings.ToUpper(words[1]) != words[1] && !strings.Contains(subs, " "+words[1]) {
			t.Errorf("index lacks %q under %q", words[1], words[0])
		}
	}
	if strings.Contains(index, "--json") {
		t.Fatal("the index lists flags; flags belong to per-command help")
	}
}
