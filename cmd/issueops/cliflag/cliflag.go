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
	return err
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
