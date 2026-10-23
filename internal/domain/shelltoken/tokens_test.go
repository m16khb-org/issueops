package shelltoken

import "testing"

func TestSplitCommandTokens(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{"empty", "", nil},
		{"single word", "hello", []string{"hello"}},
		{"two words", "hello world", []string{"hello", "world"}},
		{"double quoted", `echo "hello world"`, []string{"echo", "hello world"}},
		{"single quoted", "echo 'hello world'", []string{"echo", "hello world"}},
		{"mixed quotes", `'it' "works"`, []string{"it", "works"}},
		{"escaped newline", `"line1\nline2"`, []string{"line1\nline2"}},
		{"escaped tab", `"col1\tcol2"`, []string{"col1\tcol2"}},
		{"escaped return", `"a\rb"`, []string{"a\rb"}},
		{"literal backslash n", `"\\n"`, []string{"\\n"}},
		{"tabs between", "one\ttwo", []string{"one", "two"}},
		{"newlines between", "one\ntwo", []string{"one", "two"}},
		{"multiple spaces", "a   b", []string{"a", "b"}},
		{"gh issue with flags", `gh issue create --title "버그 수정" --body "내용"`, []string{"gh", "issue", "create", "--title", "버그 수정", "--body", "내용"}},
		{"flag with equals", `--labels="bug,feature"`, []string{"--labels=bug,feature"}},
		{"unclosed quote", `"unclosed`, []string{"unclosed"}},
		{"empty single quoted value", `--agent-id ''`, []string{"--agent-id", ""}},
		{"empty double quoted value", `--agent-id ""`, []string{"--agent-id", ""}},
		{"empty quoted joins adjacent word", `a''b`, []string{"ab"}},
		{"glab command", `glab mr create --title "fix"`, []string{"glab", "mr", "create", "--title", "fix"}},
		{"git worktree", `git worktree add -b feature ../path`, []string{"git", "worktree", "add", "-b", "feature", "../path"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitCommandTokens(tt.command)
			if !stringSlicesEqual(got, tt.want) {
				t.Errorf("SplitCommandTokens(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestHasUnquotedControlOperator(t *testing.T) {
	for _, command := range []string{
		`issueops finish --verification 'exact; clean & verified | complete'`,
		`issueops finish --verification "exact; clean & verified | complete"`,
	} {
		if HasUnquotedControlOperator(command) {
			t.Fatalf("quoted evidence punctuation must be data: %q", command)
		}
	}
	for _, command := range []string{
		"issueops finish; touch x",
		"issueops finish & touch x",
		"issueops finish | touch x",
		"issueops finish\ntouch x",
		"issueops finish\rtouch x",
	} {
		if !HasUnquotedControlOperator(command) {
			t.Fatalf("unquoted control operator must be rejected: %q", command)
		}
	}
}

func TestSplitCommandTokensCanonicalizesUnquotedBackslashEscapes(t *testing.T) {
	got := SplitCommandTokens(`\g\i\t push origin HEAD --message evidence\ value`)
	want := []string{"git", "push", "origin", "HEAD", "--message", "evidence value"}
	if !stringSlicesEqual(got, want) {
		t.Fatalf("escaped shell argv = %#v, want %#v", got, want)
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestQuoteWord(t *testing.T) {
	if got := QuoteWord(""); got != "''" {
		t.Fatalf("empty QuoteWord = %q", got)
	}
	if got := QuoteWord("it's/bin"); got != `'it'"'"'s/bin'` {
		t.Fatalf("quoted QuoteWord = %q", got)
	}
}

func TestSplitCommandTokensKeepsTrailingBackslash(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{"trailing backslash in double quote", `"a\`, []string{`a\`}},
		{"trailing backslash unquoted", `a\`, []string{`a\`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitCommandTokens(tt.command); !stringSlicesEqual(got, tt.want) {
				t.Fatalf("SplitCommandTokens(%q) = %#v, want %#v", tt.command, got, tt.want)
			}
		})
	}
}

func TestHasUnquotedControlOperatorSingleQuotedBackslash(t *testing.T) {
	if !HasUnquotedControlOperator(`echo '\'; ls`) {
		t.Fatal("backslash is literal in single quotes, so the following semicolon is active")
	}
	if HasUnquotedControlOperator(`echo "\"; ls"`) {
		t.Fatal("escaped double quote must keep the semicolon quoted")
	}
}

// Expansions a shell would perform are reported wherever a shell expands them
// (bare or double-quoted); single quotes, escapes, and a `$` that starts no
// expansion stay literal on purpose.
func TestHasShellExpansion(t *testing.T) {
	for command, want := range map[string]bool{
		`test "$(./hello.sh)" = hello`:     true,
		`echo ${HOME}`:                     true,
		`printf "%s" "$HOME"`:              true,
		`python3 -c 'print("$(x) $HOME")'`: false,
		`grep -q "foo$" file`:              false,
		`echo \$HOME`:                      false,
		`go test -run 'TestX$' ./...`:      false,
		`echo cost 5$`:                     false,
	} {
		if got := HasShellExpansion(command); got != want {
			t.Errorf("HasShellExpansion(%q) = %v, want %v", command, got, want)
		}
	}
}
