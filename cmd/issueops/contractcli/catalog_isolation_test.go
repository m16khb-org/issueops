package contractcli

import (
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"testing"
)

func TestCompatibilityContractUsesExplicitCatalogs(t *testing.T) {
	for _, name := range []string{"catalog-first", "catalog-second"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			commands := clicatalog.Commands()
			commands[0].Description = name
			tools := append(mcpcatalog.Build().Tools, map[string]any{"name": name})
			got := BuildCompatibilityContract(commands, tools)
			if !got.OK || len(got.CLICommands) != len(commands) || got.CLICommands[0].Description != name || !containsString(got.MCPTools, name) {
				t.Fatalf("explicit catalog not preserved: %+v", got)
			}
			for _, other := range []string{"catalog-first", "catalog-second"} {
				if other != name && containsString(got.MCPTools, other) {
					t.Fatalf("catalog leaked from another call: %s", other)
				}
			}
		})
	}
}
