package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	policyapp "issueops/internal/application/policy"
	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

type OverrideLoader struct{}

func (OverrideLoader) Load(repoRoot string) policyapp.OverrideSnapshot {
	overrides, err := readPolicyOverrides(repoRoot)
	if err != nil {
		return policyapp.OverrideSnapshot{Warning: policyOverrideWarning(err)}
	}
	return policyapp.OverrideSnapshot{Values: overrides}
}

func readPolicyOverrides(repoRoot string) (*policycontract.PolicyOverrides, error) {
	path := filepath.Join(repoRoot, ".issueops", "policy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("policy_override_read_failed: %w", err)
	}
	var overrides policycontract.PolicyOverrides
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
