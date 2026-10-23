package rootcmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"issueops/cmd/issueops/jsonout"
)

type Runner func([]string) error

type Command struct {
	Version       string
	Usage         func()
	Stdout        io.Writer
	Stderr        io.Writer
	Runners       map[string]Runner
	ErrorExitCode func(name string, err error) int
}

func (c Command) Run(args []string) int {
	if len(args) < 1 {
		c.printUsage()
		return 2
	}

	switch args[0] {
	case "help", "--help", "-h":
		c.printUsage()
		return 0
	case "version", "--version", "-v":
		fmt.Fprintln(c.stdout(), "issueops", c.Version)
		return 0
	}
	if ok, exitCode := c.runSubcommand(args[0], args[1:]); ok {
		return exitCode
	}
	c.printUnknownCommand(args[0])
	return 2
}

// printUnknownCommand answers an unknown command with one short line and the
// closest known names instead of the full usage (17 KB), which agents would
// otherwise read in full on every typo.
func (c Command) printUnknownCommand(name string) {
	fmt.Fprintf(c.stderr(), "issueops: unknown command %q\n", name)
	if suggestions := c.suggest(name); len(suggestions) > 0 {
		fmt.Fprintf(c.stderr(), "Did you mean: %s\n", strings.Join(suggestions, ", "))
	}
	fmt.Fprintln(c.stderr(), "Run 'issueops --help' for the full usage.")
}

func (c Command) suggest(name string) []string {
	var out []string
	for known := range c.Runners {
		if strings.HasPrefix(known, name) || strings.HasPrefix(name, known) || editDistance(name, known) <= 2 {
			out = append(out, known)
		}
	}
	sort.Strings(out)
	if len(out) > 5 {
		out = out[:5]
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

func (c Command) runSubcommand(name string, args []string) (bool, int) {
	runner, ok := c.Runners[name]
	if !ok {
		return false, 0
	}
	if err := runner(args); err != nil {
		// A help request is answered by the usage the subcommand already
		// printed; it is not a failure and must not add a "flag: help
		// requested" line or a non-zero exit.
		if errors.Is(err, flag.ErrHelp) {
			return true, 0
		}
		if !jsonout.IsReported(err) {
			fmt.Fprintln(c.stderr(), name+":", err)
		}
		return true, c.errorExitCode(name, err)
	}
	return true, 0
}

// HelpRequested reports whether args ask for usage: a leading "help" or a help
// flag before any "--" passthrough separator (argv after "--" belongs to
// another program).
func HelpRequested(args []string) bool {
	if len(args) > 0 && args[0] == "help" {
		return true
	}
	for _, arg := range args {
		switch arg {
		case "--":
			return false
		case "-h", "-help", "--help":
			return true
		}
	}
	return false
}

func (c Command) printUsage() {
	if c.Usage == nil {
		return
	}
	c.Usage()
}

func (c Command) stdout() io.Writer {
	if c.Stdout != nil {
		return c.Stdout
	}
	return io.Discard
}

func (c Command) stderr() io.Writer {
	if c.Stderr != nil {
		return c.Stderr
	}
	return io.Discard
}

func (c Command) errorExitCode(name string, err error) int {
	if c.ErrorExitCode == nil {
		return 1
	}
	return c.ErrorExitCode(name, err)
}
