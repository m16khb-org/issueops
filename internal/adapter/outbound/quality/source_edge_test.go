package quality

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/testsupport"
)

func TestCanonicalRepositoryResolvesSymlinksAndRelativePaths(t *testing.T) {
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	got, err := CanonicalRepository(link + "/.")
	if err != nil || got != real {
		t.Fatalf("CanonicalRepository(link) = %q, %v; want %q", got, err, real)
	}
	t.Chdir(real)
	got, err = CanonicalRepository(".")
	if err != nil || got != real {
		t.Fatalf("CanonicalRepository(.) = %q, %v; want %q", got, err, real)
	}
}

func TestCanonicalRepositoryRejectsMissingPath(t *testing.T) {
	_, err := CanonicalRepository(filepath.Join(t.TempDir(), "absent"))
	if err == nil || !strings.Contains(err.Error(), "symlinks") {
		t.Fatalf("err = %v", err)
	}
}

func TestRelOrAbsFallsBackToInputWhenNotRelatable(t *testing.T) {
	root := t.TempDir()
	if got := relOrAbs(root, filepath.Join(root, "a", "b.go")); got != filepath.Join("a", "b.go") {
		t.Errorf("inside root = %q", got)
	}
	// An absolute root cannot be made relative to a relative path.
	if got := relOrAbs(root, "rel/x.go"); got != "rel/x.go" {
		t.Errorf("unrelatable path = %q, want unchanged", got)
	}
}

func TestSNRClassifiesCommentsBlankAndStructuralLines(t *testing.T) {
	root := t.TempDir()
	writeProductFixture(t, root, "a.go", strings.Join([]string{
		"package a",            // signal
		"",                     // noise: blank
		"// line comment",      // noise
		"/* one-liner */",      // noise, does not open a block
		"/* opens",             // noise, opens block
		"still comment code()", // noise: inside block
		"closes */",            // noise, closes block
		"x := 1",               // signal: block closed
		"})",                   // noise: structural only
		"{",                    // noise: structural only
		"f(a, b)",              // signal
	}, "\n")+"\n")
	signal, noise, err := snrCountFile(filepath.Join(root, "a.go"))
	if err != nil || signal != 3 || noise != 8 {
		t.Fatalf("signal=%d noise=%d err=%v", signal, noise, err)
	}
	result, err := ComputeCodeSNR(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.SignalLines != 3 || result.NoiseLines != 8 || result.TotalLines != 11 || result.Ratio != 0.2727 {
		t.Fatalf("result = %+v", result)
	}
}

func TestSNRCountFileReportsOpenAndScanErrors(t *testing.T) {
	if _, _, err := snrCountFile(filepath.Join(t.TempDir(), "absent.go")); err == nil {
		t.Error("missing file must fail")
	}
	root := t.TempDir()
	writeProductFixture(t, root, "long.go", "package a\n"+strings.Repeat("x", 1024*1024+10)+"\n")
	if _, _, err := snrCountFile(filepath.Join(root, "long.go")); err == nil {
		t.Error("line over scanner limit must be an error, not a silent truncation")
	}
	if _, err := ComputeCodeSNR(root); err == nil {
		t.Error("ComputeCodeSNR must propagate the per-file failure")
	}
}

func TestComputeCodeSNREmptyRepositoryHasZeroRatio(t *testing.T) {
	result, err := ComputeCodeSNR(t.TempDir())
	if err != nil || result.TotalLines != 0 || result.Ratio != 0 {
		t.Fatalf("result = %+v err=%v", result, err)
	}
}

func TestProductionSourceRejectsMissingAndNonDirectoryRoots(t *testing.T) {
	dir := t.TempDir()
	writeProductFixture(t, dir, "file.go", productSource)
	for name, root := range map[string]string{
		"missing":      filepath.Join(dir, "absent"),
		"regular file": filepath.Join(dir, "file.go"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ComputeCodeSNR(root); err == nil {
				t.Error("SNR accepted invalid root")
			}
			functions, warnings := CollectBranchFunctions(root)
			if len(functions) != 0 || len(warnings) != 1 {
				t.Errorf("functions=%v warnings=%v", functions, warnings)
			}
		})
	}
}

