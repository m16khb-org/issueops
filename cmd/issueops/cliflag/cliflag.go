// Package cliflag parses a command's flags the way every issueops command
// reports them: a parse error comes back as one error (the root prints one
// line, or one JSON document under --json) without Go's flag-list dump, and
// --help prints the flag list on stdout. An unknown subcommand is reported the
// same way. It is a leaf package.
package cliflag

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"issueops/internal/domain/shelltoken"
)

// UnknownSubcommand is the one-line error for a group that got a subcommand it
// does not have. It names the valid ones instead of printing the usage block,
// which would land on stderr next to a --json error document.
func UnknownSubcommand(group, got string, want ...string) error {
	return fmt.Errorf("unknown %s subcommand %q (want %s)", group, got, strings.Join(want, "|"))
}

// Parse parses args into fs. On a parse error nothing is printed; on -h or
// --help the flag list goes to stdout and flag.ErrHelp is returned. A value
// flag followed directly by another of the command's flags is an error rather
// than that flag's value: `--id $ID --json` with an empty $ID would otherwise
// look up a cycle named "--json" and drop --json.
func Parse(fs *flag.FlagSet, args []string) error {
	fs.SetOutput(io.Discard)
	args = expandActorFlagsArgument(fs, args)
	if err := swallowedFlag(fs, args); err != nil {
		return err
	}
	err := fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		fs.SetOutput(os.Stdout)
		fs.Usage()
	}
	if name, undefined := strings.CutPrefix(fmt.Sprint(err), "flag provided but not defined: -"); undefined {
		// zsh does not word-split an unquoted $VAR, so `$RECORD_ACTOR_FLAGS`
		// arrives as one argument holding several flags.
		if strings.ContainsAny(name, " \t") {
			return fmt.Errorf("one argument holds several flags (%q): the shell passed a flags variable unsplit; %s", "--"+name, quotedFlagsRemedy)
		}
		var known []string
		fs.VisitAll(func(f *flag.Flag) { known = append(known, f.Name) })
		if closest := Suggest(name, known, 1); len(closest) == 1 {
			return fmt.Errorf("%w (did you mean --%s?)", err, closest[0])
		}
	}
	if err == nil {
		return literalQuotes(fs)
	}
	return err
}

// expandActorFlagsArgument splits an argument that holds several actor flags,
// the way bash would have split the unquoted variable. zsh passes
// `$RECORD_ACTOR_FLAGS` as one argument; every QA agent's first record
// command failed on that. Only an argument in flag position whose words are
// all identity flags of this command, each with a value, is expanded, so a
// flag value that merely contains "--host" is never touched.
func expandActorFlagsArgument(fs *flag.FlagSet, args []string) []string {
	out := make([]string, 0, len(args))
	expectValue := false
	for i, arg := range args {
		if arg == "--" {
			return append(out, args[i:]...)
		}
		if !expectValue && strings.HasPrefix(arg, "--") && strings.ContainsAny(arg, " \t") {
			if words, ok := identityFlagWords(fs, arg); ok {
				out = append(out, words...)
				continue
			}
		}
		out = append(out, arg)
		if expectValue {
			expectValue = false
			continue
		}
		if name := strings.TrimLeft(arg, "-"); name != arg && !strings.Contains(name, "=") {
			if f := fs.Lookup(name); f != nil {
				boolFlag, isBool := f.Value.(interface{ IsBoolFlag() bool })
				expectValue = !isBool || !boolFlag.IsBoolFlag()
			}
		}
	}
	return out
}

func identityFlagWords(fs *flag.FlagSet, arg string) ([]string, bool) {
	words := shelltoken.SplitCommandTokens(arg)
	if len(words) == 0 || len(words)%2 != 0 {
		return nil, false
	}
	for i := 0; i < len(words); i += 2 {
		name, ok := strings.CutPrefix(words[i], "--")
		if !ok || !slices.Contains(identityFlags, name) || fs.Lookup(name) == nil || strings.HasPrefix(words[i+1], "-") {
			return nil, false
		}
	}
	return words, true
}

