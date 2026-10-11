package quality

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "issueops/internal/contract/quality"
	"issueops/internal/testsupport"
)

const coverageSampleOutput = "ok  \tissueops/pkg/a\t0.1s\tcoverage: 50.0% of statements\n" +
	"?   \tissueops/pkg/b\t[no test files]\n" +
	"ok  \tissueops/pkg/c\t0.2s\tcoverage: 87.5% of statements\n"

func newCoverageRepo(t *testing.T) string {
	t.Helper()
	testsupport.IsolateGitConfig(t)
	root := t.TempDir()
	sourceGit(t, root, "init", "-q")
	writeProductFixture(t, root, "tracked.go", "package a\n")
	sourceGit(t, root, "add", ".")
	sourceGit(t, root, "commit", "-qm", "fixture")
	return root
}

func mustFingerprint(t *testing.T, root string) string {
	t.Helper()
	fingerprint, err := CoverageFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

func fixedCacheBase(base string) func() (string, error) {
	return func() (string, error) { return base, nil }
}

func TestCoverageFingerprintTracksRepositoryStateAndRelevantEnvironment(t *testing.T) {
	root := newCoverageRepo(t)
	base := mustFingerprint(t, root)
	if again := mustFingerprint(t, root); again != base {
		t.Fatalf("fingerprint is not stable: %s vs %s", base, again)
	}

	t.Run("unrelated environment is ignored", func(t *testing.T) {
		t.Setenv("ISSUEOPS_UNRELATED_COVERAGE_VAR", "1")
		if got := mustFingerprint(t, root); got != base {
			t.Fatal("unrelated env changed the fingerprint")
		}
	})
	t.Run("go environment participates", func(t *testing.T) {
		t.Setenv("GOFLAGS", "-tags=fingerprinttest")
		if got := mustFingerprint(t, root); got == base {
			t.Fatal("GOFLAGS change did not alter the fingerprint")
		}
	})
	t.Run("ignored files are ignored", func(t *testing.T) {
		writeProductFixture(t, root, ".gitignore", "ignored.txt\n")
		sourceGit(t, root, "add", ".gitignore")
		sourceGit(t, root, "commit", "-qm", "ignore")
		withIgnore := mustFingerprint(t, root)
		writeProductFixture(t, root, "ignored.txt", "x")
		if got := mustFingerprint(t, root); got != withIgnore {
			t.Fatal("gitignored file changed the fingerprint")
		}
	})
	t.Run("tracked edit", func(t *testing.T) {
		before := mustFingerprint(t, root)
		writeProductFixture(t, root, "tracked.go", "package a\nvar X = 1\n")
		if got := mustFingerprint(t, root); got == before {
			t.Fatal("tracked modification did not alter the fingerprint")
		}
	})
	t.Run("untracked content", func(t *testing.T) {
		writeProductFixture(t, root, "new.go", "package a // v1\n")
		first := mustFingerprint(t, root)
		writeProductFixture(t, root, "new.go", "package a // v2\n")
		if got := mustFingerprint(t, root); got == first {
			t.Fatal("untracked content change did not alter the fingerprint")
		}
	})
	t.Run("untracked symlink target", func(t *testing.T) {
		if err := os.Symlink("one", filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		first := mustFingerprint(t, root)
		if err := os.Remove(filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("two", filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		if got := mustFingerprint(t, root); got == first {
			t.Fatal("symlink retarget did not alter the fingerprint")
		}
	})
	t.Run("new commit", func(t *testing.T) {
		sourceGit(t, root, "add", ".")
		before := mustFingerprint(t, root)
		sourceGit(t, root, "commit", "-qm", "again")
		if got := mustFingerprint(t, root); got == before {
			t.Fatal("HEAD change did not alter the fingerprint")
		}
	})
}

func TestCoverageFingerprintFailsWithoutUsableGitHead(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	t.Run("not a repository", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
		if _, err := CoverageFingerprint(root); err == nil {
			t.Fatal("expected error outside a repository")
		}
	})
	t.Run("repository without commits", func(t *testing.T) {
		root := t.TempDir()
		sourceGit(t, root, "init", "-q")
		if _, err := CoverageFingerprint(root); err == nil {
			t.Fatal("expected error without HEAD")
		}
	})
}

func TestHashCoverageUntrackedFilesRejectsEscapesAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	writeProductFixture(t, root, "ok.txt", "x")
	for _, bad := range []string{"../outside", "/abs/path", ".", "a/../../outside"} {
		var sink strings.Builder
		err := hashCoverageUntrackedFiles(&sink, root, [][]byte{[]byte(bad)})
		if err == nil || !strings.Contains(err.Error(), "escapes repository") {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
	var sink strings.Builder
	if err := hashCoverageUntrackedFiles(&sink, root, [][]byte{[]byte("vanished.txt")}); err == nil {
		t.Error("a path listed by git but absent on disk must fail")
	}
	sink.Reset()
	if err := hashCoverageUntrackedFiles(&sink, root, [][]byte{nil, []byte("ok.txt"), {}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sink.String(), "path:ok.txt") || !strings.HasSuffix(sink.String(), "x") {
		t.Errorf("hashed stream = %q", sink.String())
	}
}

func TestHashCoverageUntrackedFilesSkipsNonRegularFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProductFixture(t, root, "dir/inner.txt", "SECRET-CONTENT")
	var sink strings.Builder
	if err := hashCoverageUntrackedFiles(&sink, root, [][]byte{[]byte("dir")}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sink.String(), "path:dir") || strings.Contains(sink.String(), "SECRET-CONTENT") {
		t.Errorf("hashed stream = %q", sink.String())
	}
}

func TestHashCoverageFileContentReportsOpenFailure(t *testing.T) {
	var sink strings.Builder
	if err := hashCoverageFileContent(&sink, filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("expected open error")
	}
}

func TestCoverageFingerprintBudgetEnforcesEveryLimit(t *testing.T) {
	t.Run("file count", func(t *testing.T) {
		var budget coverageFingerprintBudget
		for range coverageFingerprintFileCountLimit {
			if err := budget.admit("f", 0); err != nil {
				t.Fatal(err)
			}
		}
		if err := budget.admit("f", 0); err == nil || !strings.Contains(err.Error(), "untracked files") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("single file size", func(t *testing.T) {
		var budget coverageFingerprintBudget
		if err := budget.admit("exact", coverageFingerprintFileLimit); err != nil {
			t.Fatalf("file at the limit must be admitted: %v", err)
		}
		err := (&coverageFingerprintBudget{}).admit("big.bin", coverageFingerprintFileLimit+1)
		if err == nil || !strings.Contains(err.Error(), "big.bin") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("total bytes", func(t *testing.T) {
		var budget coverageFingerprintBudget
		admitted := 0
		var err error
		for err == nil {
			err = budget.admit("f", coverageFingerprintFileLimit)
			if err == nil {
				admitted++
			}
		}
		if admitted != coverageFingerprintTotalFileLimit/coverageFingerprintFileLimit || !strings.Contains(err.Error(), "untracked bytes") {
			t.Fatalf("admitted=%d err=%v", admitted, err)
		}
	})
}

func TestCoverageCachePathIsStablePerRootAndPropagatesBaseErrors(t *testing.T) {
	base := t.TempDir()
	a, err := CoverageCachePath("/work/a", fixedCacheBase(base))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := CoverageCachePath("/work/b", fixedCacheBase(base))
	same, _ := CoverageCachePath("/work//a/", fixedCacheBase(base))
	if a == b {
		t.Error("different roots share a cache file")
	}
	if a != same {
		t.Error("equivalent roots must share a cache file")
	}
	if filepath.Dir(a) != base || filepath.Ext(a) != ".json" {
		t.Errorf("path = %s", a)
	}
	boom := errors.New("no cache dir")
	if _, err := CoverageCachePath("/work/a", func() (string, error) { return "", boom }); !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}

func TestDefaultCoverageCacheBaseUsesUserCacheDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "")
	got, err := DefaultCoverageCacheBase()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, home) || !strings.HasSuffix(got, filepath.Join("issueops", "quality-coverage")) {
		t.Fatalf("base = %s", got)
	}
	t.Setenv("HOME", "")
	if _, err := DefaultCoverageCacheBase(); err == nil {
		t.Fatal("expected error when no cache dir can be determined")
	}
}

func TestRunGoTestCoverageServesExactFingerprintFromCacheAndInvalidatesOnChange(t *testing.T) {
	root := newCoverageRepo(t)
	base := t.TempDir()
	runs := 0
	execute := func(ctx context.Context, dir string) (string, error) {
		runs++
		if dir != root {
			t.Errorf("execute dir = %s", dir)
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > CoverageCommandTimeout {
			t.Errorf("execute ctx deadline = %v ok=%v", deadline, ok)
		}
		return coverageSampleOutput, nil
	}
	first, err := RunGoTestCoverage(root, execute, fixedCacheBase(base))
	if err != nil || first != coverageSampleOutput || runs != 1 {
		t.Fatalf("first: %q %v runs=%d", first, err, runs)
	}
	cached, err := RunGoTestCoverage(root, execute, fixedCacheBase(base))
	if err != nil || runs != 1 {
		t.Fatalf("second call must hit the cache: err=%v runs=%d", err, runs)
	}
	want := "issueops/pkg/a coverage: 50.0% of statements\nissueops/pkg/c coverage: 87.5% of statements\n"
	if cached != want {
		t.Fatalf("cached render = %q, want %q", cached, want)
	}
	writeProductFixture(t, root, "tracked.go", "package a\nvar Changed = true\n")
	if _, err := RunGoTestCoverage(root, execute, fixedCacheBase(base)); err != nil || runs != 2 {
		t.Fatalf("changed tree must re-execute: err=%v runs=%d", err, runs)
	}
}

func TestRunGoTestCoverageDoesNotCacheFailuresOrUnparseableOutput(t *testing.T) {
	root := newCoverageRepo(t)
	t.Run("execution error", func(t *testing.T) {
		base := t.TempDir()
		boom := errors.New("tests failed")
		out, err := RunGoTestCoverage(root, func(context.Context, string) (string, error) {
			return "FAIL\tissueops/pkg/a\tcoverage: 10.0% of statements\n", boom
		}, fixedCacheBase(base))
		if !errors.Is(err, boom) || !strings.Contains(out, "FAIL") {
			t.Fatalf("out=%q err=%v", out, err)
		}
		if entries, _ := os.ReadDir(base); len(entries) != 0 {
			t.Fatalf("failed run was cached: %v", entries)
		}
	})
	t.Run("no coverage lines", func(t *testing.T) {
		base := t.TempDir()
		runs := 0
		execute := func(context.Context, string) (string, error) { runs++; return "ok  \tpkg\t0.1s\n", nil }
		for range 2 {
			if _, err := RunGoTestCoverage(root, execute, fixedCacheBase(base)); err != nil {
				t.Fatal(err)
			}
		}
		if runs != 2 {
			t.Fatalf("output without coverage data was served from cache (runs=%d)", runs)
		}
	})
	t.Run("cache directory unavailable", func(t *testing.T) {
		out, err := RunGoTestCoverage(root, func(context.Context, string) (string, error) {
			return coverageSampleOutput, nil
		}, func() (string, error) { return "", errors.New("no cache") })
		if err != nil || out != coverageSampleOutput {
			t.Fatalf("out=%q err=%v", out, err)
		}
	})
}

func TestRunGoTestCoverageWithoutGitAlwaysExecutes(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	base := t.TempDir()
	runs := 0
	execute := func(context.Context, string) (string, error) { runs++; return coverageSampleOutput, nil }
	for range 2 {
		out, err := RunGoTestCoverage(root, execute, fixedCacheBase(base))
		if err != nil || out != coverageSampleOutput {
			t.Fatalf("out=%q err=%v", out, err)
		}
	}
	if runs != 2 {
		t.Fatalf("runs = %d, want 2 (no fingerprint, no cache)", runs)
	}
	if entries, _ := os.ReadDir(base); len(entries) != 0 {
		t.Fatalf("cache written without fingerprint: %v", entries)
	}
}

func TestReadCoverageCacheRejectsStaleOrMalformedEntries(t *testing.T) {
	root := "/work/repo"
	base := t.TempDir()
	path, err := CoverageCachePath(root, fixedCacheBase(base))
	if err != nil {
		t.Fatal(err)
	}
	packages := []contract.CoveragePackage{{Package: "p", Coverage: 12.34}}
	write := func(entry coverageCacheEntry) {
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := readCoverageCache(root, "fp", fixedCacheBase(base)); ok {
		t.Fatal("missing cache file reported as hit")
	}
	write(coverageCacheEntry{Version: coverageCacheVersion, Fingerprint: "fp", Packages: packages})
	got, ok := readCoverageCache(root, "fp", fixedCacheBase(base))
	if !ok || got != "p coverage: 12.3% of statements\n" {
		t.Fatalf("valid entry: %q ok=%v", got, ok)
	}
	for name, entry := range map[string]coverageCacheEntry{
		"old version":           {Version: coverageCacheVersion - 1, Fingerprint: "fp", Packages: packages},
		"different fingerprint": {Version: coverageCacheVersion, Fingerprint: "other", Packages: packages},
		"no packages":           {Version: coverageCacheVersion, Fingerprint: "fp"},
	} {
		write(entry)
		if _, ok := readCoverageCache(root, "fp", fixedCacheBase(base)); ok {
			t.Errorf("%s accepted", name)
		}
	}
	if err := os.WriteFile(path, []byte("{corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := readCoverageCache(root, "fp", fixedCacheBase(base)); ok {
		t.Error("corrupt JSON accepted")
	}
	if _, ok := readCoverageCache(root, "fp", func() (string, error) { return "", errors.New("x") }); ok {
		t.Error("cache base error reported as hit")
	}
}

func TestWriteCoverageCacheStoresPrivateFileWithoutTemporaries(t *testing.T) {
	base := filepath.Join(t.TempDir(), "nested", "cache")
	writeCoverageCache("/work/repo", "fp", coverageSampleOutput, fixedCacheBase(base))
	path, _ := CoverageCachePath("/work/repo", fixedCacheBase(base))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(base)
	if len(entries) != 1 {
		t.Errorf("leftover files: %v", entries)
	}
	data, _ := os.ReadFile(path)
	var entry coverageCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Fingerprint != "fp" || entry.Version != coverageCacheVersion || len(entry.Packages) != 2 {
		t.Errorf("entry = %+v", entry)
	}
}

func TestWriteCoverageCacheSkipsWhenDirectoryCannotBeCreated(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	writeProductFixture(t, filepath.Dir(blocker), "file", "x")
	writeCoverageCache("/work/repo", "fp", coverageSampleOutput, fixedCacheBase(filepath.Join(blocker, "sub")))
	if info, err := os.Stat(blocker); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("blocker disturbed: %v", err)
	}
}

func TestExecuteGoTestCoverageRunsGoTestInRoot(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	root := t.TempDir()
	writeProductFixture(t, root, "go.mod", "module fixture\n\ngo 1.21\n")
	writeProductFixture(t, root, "a.go", "package a\n\nfunc F() int { return 1 }\n")
	writeProductFixture(t, root, "a_test.go", "package a\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) { if F() != 1 { t.Fatal() } }\n")

	out, err := ExecuteGoTestCoverage(t.Context(), root)
	if err != nil || !strings.Contains(out, "coverage: 100.0% of statements") {
		t.Fatalf("out=%q err=%v", out, err)
	}

	writeProductFixture(t, root, "a_test.go", "package a\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) { t.Fatal(\"boom\") }\n")
	out, err = ExecuteGoTestCoverage(t.Context(), root)
	if err == nil || !strings.Contains(out, "boom") {
		t.Fatalf("failing tests must return error with output: out=%q err=%v", out, err)
	}
}

func TestExecuteGoTestCoverageReportsDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := ExecuteGoTestCoverage(ctx, t.TempDir()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}

func TestBoundedQualityBufferAcrossWrites(t *testing.T) {
	buffer := NewBoundedQualityBuffer(5)
	for _, chunk := range []string{"ab", "cd", "ef", "gh"} {
		if n, err := buffer.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatalf("write %q = %d, %v", chunk, n, err)
		}
	}
	if buffer.String() != "abcde" || !buffer.Truncated() {
		t.Fatalf("got %q truncated=%v", buffer.String(), buffer.Truncated())
	}
	exact := NewBoundedQualityBuffer(3)
	_, _ = exact.Write([]byte("abc"))
	if exact.Truncated() || exact.String() != "abc" {
		t.Fatalf("exact fit: %q truncated=%v", exact.String(), exact.Truncated())
	}
	_, _ = exact.Write([]byte("d"))
	if !exact.Truncated() || exact.String() != "abc" {
		t.Fatalf("overflow after full: %q truncated=%v", exact.String(), exact.Truncated())
	}
}
