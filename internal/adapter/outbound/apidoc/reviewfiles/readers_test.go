package reviewfiles

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func writeRepoFile(t *testing.T, repo, rel, body string) string {
	t.Helper()
	path := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadFileResolvesRelativeToRepoAndReportsMissing(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, repo, "src/a.ts", "export {}")

	got, err := ReadFile(repo, "./src/../src/a.ts")
	if err != nil || got != "export {}" {
		t.Fatalf("ReadFile = %q, %v", got, err)
	}
	if _, err := ReadFile(repo, "src/missing.ts"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file error = %v, want ErrNotExist", err)
	}
}

func TestReadResultJoinsRelativePathsButKeepsAbsolutePaths(t *testing.T) {
	repo, elsewhere := t.TempDir(), t.TempDir()
	rel := writeRepoFile(t, repo, "result.json", "relative")
	abs := writeRepoFile(t, elsewhere, "result.json", "absolute")

	path, data, err := ReadResult(repo, "result.json")
	if err != nil || path != rel || string(data) != "relative" {
		t.Fatalf("relative ReadResult = %q %q %v", path, data, err)
	}
	path, data, err = ReadResult(repo, abs)
	if err != nil || path != abs || string(data) != "absolute" {
		t.Fatalf("absolute ReadResult = %q %q %v", path, data, err)
	}
	path, data, err = ReadResult(repo, "nope.json")
	if !errors.Is(err, fs.ErrNotExist) || path != filepath.Join(repo, "nope.json") || data != nil {
		t.Fatalf("missing ReadResult = %q %q %v", path, data, err)
	}
}

func TestModeReadsApiDocModeFromSpecFrontMatterOnly(t *testing.T) {
	cases := []struct {
		name, spec, want string
	}{
		{"front matter value", "---\ntitle: x\napi_doc_mode : strict \n---\nbody\n", "strict"},
		{"missing key", "---\ntitle: x\n---\n", ""},
		{"key after closing delimiter is body text", "---\ntitle: x\n---\napi_doc_mode: strict\n", ""},
		{"no front matter", "api_doc_mode: strict\n", ""},
		{"unterminated front matter still reads key", "---\napi_doc_mode: advisory\n", "advisory"},
		{"empty value", "---\napi_doc_mode:\n---\n", ""},
		{"empty file", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			writeRepoFile(t, repo, ".issueops/OPEN_API_SPEC.md", tc.spec)
			if got := Mode(repo); got != tc.want {
				t.Fatalf("Mode = %q, want %q", got, tc.want)
			}
		})
	}
	if got := Mode(t.TempDir()); got != "" {
		t.Fatalf("Mode without spec = %q, want empty", got)
	}
}
