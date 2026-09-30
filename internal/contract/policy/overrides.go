package policy

// PolicyOverrides is the workspace-local JSON shape for additive command rules.
type PolicyOverrides struct {
	AdditionalShellInterpreters   []string            `json:"additional_shell_interpreters,omitempty"`
	AdditionalNetworkCommands     []string            `json:"additional_network_commands,omitempty"`
	AdditionalNetworkSubcommands  map[string][]string `json:"additional_network_subcommands,omitempty"`
	AdditionalWriteCommands       []string            `json:"additional_write_commands,omitempty"`
	AdditionalWriteSubcommands    map[string][]string `json:"additional_write_subcommands,omitempty"`
	AdditionalReadOnlyCommands    []string            `json:"additional_read_only_commands,omitempty"`
	AdditionalReadOnlySubcommands map[string][]string `json:"additional_read_only_subcommands,omitempty"`
}
