package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

type PolicyOverrides = policycontract.PolicyOverrides

func policyCatalogForWorkspace(repoRoot string) (policydomain.Catalog, []string) {
	catalog := policydomain.BuiltinCatalog()
	overrides, err := readPolicyOverrides(repoRoot)
	if err != nil {
		return catalog, []string{policyOverrideWarning(err)}
	}
	if overrides != nil {
		catalog.Apply(*overrides)
	}
	return catalog, []string{}
}

func readPolicyOverrides(repoRoot string) (*PolicyOverrides, error) {
	path := filepath.Join(repoRoot, ".issueops", "policy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("policy_override_read_failed: %w", err)
	}
	var overrides PolicyOverrides
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("policy_override_parse_failed: %w", err)
	}
	return &overrides, nil
}

func policyOverrideWarning(err error) string {
	msg := err.Error()
	if strings.HasPrefix(msg, "policy_override_parse_failed:") {
		return "policy_override_parse_failed"
	}
	if strings.HasPrefix(msg, "policy_override_read_failed:") {
		return "policy_override_read_failed"
	}
	return "policy_override_failed"
}

func commandPolicyCatalog() map[string]any {
	snapshot := policydomain.BuiltinCatalog().Snapshot()
	return map[string]any{
		"shell_interpreters":     snapshot.ShellInterpreters,
		"network_commands":       snapshot.NetworkCommands,
		"network_subcommands":    snapshot.NetworkSubcommands,
		"write_commands":         snapshot.WriteCommands,
		"write_subcommands":      snapshot.WriteSubcommands,
		"read_only_commands":     snapshot.ReadOnlyCommands,
		"read_only_subcommands":  snapshot.ReadOnlySubcommands,
		"secret_path_patterns":   []string{"env files", "private keys", "credentials", "secret-like paths"},
		"secret_arg_assignments": []string{"token=", "password=", "secret=", "api_key=", "credential=", "authorization="},
	}
}
