package policy

import (
	"slices"
	"strings"
	"testing"
	"time"

	policycontract "issueops/internal/contract/policy"
)

// decideForTest evaluates argv in the given tier with the builtin catalog and a
// healthy workspace, so the only deny reasons are the command's own.
func decideForTest(argv []string, writeAllowed bool) (CommandClassification, CommandDecision) {
	classification := BuiltinCatalog().Classify(argv)
	decision := EvaluateCommandDecision(policycontract.CommandPolicyRequest{
		WorkspaceRoot: "/repo", CWD: "/repo", Argv: argv, Timeout: "30s", WriteAllowed: writeAllowed,
	}, CommandFacts{
		RootDirectory: true, CWDDirectory: true, CWDWithinRoot: true,
		Timeout: 30 * time.Second, TimeoutValid: true,
		ShellCommand: classification.ShellCommand, UsesNetwork: classification.UsesNetwork,
		Writes: classification.Writes, ReadOnlyAllowed: classification.ReadOnlyAllowed,
	})
	return classification, decision
}

// A read-only run must not delete files, edit refs or config, or run a program
// named on the command line, even though the command itself is on the allowlist.
func TestReadOnlyTierDeniesWriteCapableArguments(t *testing.T) {
	for _, line := range []string{
		"find . -delete",
		"find . -name x -exec rm {} ;",
		"find . -execdir rm {} ;",
		"find . -ok rm {} ;",
		"find . -okdir rm {} ;",
		"find . -fprint out.txt",
		"find . -fprint0 out.txt",
		"find . -fprintf out.txt %p",
		"find . -fls out.txt",
		"rg --pre ./hook foo",
		"rg --pre=./hook foo",
		"rg --hostname-bin ./hook foo",
		"git branch topic",
		"git branch topic HEAD~1",
		"git branch -d topic",
		"git branch -D topic",
		"git branch -vD topic",
		"git branch -ad topic",
		"git branch --delete topic",
		"git branch --del topic",
		"git branch -m old new",
		"git branch -M new",
		"git branch --move old new",
		"git branch -c old new",
		"git branch -C old new",
		"git branch --copy old new",
		"git branch -u origin/main",
		"git branch --set-upstream-to=origin/main",
		"git branch --set-upstream-to origin/main",
		"git branch --unset-upstream",
		"git branch --edit-description",
		"git branch -f topic HEAD",
		"git branch --force topic HEAD",
		"git branch --track topic origin/topic",
		"git branch --no-track topic",
		"git branch --create-reflog topic",
		"git branch --recurse-submodules topic",
		"git branch --contains HEAD -d topic",
		"git branch --sort committerdate topic",
		"git config user.name Habin",
		"git config --global user.name Habin",
		"git config --add user.name Habin",
		"git config --unset user.name",
		"git config --unset-all user.name",
		"git config --replace-all user.name Habin",
		"git config --rename-section a b",
		"git config --remove-section a",
		"git config -e",
		"git config --edit",
		"git config --get user.name --unset",
		"git config --list --edit",
		"git config set user.name Habin",
		"git config unset user.name",
		"git config rename-section a b",
		"git config remove-section a",
		"git config edit",
		"git config",
		"git config --global",
		// Legacy git config stops option parsing at the first operand, so a
		// trailing read flag is a value and the command still sets the key.
		"git config user.name Habin --get",
		"git config --file=x.cfg user.name Habin --get",
		"git config user.name Habin -l",
		"git config user.name Habin --list",
		"git config core.fsmonitor touch-marker --get-all",
		// -f takes the next argument as the file name, leaving `k v` to set.
		"git config -f --get k v",
		"git remote rename a b",
		"git remote add origin url",
		"git remote mv a b",
		"git remote remove origin",
		"git remote rm origin",
		"git remote set-head origin main",
		"git remote set-branches origin main",
		"git remote set-url origin url",
		"git remote prune origin",
		"git remote update",
		"git remote -v add origin url",
		"git grep -O foo",
		"git grep -Ovim foo",
		"git grep -nO foo",
		"git grep --open-files-in-pager foo",
		"git grep --open-files-in-pager=vim foo",
		"git show --ext-diff",
		"git diff --ext-diff",
		"git log --ext-diff",
		"git diff --output=out.patch",
		"git diff --output out.patch",
		"git show --output=out.patch HEAD",
		"git log --output=out.log",
		"go env -w GOFLAGS=-mod=mod",
		"go env -u GOFLAGS",
		"go env --w GOFLAGS=-mod=mod",
		"go env -w=true GOFLAGS=-mod=mod",
		"go list -toolexec ./hook ./...",
		"go list -toolexec=./hook ./...",
		"go list -exec ./hook ./...",
		"go list -exec=./hook ./...",
		"go list -export ./...",
		"go list -compiled ./...",
		"go list -mod=mod ./...",
		"go list -mod mod ./...",
		"go list -modfile=alt.mod -mod=mod ./...",
		"sed -i s/a/b/ file",
		"sed 1e file",
		"sed -n p file",
		"awk BEGIN{system(\"rm\")}",
		"awk {print} file",
	} {
		argv := strings.Fields(line)
		classification, decision := decideForTest(argv, false)
		if !classification.Writes {
			t.Errorf("%q: Writes=false", line)
		}
		if decision.Allowed || !slices.Contains(decision.DenyReasons, "write_not_allowed") {
			t.Errorf("%q: read-only decision=%+v", line, decision)
		}
	}
}

