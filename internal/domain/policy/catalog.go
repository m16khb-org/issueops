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
		writeCommands: stringSet("touch", "mkdir", "rmdir", "rm", "mv", "cp", "install", "chmod", "chown", "tee", "python", "python3", "node", "ruby", "perl"),
		writeSubcommands: map[string]map[string]bool{
			"git": stringSet("add", "commit", "reset", "clean", "checkout", "switch", "merge", "rebase", "cherry-pick", "revert", "push", "pull", "apply", "am", "stash"),
			"go":  stringSet("build", "test", "run", "install", "mod", "work", "generate"),
		},
		readOnlyCommands: stringSet("pwd", "ls", "cat", "grep", "rg", "find", "sed", "awk", "head", "tail", "wc", "test", "stat", "true", "false"),
		readOnlySubcommands: map[string]map[string]bool{
			"git": stringSet("status", "diff", "log", "show", "rev-parse", "branch", "remote", "ls-files", "grep", "describe", "merge-base", "config"),
			"go":  stringSet("version", "env", "list"),
		},
	}
}

func (catalog *Catalog) Apply(overrides policycontract.PolicyOverrides) {
	for _, value := range overrides.AdditionalShellInterpreters {
		catalog.shellInterpreters[value] = true
	}
	for _, value := range overrides.AdditionalNetworkCommands {
		catalog.networkCommands[value] = true
	}
	for command, subcommands := range overrides.AdditionalNetworkSubcommands {
		if catalog.networkSubcommands[command] == nil {
			catalog.networkSubcommands[command] = map[string]bool{}
		}
		for _, subcommand := range subcommands {
			catalog.networkSubcommands[command][subcommand] = true
		}
	}
	for _, value := range overrides.AdditionalWriteCommands {
		catalog.writeCommands[value] = true
	}
	for command, subcommands := range overrides.AdditionalWriteSubcommands {
		if catalog.writeSubcommands[command] == nil {
			catalog.writeSubcommands[command] = map[string]bool{}
		}
		for _, subcommand := range subcommands {
			catalog.writeSubcommands[command][subcommand] = true
		}
	}
	for _, value := range overrides.AdditionalReadOnlyCommands {
		catalog.readOnlyCommands[value] = true
	}
	for command, subcommands := range overrides.AdditionalReadOnlySubcommands {
		if catalog.readOnlySubcommands[command] == nil {
			catalog.readOnlySubcommands[command] = map[string]bool{}
		}
		for _, subcommand := range subcommands {
			catalog.readOnlySubcommands[command][subcommand] = true
		}
	}
}

func (catalog Catalog) Classify(argv []string) CommandClassification {
	if len(argv) == 0 {
		return CommandClassification{}
	}
	base := strings.ToLower(filepath.Base(argv[0]))
	return CommandClassification{
		ShellCommand:    catalog.shellInterpreters[base],
		UsesNetwork:     catalog.networkCommands[base] || subcommandAllowed(catalog.networkSubcommands, base, argv),
		Writes:          catalog.writeCommands[base] || subcommandAllowed(catalog.writeSubcommands, base, argv),
		ReadOnlyAllowed: catalog.readOnlyCommands[base] || subcommandAllowed(catalog.readOnlySubcommands, base, argv),
	}
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
