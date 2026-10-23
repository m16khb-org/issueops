package projectdoc

import (
	"fmt"
	"testing"
)

func TestEchoedSignalsCapsFilesAndKeepsTheTotal(t *testing.T) {
	files := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("f%03d", i)
		}
		return out
	}
	exact := ProjectSignals{Files: files(MaxEchoedSignalFiles), Languages: []string{"Go"}}
	if got := EchoedSignals(exact); len(got.Files) != MaxEchoedSignalFiles || got.FilesTotal != 0 {
		t.Fatalf("at the cap nothing is cut: %d files, total %d", len(got.Files), got.FilesTotal)
	}
	large := ProjectSignals{Files: files(MaxEchoedSignalFiles + 1), Languages: []string{"Go"}}
	got := EchoedSignals(large)
	if len(got.Files) != MaxEchoedSignalFiles || got.FilesTotal != MaxEchoedSignalFiles+1 || got.Files[0] != "f000" || got.Languages[0] != "Go" {
		t.Fatalf("over the cap: %d files, total %d", len(got.Files), got.FilesTotal)
	}
	if len(large.Files) != MaxEchoedSignalFiles+1 {
		t.Fatal("the scanned signals must stay whole for the renderer")
	}
}
