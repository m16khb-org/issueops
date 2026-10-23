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
// --help the flag list goes to stdout and flag.ErrHelp is returned.
func Parse(fs *flag.FlagSet, args []string) error {
	fs.SetOutput(io.Discard)
	err := fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		fs.SetOutput(os.Stdout)
		fs.Usage()
	}
	return err
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
