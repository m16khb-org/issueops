// Package policy holds the pure command-policy rules: the command catalog and its
// classification, the allow/deny decision over observed facts, the capability tier,
// and secret redaction. It does no filesystem or process access; adapters observe
// the facts and this domain layer decides.
package policy

import (
	"path/filepath"
	"sort"
	"strings"

	policycontract "issueops/internal/contract/policy"
)

// Catalog is the set of shell interpreters, network commands, write commands and
// read-only commands (with per-command subcommand sets) that Classify consults.
// Build one with BuiltinCatalog and extend it with Apply.
type Catalog struct {
	shellInterpreters   map[string]bool
	networkCommands     map[string]bool
	networkSubcommands  map[string]map[string]bool
	writeCommands       map[string]bool
	writeSubcommands    map[string]map[string]bool
	readOnlyCommands    map[string]bool
	readOnlySubcommands map[string]map[string]bool
}

// CommandClassification is what Classify concluded about one argv: whether it
// runs a shell interpreter, uses the network, writes, or is on the read-only allowlist.
type CommandClassification struct {
	ShellCommand    bool
	UsesNetwork     bool
	Writes          bool
	ReadOnlyAllowed bool
}

// CatalogSnapshot is a deterministic, sorted copy of a Catalog for reporting.
// It shares no maps or slices with the Catalog it came from.
type CatalogSnapshot struct {
	ShellInterpreters   []string
	NetworkCommands     []string
	NetworkSubcommands  map[string][]string
	WriteCommands       []string
	WriteSubcommands    map[string][]string
	ReadOnlyCommands    []string
	ReadOnlySubcommands map[string][]string
}

// BuiltinCatalog returns a new Catalog holding the built-in command lists. Each call
// builds fresh maps, so Apply on one result never affects another.
func BuiltinCatalog() Catalog {
	return Catalog{
		shellInterpreters: stringSet("sh", "bash", "zsh", "fish", "dash", "ksh", "cmd", "powershell", "pwsh"),
		networkCommands:   stringSet("curl", "wget", "ssh", "scp", "sftp", "rsync", "gh", "brew", "npm", "pnpm", "yarn", "pip", "pip3"),
		networkSubcommands: map[string]map[string]bool{
			"git": stringSet("clone", "fetch", "pull", "push", "ls-remote", "submodule"),
		},
		writeCommands: stringSet("touch", "mkdir", "rmdir", "rm", "mv", "cp", "install", "chmod", "chown", "tee", "python", "python3", "node", "ruby", "perl", "sed", "awk"),
		writeSubcommands: map[string]map[string]bool{
			"git": stringSet("add", "commit", "reset", "clean", "checkout", "switch", "merge", "rebase", "cherry-pick", "revert", "push", "pull", "apply", "am", "stash"),
			"go":  stringSet("build", "test", "run", "install", "mod", "work", "generate"),
		},
		readOnlyCommands: stringSet("pwd", "ls", "cat", "grep", "rg", "find", "head", "tail", "wc", "test", "stat", "true", "false"),
		readOnlySubcommands: map[string]map[string]bool{
			"git": stringSet("status", "diff", "log", "show", "rev-parse", "branch", "remote", "ls-files", "grep", "describe", "merge-base", "config"),
			"go":  stringSet("version", "env", "list"),
		},
	}
}

// Apply adds the additional commands and subcommands named in overrides to the
// catalog. It only adds entries and never removes a built-in one; the catalog's maps are mutated in place.
func (catalog *Catalog) Apply(overrides policycontract.Overrides) {
	addToSet(catalog.shellInterpreters, overrides.AdditionalShellInterpreters)
	addToSet(catalog.networkCommands, overrides.AdditionalNetworkCommands)
	addSubcommands(catalog.networkSubcommands, overrides.AdditionalNetworkSubcommands)
	addToSet(catalog.writeCommands, overrides.AdditionalWriteCommands)
	addSubcommands(catalog.writeSubcommands, overrides.AdditionalWriteSubcommands)
	addToSet(catalog.readOnlyCommands, overrides.AdditionalReadOnlyCommands)
	addSubcommands(catalog.readOnlySubcommands, overrides.AdditionalReadOnlySubcommands)
}

