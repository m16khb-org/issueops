package rootcmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"issueops/cmd/issueops/cliflag"
	"issueops/cmd/issueops/jsonout"
	clicontract "issueops/internal/contract/cli"
	cliparse "issueops/internal/domain/cli"
)

type Runner func([]string) error

type Command struct {
	Version string
	Usage   func()
	// Index prints the command list without flags; nil falls back to Usage.
	Index         func()
	Stdout        io.Writer
	Stderr        io.Writer
	Runners       map[string]Runner
	ErrorExitCode func(name string, err error) int
}

func (c Command) Run(args []string) int {
	if len(args) < 1 {
		c.printIndex()
		return 2
	}

	switch args[0] {
	case "help", "--help", "-h":
		if slices.Contains(args[1:], "--all") {
			c.printUsage()
		} else {
			c.printIndex()
		}
		return 0
	case "version", "--version", "-v":
		fmt.Fprintln(c.stdout(), "issueops", c.Version)
		return 0
	}
	if ok, exitCode := c.runSubcommand(args[0], args[1:]); ok {
		return exitCode
	}
	if JSONRequested(args[1:]) {
		// QA (codex): `issueops project-docs-route --json` left stdout empty;
		// under --json the refusal is the one JSON document there too.
		message := fmt.Sprintf("unknown command %q", args[0])
		if suggestions := c.suggest(args[0]); len(suggestions) > 0 {
			message += "; did you mean: " + strings.Join(suggestions, ", ")
		}
		_ = jsonout.PrintTo(c.stdout(), map[string]any{"ok": false, "error": message + " (issueops --help lists commands)"})
		return 2
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
	fmt.Fprintln(c.stderr(), "Run 'issueops --help' to list commands.")
}

// suggest returns up to three known commands closest to name.
func (c Command) suggest(name string) []string {
	// QA: `issueops issueops-abandon` was the skill name typed as a command;
	// the word after the prefix names the command (cleanup abandon).
	if rest, ok := strings.CutPrefix(name, "issueops-"); ok && rest != "" {
		name = rest
	}
	known := make([]string, 0, len(c.Runners))
	for command := range c.Runners {
		known = append(known, command)
	}
	suggestions := cliflag.Suggest(name, known, 3)
	// QA: `issueops review` (the skill is issueops-review) suggested only
	// review-metrics; the commands that record reviews hold the word as a
	// part of their path (devils-advocate review, implementation-review record).
	if paths := commandPathsContainingWord(name); len(paths) > 0 {
		suggestions = append(paths, slices.DeleteFunc(suggestions, func(s string) bool { return slices.Contains(paths, s) })...)
	}
	// QA: `project-docs` and `project-docs-route` were the MCP tool names
	// (project_docs_*) spelled as one command; the group is `project`.
	if prefix, rest, ok := strings.Cut(name, "-"); ok && rest != "" && c.Runners[prefix] != nil {
		suggestions = append([]string{prefix + " <subcommand> (issueops " + prefix + " --help lists them)"}, slices.DeleteFunc(suggestions, func(s string) bool { return s == prefix })...)
	}
	return suggestions
}

func (c Command) runSubcommand(name string, args []string) (bool, int) {
	runner, ok := c.Runners[name]
	if !ok {
		return false, 0
	}
	jsonout.ResetStdoutDocument()
	if err := runner(args); err != nil {
		// A help request is answered by the usage the subcommand already
		// printed; it is not a failure and must not add a "flag: help
		// requested" line or a non-zero exit.
		if errors.Is(err, flag.ErrHelp) {
			return true, 0
		}
		if !jsonout.IsReported(err) {
			// Under --json the failure is the one JSON document on stdout, so
			// `--json 2>&1 | jq` parses it; otherwise a plain stderr line. A
			// command that already printed its JSON result keeps stdout to
			// that one document and the error goes to stderr.
			if JSONRequested(args) && !jsonout.WroteStdoutDocument() {
				message := err.Error()
				if !strings.HasPrefix(message, name+" ") && !strings.HasPrefix(message, name+":") {
					message = name + ": " + message
				}
				_ = jsonout.PrintTo(c.stdout(), map[string]any{"ok": false, "error": message})
			} else {
				fmt.Fprintln(c.stderr(), name+":", err)
			}
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

// JSONRequested reports whether args ask for JSON output: a --json flag (or
// --json=true) before any "--" passthrough separator.
func JSONRequested(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--":
			return false
		case "--json", "-json", "--json=true", "-json=true":
			return true
		}
	}
	return false
}

func (c Command) printIndex() {
	if c.Index == nil {
		c.printUsage()
		return
	}
	c.Index()
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

// commandPathsContainingWord lists catalog command paths that hold word as one
// of their space- or hyphen-separated parts, up to six, in catalog order.
func commandPathsContainingWord(word string) []string {
	if len(word) < 3 || strings.Contains(word, " ") {
		return nil
	}
	if strings.Contains(word, "-") {
		// QA: `issueops sync-pr` (the subcommand without its group) got no
		// suggestion; name the group path that ends with it.
		return commandPathsEndingWith(word)
	}
	var paths []string
	for line := range strings.SplitSeq(clicontract.IssueOpsUsageCatalog, "\n") {
		path := cliparse.IssueOpsUsageKey(strings.TrimSpace(line))
		if path == "" || path == word || slices.Contains(paths, path) {
			continue
		}
		if slices.Contains(strings.FieldsFunc(path, func(r rune) bool { return r == ' ' || r == '-' }), word) {
			paths = append(paths, path)
			if len(paths) == 6 {
				break
			}
		}
	}
	return paths
}

// commandPathsEndingWith lists catalog paths whose last part is name, such as
// "remote sync-pr" for sync-pr.
func commandPathsEndingWith(name string) []string {
	var paths []string
	for line := range strings.SplitSeq(clicontract.IssueOpsUsageCatalog, "\n") {
		path := cliparse.IssueOpsUsageKey(strings.TrimSpace(line))
		if strings.HasSuffix(path, " "+name) && !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	return paths
}
