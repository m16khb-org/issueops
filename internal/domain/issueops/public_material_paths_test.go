package issueops

import (
	"strings"
	"testing"
)

func TestPublicMaterialPaths(t *testing.T) {
	const source = "/Users/synthetic/work/repo"
	const worktree = source + ".worktrees/527"
	for _, tc := range []struct{ name, input, want string }{
		{"root line then URL", source + ":12 https://example.test/?cwd=" + source + "/file", "$SOURCE_ROOT:12 https://example.test/?cwd=" + source + "/file"},
		{"roots", source + "/main.go:12 " + worktree + "/test.go", "$SOURCE_ROOT/main.go:12 $WORKTREE/test.go"},
		{"home", "/Users/synthetic/.codex /Users/other/.codex", "$HOME/.codex /Users/other/.codex"},
		{"markdown", "`" + source + "`\n```sh\ncd '" + worktree + "'\n```\n[plan](" + source + "/plan.md)", "`$SOURCE_ROOT`\n```sh\ncd '$WORKTREE'\n```\n[plan]($SOURCE_ROOT/plan.md)"},
		{"argv", "--cwd=\"" + worktree + "\"", "--cwd=\"$WORKTREE\""},
		{"prefixes", source + "-old " + source + ".bak " + worktree + "0 prefix" + source + " /Users/synthetic2/a", "$HOME/work/repo-old $HOME/work/repo.bak $HOME/work/repo.worktrees/5270 prefix" + source + " /Users/synthetic2/a"},
		{"URL apostrophe", "https://example.test/?cwd='" + source + "/file", "https://example.test/?cwd='" + source + "/file"},
		{"compact JSON path then URL", `{"file":"` + source + `:12","url":"https://example.test/?cwd=` + source + `/file"}`, `{"file":"$SOURCE_ROOT:12","url":"https://example.test/?cwd=` + source + `/file"}`},
		{"compact JSON URL then path", `{"url":"https://example.test/","file":"` + source + `/a"}`, `{"url":"https://example.test/","file":"$SOURCE_ROOT/a"}`},
		{"Markdown reference URL", "[ref]:https://example.test/?cwd=" + source + "/file", "[ref]:https://example.test/?cwd=" + source + "/file"},
		{"urls", "https://host.test" + source + "/a?cwd=" + worktree + " https://host.test/?cwd=" + source + "#" + worktree + " file://" + source + "/a //host.test" + worktree + "/a", "https://host.test" + source + "/a?cwd=" + worktree + " https://host.test/?cwd=" + source + "#" + worktree + " file://" + source + "/a //host.test" + worktree + "/a"},
		{"plain", "$SOURCE_ROOT/a $WORKTREE/b $HOME/c ./relative ../relative ordinary body /tmp/unrelated", "$SOURCE_ROOT/a $WORKTREE/b $HOME/c ./relative ../relative ordinary body /tmp/unrelated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := string(NormalizePublicMaterialPaths([]byte(tc.input), source, worktree))
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if again := string(NormalizePublicMaterialPaths([]byte(got), source, worktree)); again != got {
				t.Fatalf("not idempotent: %q", again)
			}
		})
	}
}

func TestPublicMaterialPathsRoots(t *testing.T) {
	for _, tc := range []struct{ name, source, worktree, input, want string }{
		{"nested", "/home/synthetic/repo", "/home/synthetic/repo/worktree", "/home/synthetic/repo/worktree/a /home/synthetic/repo/b /home/synthetic/.config", "$WORKTREE/a $SOURCE_ROOT/b $HOME/.config"},
		{"reverse nested", "/home/synthetic/repo/source", "/home/synthetic/repo", "/home/synthetic/repo/source/a /home/synthetic/repo/b", "$SOURCE_ROOT/a $WORKTREE/b"},
		{"spaces", "/Users/synthetic/My Repo", "/Users/synthetic/My Repo.worktrees/527", "'/Users/synthetic/My Repo/a.go' [x](</Users/synthetic/My Repo.worktrees/527/a.go>)", "'$SOURCE_ROOT/a.go' [x](<$WORKTREE/a.go>)"},
		{"URL cursor after spaced root", "/Users/synthetic/My repo:12", "", "`/Users/synthetic/My repo:12` https://example.test/?cwd=/Users/synthetic/My%20repo", "`$SOURCE_ROOT` https://example.test/?cwd=/Users/synthetic/My%20repo"},
		{"invalid", "relative/root", "/", "/a relative/root /Users/other/x", "/a relative/root /Users/other/x"},
		{"empty", "", "", "/home/synthetic/x", "/home/synthetic/x"},
		{"two homes", "/home/one/repo", "/Users/two/worktree", "/home/one/.config /Users/two/.config", "$HOME/.config $HOME/.config"},
		{"trailing slash", "/home/synthetic/repo/", "", "/home/synthetic/repo/a", "$SOURCE_ROOT/a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(NormalizePublicMaterialPaths([]byte(tc.input), tc.source, tc.worktree)); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func BenchmarkPublicMaterialPaths(b *testing.B) {
	for _, size := range []struct {
		name    string
		repeats int
	}{{"small", 20}, {"large", 20000}} {
		b.Run(size.name, func(b *testing.B) {
			body := []byte(strings.Repeat("`/home/synthetic/repo/src.go:12` https://host.test/home/synthetic/repo/src.go\n", size.repeats))
			b.SetBytes(int64(len(body)))
			b.ReportAllocs()
			for b.Loop() {
				NormalizePublicMaterialPaths(body, "/home/synthetic/repo", "/home/synthetic/repo.worktrees/527")
			}
		})
	}
}
