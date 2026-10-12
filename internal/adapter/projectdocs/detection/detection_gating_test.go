package detection

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFrameworksParsesGoModOnlyWhenListed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\nrequire github.com/spf13/cobra v1.0.0\n")
	writeFile(t, filepath.Join(dir, "package.json"), "{}")

	if got := Frameworks(dir, []string{"package.json"}, func(string) {}); len(got) != 0 {
		t.Fatalf("go.mod not in files must not be read, got %v", got)
	}
	if got := Frameworks(dir, []string{"go.mod"}, func(string) {}); !slices.Equal(got, []string{"Cobra"}) {
		t.Fatalf("got %v, want [Cobra]", got)
	}
}

func TestProjectTypesEvidenceAndLanguageGate(t *testing.T) {
	dir := t.TempDir()
	var evidence []string
	add := func(e string) { evidence = append(evidence, e) }

	if got := ProjectTypes(dir, nil, nil, true, add); !slices.Equal(got, []string{"monorepo"}) || len(evidence) != 0 {
		t.Fatalf("monorepo only: types=%v evidence=%v", got, evidence)
	}
	if got := ProjectTypes(dir, nil, nil, false, add); len(got) != 0 {
		t.Fatalf("no signals must yield no types, got %v", got)
	}
	if got := ProjectTypes(dir, []string{"Ruby"}, nil, false, add); !slices.Equal(got, []string{"library"}) {
		t.Fatalf("language only: got %v", got)
	}

	if err := os.Mkdir(filepath.Join(dir, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	evidence = nil
	if got := ProjectTypes(dir, nil, nil, false, add); !slices.Equal(got, []string{"cli"}) || !slices.Equal(evidence, []string{"cmd/"}) {
		t.Fatalf("cmd dir: types=%v evidence=%v", got, evidence)
	}
}
