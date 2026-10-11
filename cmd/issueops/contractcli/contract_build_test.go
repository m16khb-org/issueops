package contractcli

import (
	"slices"
	"strings"
	"testing"

	clicontract "issueops/internal/contract/cli"
	"issueops/internal/testsupport"
)

func toolMaps(names ...string) []map[string]any {
	out := make([]map[string]any, 0, len(names))
	for _, n := range names {
		out = append(out, map[string]any{"name": n})
	}
	return out
}

func TestBuildCompatibilityContractFlagsMissingRequiredAndExtraIssueopsTools(t *testing.T) {
	required := []string{"contract_schema", "worker_enqueue", "command_fake_run"}
	good := BuildCompatibilityContract(nil, toolMaps(append(slices.Clone(required), "issueops_execution")...))
	if !good.OK || len(good.Warnings) != 0 {
		t.Fatalf("good=%+v", good)
	}

	missing := BuildCompatibilityContract(nil, toolMaps("contract_schema", "issueops_execution"))
	if missing.OK || !slices.Equal(missing.Warnings, []string{"missing_mcp_tool:worker_enqueue", "missing_mcp_tool:command_fake_run"}) {
		t.Fatalf("missing=%+v", missing)
	}

	extra := BuildCompatibilityContract(nil, toolMaps(append(slices.Clone(required), "issueops_execution", "issueops_next")...))
	if extra.OK || !slices.Equal(extra.Warnings, []string{"issueops_mcp_surface_mismatch:issueops_execution,issueops_next"}) {
		t.Fatalf("extra=%+v", extra)
	}

	none := BuildCompatibilityContract(nil, toolMaps(required...))
	if none.OK || !slices.Equal(none.Warnings, []string{"issueops_mcp_surface_mismatch:"}) {
		t.Fatalf("none=%+v", none)
	}
}

func TestBuildCompatibilityContractSortsNamesIgnoresNonStringAndHashTracksSurface(t *testing.T) {
	tools := append(toolMaps("worker_enqueue", "contract_schema"), map[string]any{"name": 7}, map[string]any{})
	got := BuildCompatibilityContract(nil, tools)
	if !slices.Equal(got.MCPTools, []string{"contract_schema", "worker_enqueue"}) {
		t.Fatalf("MCPTools=%v", got.MCPTools)
	}
	same := BuildCompatibilityContract(nil, tools)
	if got.Hash == "" || got.Hash != same.Hash {
		t.Fatalf("hash not deterministic: %q vs %q", got.Hash, same.Hash)
	}
	withCommand := BuildCompatibilityContract([]clicontract.Command{{Name: "extra"}}, tools)
	if withCommand.Hash == got.Hash {
		t.Fatal("hash ignores CLI command surface")
	}
	withTool := BuildCompatibilityContract(nil, append(slices.Clone(tools), map[string]any{"name": "zzz"}))
	if withTool.Hash == got.Hash {
		t.Fatal("hash ignores MCP tool surface")
	}
}

func TestRunContractCheckPrintsWarningsAndFailsOnBrokenSurface(t *testing.T) {
	var runErr error
	out := testsupport.CaptureStdout(t, func() error {
		runErr = Run([]string{"check"}, nil, toolMaps("contract_schema"), nil)
		return nil
	})
	if runErr == nil || runErr.Error() != "contract check failed" {
		t.Fatalf("err=%v", runErr)
	}
	for _, want := range []string{"warning: missing_mcp_tool:worker_enqueue\n", "warning: missing_mcp_tool:command_fake_run\n", "warning: issueops_mcp_surface_mismatch:\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("out=%q missing %q", out, want)
		}
	}
}

func TestRunContractSchemaTextListsWarningsForBrokenSurface(t *testing.T) {
	out := testsupport.CaptureStdout(t, func() error {
		return Run([]string{"schema"}, nil, toolMaps("contract_schema"), nil)
	})
	if !strings.HasPrefix(out, "issueops_cli_mcp_compatibility v3 ") || !strings.Contains(out, "warning: missing_mcp_tool:worker_enqueue") {
		t.Fatalf("out=%q", out)
	}
}

func TestRunContractConformanceRoutesToConformanceRunner(t *testing.T) {
	c := NewConformance(ConformanceDependencies{})
	err := Run([]string{"conformance", "bogus"}, nil, nil, c)
	if err == nil || err.Error() != `unknown conformance subcommand "bogus"` {
		t.Fatalf("err=%v", err)
	}
}
