package policy

import (
	"path/filepath"
	"sort"
	"strings"

	policycontract "issueops/internal/contract/policy"
)

type Catalog struct {
	shellInterpreters   map[string]bool
	networkCommands     map[string]bool
	networkSubcommands  map[string]map[string]bool
	writeCommands       map[string]bool
	writeSubcommands    map[string]map[string]bool
	readOnlyCommands    map[string]bool
	readOnlySubcommands map[string]map[string]bool
}

type CommandClassification struct {
	ShellCommand    bool
	UsesNetwork     bool
	Writes          bool
	ReadOnlyAllowed bool
}

type CatalogSnapshot struct {
	ShellInterpreters   []string
	NetworkCommands     []string
	NetworkSubcommands  map[string][]string
	WriteCommands       []string
	WriteSubcommands    map[string][]string
	ReadOnlyCommands    []string
	ReadOnlySubcommands map[string][]string
}

func BuiltinCatalog() Catalog {
	return Catalog{
		shellInterpreters: stringSet("sh", "bash", "zsh", "fish", "dash", "ksh"),
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

func (catalog *Catalog) Apply(overrides policycontract.PolicyOverrides) {
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

func (catalog Catalog) Classify(argv []string) CommandClassification {
	if len(argv) == 0 {
		return CommandClassification{}
	}
	base := strings.ToLower(filepath.Base(argv[0]))
	return CommandClassification{
		ShellCommand:    catalog.shellInterpreters[base] && !syntaxCheckOnly(argv),
		UsesNetwork:     catalog.networkCommands[base] || subcommandAllowed(catalog.networkSubcommands, base, argv),
		Writes:          catalog.writeCommands[base] || subcommandAllowed(catalog.writeSubcommands, base, argv) || writeCapableArguments(base, argv),
		ReadOnlyAllowed: catalog.readOnlyCommands[base] || subcommandAllowed(catalog.readOnlySubcommands, base, argv),
	}
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
	if len(argv) != 3 || argv[1] != "-n" || strings.ToLower(filepath.Base(argv[0])) == "fish" {
		return false
	}
	file := argv[2]
	return file != "" && !strings.HasPrefix(file, "-") && !strings.HasPrefix(file, "+")
}

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
