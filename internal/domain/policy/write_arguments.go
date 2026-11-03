package policy

import (
	"slices"
	"strings"
)

// writeCapableArguments reports argv that uses a write- or exec-capable form of
// a command the read-only tier otherwise allows: `find -delete`, `rg --pre`,
// `git branch -D`, `go env -w`. Classify keys on the command and git/go
// subcommand only, so without this a "read-only" run could delete files, edit
// refs or config, or run a program named on the command line. Pure over argv:
// it is applied to the builtin bases regardless of workspace overrides.
func writeCapableArguments(base string, argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	rest := argv[1:]
	switch base {
	case "find":
		return hasExactArgument(rest, "-delete", "-exec", "-execdir", "-ok", "-okdir", "-fprint", "-fprint0", "-fprintf", "-fls")
	case "rg":
		// --pre and --hostname-bin run the program named by the flag value.
		for _, arg := range rest {
			if name, ok := longFlagName(arg); ok && (name == "--pre" || name == "--hostname-bin") {
				return true
			}
		}
		return false
	case "git":
		return gitArgumentsWrite(strings.ToLower(rest[0]), rest[1:])
	case "go":
		return goArgumentsWrite(strings.ToLower(rest[0]), rest[1:])
	}
	return false
}

func gitArgumentsWrite(subcommand string, args []string) bool {
	switch subcommand {
	case "branch":
		return gitBranchWrites(args)
	case "config":
		return gitConfigWrites(args)
	case "remote":
		// `git remote -v add x y` still adds: the subcommand is the first
		// operand, not necessarily args[0].
		for _, arg := range args {
			if arg == "--" || !strings.HasPrefix(arg, "-") {
				switch strings.ToLower(arg) {
				case "add", "rename", "mv", "remove", "rm", "set-head", "set-branches", "set-url", "prune", "update":
					return true
				}
				return false
			}
		}
		return false
	case "grep":
		// -O[<pager>] and --open-files-in-pager run a pager program.
		return gitHasFlag(args, []string{"--open-files-in-pager"}, "O")
	case "show", "diff", "log":
		// --ext-diff runs the configured external diff program; --output
		// writes the diff to a file.
		return gitHasFlag(args, []string{"--ext-diff", "--output"}, "")
	}
	return false
}

// gitHasFlag reports whether args, up to the `--` path separator, contain one
// of the long flags (git accepts unambiguous abbreviations, so a prefix counts)
// or a short-option cluster containing one of the shortLetters.
func gitHasFlag(args, long []string, shortLetters string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if name, ok := longFlagName(arg); ok {
			if abbreviatesAny(name, long) {
				return true
			}
			continue
		}
		if letters, ok := shortCluster(arg); ok && shortLetters != "" && strings.ContainsAny(letters, shortLetters) {
			return true
		}
	}
	return false
}

// gitBranchWrites treats `git branch` as a write unless it only lists. A bare
// operand creates a branch, so it is a write unless a list-mode flag makes the
// operand a pattern or commit (`--contains HEAD`, `-l 'feat/*'`).
func gitBranchWrites(args []string) bool {
	list, operand := false, false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operand = operand || i+1 < len(args)
			break
		}
		if name, ok := longFlagName(arg); ok {
			write, isList, takesValue := gitBranchLongFlag(name, arg)
			if write {
				return true
			}
			list = list || isList
			if takesValue {
				i++
			}
			continue
		}
		if letters, ok := shortCluster(arg); ok {
			write, isList := gitBranchShortFlags(letters)
			if write {
				return true
			}
			list = list || isList
			continue
		}
		operand = true
	}
	return operand && !list
}

// gitBranchLongFlag classifies a `git branch` long flag: write for a flag that
// modifies branches, list for a list-mode flag, and takesValue when the next
// argument is the flag's value (`--sort key`, not `--sort=key`).
func gitBranchLongFlag(name, arg string) (write, list, takesValue bool) {
	writeLong := []string{
		"--delete", "--move", "--copy", "--set-upstream-to", "--unset-upstream", "--edit-description",
		"--force", "--track", "--no-track", "--create-reflog", "--recurse-submodules",
	}
	listLong := map[string]bool{
		"--list": true, "--all": true, "--remotes": true, "--contains": true, "--no-contains": true,
		"--merged": true, "--no-merged": true, "--points-at": true, "--show-current": true,
	}
	if abbreviatesAny(name, writeLong) {
		return true, false, false
	}
	takesValue = (name == "--sort" || name == "--format") && !strings.Contains(arg, "=")
	return false, listLong[name], takesValue
}

// gitBranchShortFlags classifies a `git branch` short-option cluster.
func gitBranchShortFlags(letters string) (write, list bool) {
	return strings.ContainsAny(letters, "dDmMcCuft"), strings.ContainsAny(letters, "lar")
}