func TestReadOnlyTierKeepsReadForms(t *testing.T) {
	for _, line := range []string{
		"find . -name x",
		"find . -type f -print",
		"rg foo",
		"rg -n --glob *.go foo",
		"git branch",
		"git branch -v",
		"git branch -vv",
		"git branch -a",
		"git branch -r",
		"git branch --all",
		"git branch --list",
		"git branch --list feat/*",
		"git branch -l feat/*",
		"git branch --show-current",
		"git branch --contains HEAD",
		"git branch --contains=HEAD",
		"git branch --no-contains HEAD",
		"git branch --merged main",
		"git branch --no-merged main",
		"git branch --points-at HEAD",
		"git branch --sort=-committerdate",
		"git branch --sort committerdate",
		"git branch --format=%(refname)",
		"git branch -a --contains HEAD",
		"git config --get user.name",
		"git config --get-all user.name",
		"git config --get-regexp user",
		"git config --get-urlmatch http.proxy https://example.com",
		"git config --get-color color.diff.new",
		"git config --get-colorbool color.diff",
		"git config --global --get user.name",
		"git config -l",
		"git config --list",
		"git config --list --show-origin",
		"git config get user.name",
		"git config list",
		"git config user.name",
		"git config --global user.name",
		"git config --file .gitmodules submodule.x.url",
		"git config --type=bool core.bare",
		"git config --get user.name extra-value-pattern",
		"git remote",
		"git remote -v",
		"git remote show origin",
		"git remote get-url origin",
		"git grep foo",
		"git grep -n -e foo -- internal",
		"git diff",
		"git diff --stat HEAD~1",
		"git diff --no-ext-diff",
		"git diff --output-indicator-new=+",
		"git log --oneline",
		"git log -n 5 --format=%H",
		"git show HEAD",
		"git status --short",
		"go version",
		"go env GOPATH",
		"go env -json",
		"go list ./...",
		"go list -json ./...",
		"go list -mod=readonly ./...",
		"go list -mod vendor ./...",
		"cat note.txt",
	} {
		argv := strings.Fields(line)
		classification, decision := decideForTest(argv, false)
		if classification.Writes || !classification.ReadOnlyAllowed {
			t.Errorf("%q: classification=%+v", line, classification)
		}
		if !decision.Allowed {
			t.Errorf("%q: read-only decision=%+v", line, decision)
		}
	}
}