func TestProductionSourceWalkReportsUnreadableDirectoryButKeepsRest(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits are not enforced for root")
	}
	root := t.TempDir()
	writeProductFixture(t, root, "ok.go", productSource)
	writeProductFixture(t, root, "locked/hidden.go", productSource)
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	functions, warnings := CollectBranchFunctions(root)
	if len(functions) != 1 || functions[0].File != "ok.go" || len(warnings) == 0 {
		t.Fatalf("functions=%v warnings=%v", functions, warnings)
	}
	if _, err := ComputeCodeSNR(root); err == nil {
		t.Fatal("SNR accepted an incomplete scan")
	}
}

func TestProductionSourceSkipsTrackedFilesThatNoLongerExistOrLostTheirDirectory(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	root := t.TempDir()
	sourceGit(t, root, "init", "-q")
	writeProductFixture(t, root, "keep.go", productSource)
	writeProductFixture(t, root, "gone.go", productSource)
	writeProductFixture(t, root, "pkg/inner.go", productSource)
	sourceGit(t, root, "add", ".")
	sourceGit(t, root, "commit", "-qm", "fixture")
	if err := os.Remove(filepath.Join(root, "gone.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "pkg")); err != nil {
		t.Fatal(err)
	}
	writeProductFixture(t, root, "pkg", "now a file")
	functions, warnings := CollectBranchFunctions(root)
	if len(warnings) != 0 || len(functions) != 1 || functions[0].File != "keep.go" {
		t.Fatalf("functions=%v warnings=%v", functions, warnings)
	}
	if _, err := ComputeCodeSNR(root); err != nil {
		t.Fatalf("SNR failed on deleted tracked files: %v", err)
	}
}

func TestProductionFileFilterRejectsPathsEscapingRoot(t *testing.T) {
	root := t.TempDir()
	filter := &productionFileFilter{root: root, seen: map[string]bool{}, files: []string{}}
	for _, relative := range []string{"../outside.go", "..", "/etc/passwd.go"} {
		filter.add(relative)
	}
	filter.add("")
	if len(filter.files) != 0 || len(filter.scanErrors) != 3 {
		t.Fatalf("files=%v errors=%v", filter.files, filter.scanErrors)
	}
}

func TestCollectBranchFunctionsCountsEveryBranchKindAndOrdersDeterministically(t *testing.T) {
	root := t.TempDir()
	writeProductFixture(t, root, "b.go", "package p\nfunc Two() { if true {}; if true {} }\n")
	writeProductFixture(t, root, "a.go", `package p
func Two() { if true {}; if true {} }
func AllKinds(v any, ch chan int, xs []int) {
	if true {}
	for {}
	for range xs {}
	switch 1 { case 1: }
	switch v.(type) { case int: }
	select { case <-ch: default: }
}
func Declared()
func Plain() {}
`)
	functions, warnings := CollectBranchFunctions(root)
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	type row struct {
		file, name string
		branches   int
	}
	var got []row
	for _, fn := range functions {
		got = append(got, row{filepath.ToSlash(fn.File), fn.Name, fn.Branches})
	}
	// if=1, for=1, range=1, switch+case=2, typeswitch+case=2, select+2 comm=3.
	want := []row{{"a.go", "AllKinds", 10}, {"a.go", "Two", 2}, {"b.go", "Two", 2}, {"a.go", "Plain", 0}}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order[%d] = %v, want %v (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestAuditSeparatorRequiresPipeWidthAndThreeDashes(t *testing.T) {
	tests := map[string]bool{
		"|---|---|":       true,
		"| :--- | ---: |": true,
		"|:-:|:-:|":       false,
		"---|---":         false,
		"|---|---|---|":   false,
		"|--|---|":        false,
		"|---|abc|":       false,
		"|---|---x|":      false,
	}
	for line, want := range tests {
		if got := auditSeparator(line, 2); got != want {
			t.Errorf("auditSeparator(%q) = %v, want %v", line, got, want)
		}
	}
}
