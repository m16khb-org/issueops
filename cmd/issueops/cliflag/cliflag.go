// Package cliflag parses a command's flags the way every issueops command
// reports them: a parse error comes back as one error (the root prints one
// line, or one JSON document under --json) without Go's flag-list dump, and
// --help prints the flag list on stdout. It is a leaf package.
package cliflag

import (
	"errors"
	"flag"
	"io"
	"os"
)

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