// The write tier (gates CHECK) still runs the programmable tools and the
// write forms; only the read-only tier refuses them.
func TestWriteTierStillAllowsProgrammableCommands(t *testing.T) {
	for _, line := range []string{
		"sed -n 1,5p file",
		"awk {print} file",
		"find . -name x -delete",
		"git branch topic",
		"git config user.name Habin",
	} {
		_, decision := decideForTest(strings.Fields(line), true)
		if !decision.Allowed {
			t.Errorf("%q: write-tier decision=%+v", line, decision)
		}
	}
}

// sed and awk are programmable like perl: the read-only tier refuses them
// outright instead of only on their write-capable arguments.
func TestSedAndAwkAreNotReadOnlyAllowed(t *testing.T) {
	for _, command := range []string{"sed", "awk"} {
		classification := BuiltinCatalog().Classify([]string{command, "p"})
		if classification.ReadOnlyAllowed || !classification.Writes {
			t.Errorf("%s classification=%+v", command, classification)
		}
	}
}

// A workspace override can still list a command as read-only; the argument
// check applies to the builtin bases regardless.
func TestWorkspaceOverridesDoNotUnlockWriteArguments(t *testing.T) {
	catalog := BuiltinCatalog()
	catalog.Apply(policycontract.Overrides{
		AdditionalReadOnlyCommands:    []string{"repo-tool"},
		AdditionalReadOnlySubcommands: map[string][]string{"git": {"blame"}},
	})
	if got := catalog.Classify([]string{"repo-tool", "-delete"}); !got.ReadOnlyAllowed || got.Writes {
		t.Fatalf("override command classification=%+v", got)
	}
	if got := catalog.Classify([]string{"git", "blame", "file"}); !got.ReadOnlyAllowed || got.Writes {
		t.Fatalf("override subcommand classification=%+v", got)
	}
	if got := catalog.Classify([]string{"find", ".", "-delete"}); !got.Writes {
		t.Fatalf("find -delete classification=%+v", got)
	}
}

func TestGitConfigWritesTable(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{"get", "--unset"}, true},
		{[]string{"GET", "user.name"}, false},
		{[]string{"List"}, false},
		{[]string{"SET", "k", "v"}, true},
		{[]string{"user.name"}, false},
		{[]string{"user.name", "v"}, true},
		{[]string{"--get", "user.name"}, false},
		{[]string{"--get-regexp", "user\\..*"}, false},
		{[]string{"--get-urlmatch", "http", "https://x"}, false},
		{[]string{"--get-color", "color.ui"}, false},
		{[]string{"--get-colorbool", "color.ui"}, false},
		{[]string{"--list"}, false},
		{[]string{"-l"}, false},
		{[]string{"-lz"}, false},
		{[]string{"--file=x.cfg", "--get", "k"}, false},
		{[]string{"--file", "x.cfg", "--get", "k"}, false},
		{[]string{"--blob", "HEAD:x", "k"}, false},
		{[]string{"--type", "int", "k"}, false},
		{[]string{"--type=int", "k"}, false},
		{[]string{"--default", "d", "--get", "k"}, false},
		{[]string{"--comment", "c", "k"}, false},
		{[]string{"--comment", "c", "k", "v"}, true},
		{[]string{"-f", "x.cfg", "k"}, false},
		{[]string{"-f", "--get", "k", "v"}, true},
		{[]string{"-lf", "x.cfg"}, false},
		{[]string{"-fl", "k"}, false},
		{[]string{"-f"}, true},
		{[]string{"--file"}, true},
		{[]string{"--"}, true},
		{[]string{"--", "k"}, false},
		{[]string{"--", "k", "v"}, true},
		{[]string{"--get", "--", "k", "v"}, false},
		{[]string{"-x", "k"}, false},
		{[]string{"k", "--get"}, true},
		{[]string{"k", "--", "v"}, true},
		{[]string{"-"}, false},
		{[]string{"--=x", "k"}, false},
		{[]string{"--ed"}, true},
		{[]string{"-e"}, true},
		{[]string{"-ne"}, true},
		{[]string{"--get", "--ed"}, true},
		{[]string{"--list", "--", "--edit"}, false},
	} {
		if got := gitConfigWrites(tc.args); got != tc.want {
			t.Errorf("gitConfigWrites(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}