// quotedFlagsRemedy says how to pass the shell-quoted actor flag text whoami
// returns. zsh's ${=VAR} splits on blanks but keeps the quote characters, so
// '--session-id' arrives as "'abc'" (QA: a Codex session lost a receipt that
// way after following the old hint).
const quotedFlagsRemedy = "the flags text is shell-quoted, so paste it into the command itself or run the command through eval (eval \"issueops ... $VAR\"); zsh ${=VAR} keeps the quote characters"

// identityFlags never hold a literal quote; one wrapped in quotes means the
// shell split a quoted flags string without unquoting it.
var identityFlags = []string{"host", "session-id", "agent-id", "session-pid", "session-started-at", "session-executable", "cwd"}

func literalQuotes(fs *flag.FlagSet) error {
	for _, name := range identityFlags {
		f := fs.Lookup(name)
		if f == nil {
			continue
		}
		value := f.Value.String()
		if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') && value[len(value)-1] == value[0] {
			return fmt.Errorf("--%s value %s still carries its quote characters: %s", name, value, quotedFlagsRemedy)
		}
	}
	return nil
}

// Suggest returns up to limit known names closest to name: prefix matches
// first, then by edit distance (at most 2), ties by name.
func Suggest(name string, known []string, limit int) []string {
	type candidate struct {
		name string
		rank int
	}
	var found []candidate
	for _, candidateName := range known {
		rank := editDistance(name, candidateName)
		if strings.HasPrefix(candidateName, name) || strings.HasPrefix(name, candidateName) {
			rank = 0
		}
		if rank <= 2 {
			found = append(found, candidate{candidateName, rank})
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].rank != found[j].rank {
			return found[i].rank < found[j].rank
		}
		return found[i].name < found[j].name
	})
	out := make([]string, 0, min(len(found), limit))
	for _, candidate := range found[:min(len(found), limit)] {
		out = append(out, candidate.name)
	}
	return out
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// swallowedFlag walks args the way flag.Parse does (up to "--" or the first
// positional) and reports a value flag whose value is one of fs's own flags.
// The loop skips the value token itself, so it keeps an explicit index.
func swallowedFlag(fs *flag.FlagSet, args []string) error {
	for i := 0; i < len(args); i++ {
		name, hasValue, ok := flagToken(args[i])
		if !ok {
			return nil
		}
		defined := fs.Lookup(name)
		if hasValue || defined == nil || isBoolFlag(defined) || i+1 >= len(args) {
			continue
		}
		i++
		if next, _, isFlag := flagToken(args[i]); isFlag && fs.Lookup(next) != nil {
			return fmt.Errorf("flag -%s needs a value but is followed by flag %s (is the variable meant for -%s empty?)", name, args[i], name)
		}
	}
	return nil
}

// flagToken parses "-name", "--name", or "-name=value" the way the flag
// package does; ok is false for "--", "-", and positionals.
func flagToken(arg string) (name string, hasValue, ok bool) {
	if len(arg) < 2 || arg[0] != '-' || arg == "--" {
		return "", false, false
	}
	name = strings.TrimPrefix(arg[1:], "-")
	if name == "" || name[0] == '-' || name[0] == '=' {
		return "", false, false
	}
	name, _, hasValue = strings.Cut(name, "=")
	return name, hasValue, true
}

func isBoolFlag(f *flag.Flag) bool {
	boolFlag, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && boolFlag.IsBoolFlag()
}

// ParseNoArgs is Parse for a command that takes flags only. Go's flag parser
// stops at the first positional argument, so a stray one (`next io-1 --json`)
// would otherwise silently drop the flags after it; it is an error instead.
func ParseNoArgs(fs *flag.FlagSet, args []string) error {
	if err := Parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected positional argument %q (this command takes flags only; flags after it were not read)", fs.Arg(0))
	}
	return nil
}