func addToSet(set map[string]bool, values []string) {
	for _, value := range values {
		set[value] = true
	}
}

func addSubcommands(catalog map[string]map[string]bool, additions map[string][]string) {
	for command, subcommands := range additions {
		if catalog[command] == nil {
			catalog[command] = map[string]bool{}
		}
		addToSet(catalog[command], subcommands)
	}
}

// Classify reports how the catalog classifies argv. It matches argv[0] by its
// lowercase base name without a Windows executable extension and the first
// argument as a subcommand. An empty argv yields the zero classification.
func (catalog Catalog) Classify(argv []string) CommandClassification {
	if len(argv) == 0 {
		return CommandClassification{}
	}
	base := commandBase(argv[0])
	return CommandClassification{
		ShellCommand:    catalog.shellInterpreters[base] && !syntaxCheckOnly(argv),
		UsesNetwork:     catalog.networkCommands[base] || subcommandAllowed(catalog.networkSubcommands, base, argv),
		Writes:          catalog.writeCommands[base] || subcommandAllowed(catalog.writeSubcommands, base, argv) || writeCapableArguments(base, argv),
		ReadOnlyAllowed: catalog.readOnlyCommands[base] || subcommandAllowed(catalog.readOnlySubcommands, base, argv),
	}
}

// commandBase names argv[0] the way the catalog lists commands: the lowercase
// file name without a Windows executable extension, so C:\Git\bin\bash.exe
// and BASH.EXE classify as bash.
func commandBase(argv0 string) string {
	base := strings.ToLower(filepath.Base(argv0))
	for _, extension := range []string{".exe", ".com", ".bat", ".cmd"} {
		if trimmed, ok := strings.CutSuffix(base, extension); ok && trimmed != "" {
			return trimmed
		}
	}
	return base
}

// syntaxCheckOnly reports `sh -n FILE`: the shell parses FILE and executes
// nothing (POSIX -n, noexec), so it is not a shell execution. Repositories
// document `sh -n script.sh` as their lint step, and gate CHECKs could not run
// it. Exactly one operand is accepted: shells read options until the first
// operand, so a `+n` or `+o noexec` after -n turns execution back on (verified:
// `sh -n +n x.sh` runs x.sh), and further operands are positional parameters,
// not more files. Checked on macOS: sh, bash (BASH_ENV), and zsh (~/.zshenv)
// run no startup file under -n; fish is left out because its startup behavior
// under -n is unverified.
func syntaxCheckOnly(argv []string) bool {
	if len(argv) != 3 || argv[1] != "-n" || commandBase(argv[0]) == "fish" {
		return false
	}
	file := argv[2]
	return file != "" && !strings.HasPrefix(file, "-") && !strings.HasPrefix(file, "+")
}

// Snapshot returns the catalog contents as sorted slices, so the same catalog
// always renders the same report.
func (catalog Catalog) Snapshot() CatalogSnapshot {
	return CatalogSnapshot{
		ShellInterpreters:   sortedKeys(catalog.shellInterpreters),
		NetworkCommands:     sortedKeys(catalog.networkCommands),
		NetworkSubcommands:  sortedSubcommandCatalog(catalog.networkSubcommands),
		WriteCommands:       sortedKeys(catalog.writeCommands),
		WriteSubcommands:    sortedSubcommandCatalog(catalog.writeSubcommands),
		ReadOnlyCommands:    sortedKeys(catalog.readOnlyCommands),
		ReadOnlySubcommands: sortedSubcommandCatalog(catalog.readOnlySubcommands),
	}
}

func stringSet(items ...string) map[string]bool {
	result := make(map[string]bool, len(items))
	for _, item := range items {
		result[item] = true
	}
	return result
}

func subcommandAllowed(catalog map[string]map[string]bool, base string, argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	allowed, ok := catalog[base]
	return ok && allowed[strings.ToLower(argv[1])]
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedSubcommandCatalog(catalog map[string]map[string]bool) map[string][]string {
	result := make(map[string][]string, len(catalog))
	for command, subcommands := range catalog {
		result[command] = sortedKeys(subcommands)
	}
	return result
}
