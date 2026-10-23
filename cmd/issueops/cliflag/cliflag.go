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
	"sort"
	"strings"
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
	if err := swallowedFlag(fs, args); err != nil {
		return err
	}
	err := fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		fs.SetOutput(os.Stdout)
		fs.Usage()
	}
	if name, undefined := strings.CutPrefix(fmt.Sprint(err), "flag provided but not defined: -"); undefined {
		var known []string
		fs.VisitAll(func(f *flag.Flag) { known = append(known, f.Name) })
		if closest := Suggest(name, known, 1); len(closest) == 1 {
			return fmt.Errorf("%w (did you mean --%s?)", err, closest[0])
		}
	}
	return err
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
