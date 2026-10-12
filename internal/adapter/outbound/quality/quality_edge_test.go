package quality

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"issueops/internal/testsupport"
)

func auditWarningPrefix(root string) string {
	return "audit scan " + filepath.Join(root, ".issueops", "PROJECT_AUDIT.md") + ":"
}

func TestCollectAuditItemsReportsOneBasedLineNumbersForEachRowProblem(t *testing.T) {
	root := t.TempDir()
	document := "# Open\n\n" + auditHeader +
		"| A1 | Core | Fix | P1 |\n" +
		"| A2 | Core | Fix | P9 | Small |\n" +
		"| A3 |  | Fix | P1 | Small |\n"
	writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", document)
	items, warnings := CollectAuditItems(root)
	prefix := auditWarningPrefix(root)
	want := []string{
		prefix + "5: audit row width does not match header",
		prefix + "6: invalid audit priority: require P0, P1, P2 or P3",
		prefix + "7: empty required audit cell: area",
	}
	if len(items) != 0 || !reflect.DeepEqual(warnings, want) {
		t.Fatalf("items=%+v warnings=%q want %q", items, warnings, want)
	}
}

func TestCollectAuditItemsReportsHeaderAndSeparatorLineNumbers(t *testing.T) {
	prefix := func(root string) string { return auditWarningPrefix(root) }
	t.Run("invalid header", func(t *testing.T) {
		root := t.TempDir()
		writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", "# Open\n\n| ID | Area |\n|---|---|\n")
		_, warnings := CollectAuditItems(root)
		want := prefix(root) + "3: invalid audit header: require unique ID, Area, Title, Priority, Size columns"
		if len(warnings) == 0 || warnings[0] != want {
			t.Fatalf("warnings = %q, want first %q", warnings, want)
		}
	})
	t.Run("invalid separator", func(t *testing.T) {
		root := t.TempDir()
		writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", "# Open\n\n| ID | Area | Title | Priority | Size |\n| nope |\n")
		_, warnings := CollectAuditItems(root)
		want := prefix(root) + "3: missing or invalid audit table separator"
		if len(warnings) == 0 || warnings[0] != want {
			t.Fatalf("warnings = %q, want first %q", warnings, want)
		}
	})
	t.Run("header is the final line", func(t *testing.T) {
		root := t.TempDir()
		writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", "# Open\n| ID | Area | Title | Priority | Size |")
		items, warnings := CollectAuditItems(root)
		want := prefix(root) + "2: missing or invalid audit table separator"
		if len(items) != 0 || len(warnings) == 0 || warnings[0] != want {
			t.Fatalf("items=%+v warnings=%q, want first %q", items, warnings, want)
		}
	})
}

func TestCollectAuditItemsRecognizesHeadingsUpToLevelSix(t *testing.T) {
	row := "| A1 | Core | Fix | P1 | Small |\n"
	t.Run("level six open heading", func(t *testing.T) {
		root := t.TempDir()
		writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", "## Resolved\n\n###### Open\n\n"+auditHeader+row)
		items, warnings := CollectAuditItems(root)
		if len(items) != 1 || items[0].ID != "A1" || len(warnings) != 0 {
			t.Fatalf("items=%+v warnings=%q", items, warnings)
		}
	})
	t.Run("level seven is not a heading", func(t *testing.T) {
		root := t.TempDir()
		writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", "## Resolved\n\n####### Open\n\n"+auditHeader+row)
		items, warnings := CollectAuditItems(root)
		if len(items) != 0 || len(warnings) != 1 || !strings.Contains(warnings[0], "no valid open audit table") {
			t.Fatalf("items=%+v warnings=%q", items, warnings)
		}
	})
	t.Run("bare hashes after the table", func(t *testing.T) {
		root := t.TempDir()
		writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", "# Open\n"+auditHeader+row+"###\n#\n")
		items, warnings := CollectAuditItems(root)
		if len(items) != 1 || len(warnings) != 0 {
			t.Fatalf("items=%+v warnings=%q", items, warnings)
		}
	})
}

func TestCollectAuditItemsZeroDeclarationWithoutHeadingsIsNotAccepted(t *testing.T) {
	root := t.TempDir()
	writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", auditNone+"\n")
	items, warnings := CollectAuditItems(root)
	if len(items) != 0 || len(warnings) != 1 || !strings.Contains(warnings[0], "no valid open audit table or explicit zero declaration") {
		t.Fatalf("items=%+v warnings=%q", items, warnings)
	}
}

func TestCollectBranchFunctionsOrdersEqualBranchesByFileBeforeLine(t *testing.T) {
	root := t.TempDir()
	writeProductFixture(t, root, "b.go", "package p\nfunc B() { if true {} }\n")
	writeProductFixture(t, root, "a.go", "package p\n\nfunc A() { if true {} }\n")
	functions, warnings := CollectBranchFunctions(root)
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	var got []string
	for _, fn := range functions {
		got = append(got, fn.File+":"+fn.Name)
	}
	if want := []string{"a.go:A", "b.go:B"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %q, want %q", got, want)
	}
}

func TestSNRCountFileAcceptsLineJustUnderOneMiB(t *testing.T) {
	dir := t.TempDir()
	writeProductFixture(t, dir, "long.go", strings.Repeat("a", 1024*1024-1)+"\n")
	signal, noise, err := snrCountFile(filepath.Join(dir, "long.go"))
	if err != nil || signal != 1 || noise != 0 {
		t.Fatalf("1 MiB-1 line: signal=%d noise=%d err=%v", signal, noise, err)
	}
}

func TestProductionSourceHonorsGitIgnoreWhenRootIsSubdirectoryOfRepository(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := t.TempDir()
	sourceGit(t, repo, "init", "-q")
	writeProductFixture(t, repo, ".gitignore", "skipped.go\n")
	writeProductFixture(t, repo, "sub/kept.go", productSource)
	writeProductFixture(t, repo, "sub/skipped.go", productSource)
	assertProductCollectors(t, filepath.Join(repo, "sub"), []string{"kept.go"})
}

func TestHashCoverageUntrackedFilesHashesEveryFileAndReportsUnreadableContent(t *testing.T) {
	root := t.TempDir()
	writeProductFixture(t, root, "a.txt", "first")
	writeProductFixture(t, root, "b.txt", "second")
	var sink strings.Builder
	if err := hashCoverageUntrackedFiles(&sink, root, [][]byte{[]byte("a.txt"), []byte("b.txt")}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sink.String(), "first") || !strings.Contains(sink.String(), "second") {
		t.Errorf("both untracked files must be hashed: %q", sink.String())
	}

	unreadable := filepath.Join(root, "locked.txt")
	writeProductFixture(t, root, "locked.txt", "x")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })
	if f, err := os.Open(unreadable); err == nil {
		_ = f.Close()
		t.Skip("file permissions are not enforced for this user")
	}
	if err := hashCoverageUntrackedFiles(&sink, root, [][]byte{[]byte("locked.txt")}); err == nil {
		t.Fatal("unreadable untracked file must fail the fingerprint")
	}
}

func TestHashCoverageFileContentReportsReadFailure(t *testing.T) {
	var sink strings.Builder
	if err := hashCoverageFileContent(&sink, t.TempDir()); err == nil {
		t.Fatal("reading a directory must fail")
	}
}