// gitConfigWrites treats `git config` as read-only only for an explicit read
// mode or the bare `git config <key>` lookup; anything else may set a value.
// Legacy `git config` stops option parsing at the first operand, so a read flag
// after the key is a value (`git config k v --get` sets k), and `-f`/`--file`
// take the next argument as a file name (`git config -f --get k v` sets k).
func gitConfigWrites(args []string) bool {
	writeLong := []string{"--add", "--unset", "--unset-all", "--replace-all", "--rename-section", "--remove-section", "--edit"}
	subcommand := ""
	if len(args) > 0 {
		subcommand = strings.ToLower(args[0])
	}
	switch subcommand {
	case "set", "unset", "rename-section", "remove-section", "edit":
		return true
	}
	if gitHasFlag(args, writeLong, "e") {
		return true
	}
	switch subcommand {
	case "get", "list":
		return false
	}
	read, operands := scanGitConfigArgs(args)
	return !read && operands != 1
}

// scanGitConfigArgs reports whether args select a read mode and how many
// operands follow the options (0 when the scan ends without reaching one).
func scanGitConfigArgs(args []string) (read bool, operands int) {
	readLong := map[string]bool{
		"--get": true, "--get-all": true, "--get-regexp": true, "--get-urlmatch": true,
		"--get-color": true, "--get-colorbool": true, "--list": true,
	}
	valueLong := map[string]bool{"--file": true, "--blob": true, "--type": true, "--default": true, "--comment": true}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return read, len(args) - i - 1
		}
		if name, ok := longFlagName(arg); ok {
			read = read || readLong[name]
			if valueLong[name] && !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		if letters, ok := shortCluster(arg); ok {
			read = read || strings.Contains(letters, "l")
			if strings.HasSuffix(letters, "f") {
				i++
			}
			continue
		}
		// The first operand ends option parsing: every later argument is an
		// operand, flag-shaped or not.
		return read, len(args) - i
	}
	return read, 0
}

func goArgumentsWrite(subcommand string, args []string) bool {
	flags, ok := goWriteFlags(subcommand)
	if !ok {
		return false
	}
	for i, arg := range args {
		if arg == "--" {
			return false
		}
		name, ok := goFlagName(arg)
		if !ok {
			continue
		}
		if flags[name] {
			return true
		}
		if subcommand == "list" && name == "mod" && goListModWrites(arg, args, i) {
			return true
		}
	}
	return false
}

// goWriteFlags returns the flags that make a go subcommand write, and false
// for subcommands that never do.
func goWriteFlags(subcommand string) (map[string]bool, bool) {
	switch subcommand {
	case "env":
		return map[string]bool{"w": true, "u": true}, true
	case "list":
		// -export and -compiled build packages like go build; -mod=mod lets the
		// go command rewrite go.mod and go.sum.
		return map[string]bool{"toolexec": true, "exec": true, "export": true, "compiled": true}, true
	default:
		return nil, false
	}
}

// goListModWrites reports whether the -mod flag at args[i] selects anything
// other than readonly or vendor.
func goListModWrites(arg string, args []string, i int) bool {
	value, hasValue := strings.CutPrefix(strings.TrimLeft(arg, "-"), "mod=")
	if !hasValue && i+1 < len(args) {
		value = args[i+1]
	}
	return value != "readonly" && value != "vendor"
}

// goFlagName returns the flag name of -name, --name, -name=value or
// --name=value, as the Go flag package parses them.
func goFlagName(arg string) (string, bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
	name, _, _ = strings.Cut(name, "=")
	return name, name != ""
}

func hasExactArgument(args []string, wanted ...string) bool {
	for _, arg := range args {
		if slices.Contains(wanted, arg) {
			return true
		}
	}
	return false
}

// longFlagName returns the `--name` part of `--name` or `--name=value`.
func longFlagName(arg string) (string, bool) {
	if !strings.HasPrefix(arg, "--") || len(arg) == 2 {
		return "", false
	}
	name, _, _ := strings.Cut(arg, "=")
	return name, true
}

// shortCluster returns the letters of a `-abc` option cluster.
func shortCluster(arg string) (string, bool) {
	if len(arg) < 2 || arg[0] != '-' || arg[1] == '-' {
		return "", false
	}
	return arg[1:], true
}

// abbreviatesAny reports whether name is one of the full long flags or a
// prefix of one longer than `--`: git resolves unambiguous prefixes, so
// `--del` is `--delete`. Ambiguous prefixes are treated as writes (fail closed).
func abbreviatesAny(name string, full []string) bool {
	if len(name) <= 2 {
		return false
	}
	for _, candidate := range full {
		if strings.HasPrefix(candidate, name) {
			return true
		}
	}
	return false
}
