package issueops

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseWorkspaceProcessesResolvesEachEligiblePathOnce(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "file")
	distinct := root + string(filepath.Separator) + "." + string(filepath.Separator) + "file"
	outside := root + "-sibling/file"
	calls := map[string]int{}
	output := fmt.Sprintf("p10\ncwriter\nfcwd\nn%s\nf1\naw\nn%s\nf2\nau\nn%s (deleted)\nf3\naw\nn%s\nn%s\nn%s\nn%s\nf4\nar\nn%s\nf5\naw\nnrelative\np0\nfcwd\nn%s\np20\nfcwd\nn%s\n",
		inside, inside, inside, distinct, outside, outside, distinct, root+"/read-only", root+"/invalid-pid", root+"/excluded")
	want := []workspaceProcess{
		{PID: 10, Command: "writer", FD: "cwd", Path: inside},
		{PID: 10, Command: "writer", FD: "1", Access: "w", Path: inside},
		{PID: 10, Command: "writer", FD: "2", Access: "u", Path: inside},
		{PID: 10, Command: "writer", FD: "3", Access: "w", Path: distinct},
		{PID: 10, Command: "writer", FD: "3", Access: "w", Path: distinct},
	}

	got, err := parseWorkspaceProcessesWithPathPredicate(output, root, map[int]bool{20: true}, func(path, root string) bool {
		calls[path]++
		return pathWithinResolved(path, root)
	})

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("processes=%+v want=%+v error=%v", got, want, err)
	}
	wantCalls := map[string]int{inside: 1, distinct: 1, outside: 1}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("resolution calls=%v want=%v", calls, wantCalls)
	}
	t.Logf("eligible rows=7 distinct path resolutions=%d", len(calls))
}

func TestParseWorkspaceProcessesPreservesResolvedBoundaries(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "workspace")
	outside := filepath.Join(base, "workspace-sibling")
	for _, dir := range []string{root, outside} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	escape := filepath.Join(root, "escape")
	entry := filepath.Join(outside, "entry")
	alias := filepath.Join(base, "alias")
	for link, target := range map[string]string{escape: outside, entry: root, alias: root} {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	inside := filepath.Join(root, "file")
	inbound := filepath.Join(entry, "file")
	deleted := filepath.Join(root, "missing")
	output := strings.Join([]string{
		"p30", "cfirst", "fcwd", "n" + root, "f1", "ar", "n" + inside,
		"f2", "aw", "n" + filepath.Join(escape, "file"), "n" + filepath.Join(outside, "file"),
		"n" + inbound, "n" + inbound, "n" + deleted + " (deleted)",
		"nrelative", "n->0x123", "ncount=2", "p31", "csecond", "f3", "au", "n" + inside,
	}, "\n")
	want := []workspaceProcess{
		{PID: 30, Command: "first", FD: "cwd", Path: root},
		{PID: 30, Command: "first", FD: "2", Access: "w", Path: inbound},
		{PID: 30, Command: "first", FD: "2", Access: "w", Path: inbound},
		{PID: 30, Command: "first", FD: "2", Access: "w", Path: deleted},
		{PID: 31, Command: "second", FD: "3", Access: "u", Path: inside},
	}

	got, err := parseWorkspaceProcesses(output, alias, nil)

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("processes=%+v want=%+v error=%v", got, want, err)
	}
}

func TestParseWorkspaceProcessesDoesNotReusePreviousContainment(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{root, outside} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	output := "p42\nfcwd\nn" + link + "\n"
	first, err := parseWorkspaceProcesses(output, root, nil)
	if err != nil || len(first) != 1 {
		t.Fatalf("first processes=%+v error=%v", first, err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	second, err := parseWorkspaceProcesses(output, root, nil)

	if err != nil || len(second) != 0 {
		t.Fatalf("second processes=%+v error=%v", second, err)
	}
}

func TestParseWorkspaceProcessesRejectsMalformedInput(t *testing.T) {
	for name, output := range map[string]string{
		"invalid pid": "pnot-a-pid\n",
		"long row":    "n" + strings.Repeat("x", 70*1024),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseWorkspaceProcesses(output, t.TempDir(), nil)
			if err == nil {
				t.Fatal("expected parser error")
			}
		})
	}
}
