package agentmodel

import (
	"fmt"
	"slices"
	"strings"

	contract "issueops/internal/contract/agentmodel"
)

var claudeAliases = []string{"opus", "sonnet", "haiku", "fable"}

// ValidateSetting checks one `issueops model set` request. Malformed hosts,
// roles, efforts, and model strings are errors. A model the host may not
// know is only a warning, so users can pin a model before it ships.
// codexCatalog is the locally cached Codex model list; empty skips the check.
func ValidateSetting(host string, role contract.Role, layer contract.Layer, codexCatalog []string) ([]string, error) {
	if !slices.Contains(configurableHosts, host) {
		return nil, fmt.Errorf("host %q is not configurable (use %s)", host, strings.Join(configurableHosts, ", "))
	}
	if !knownRole(role) {
		return nil, fmt.Errorf("unknown role %q (use %s)", role, joinRoles())
	}
	if layer.Model == "" && layer.Effort == "" {
		return nil, fmt.Errorf("set needs a model or effort")
	}
	if err := validateLayer(host, layer); err != nil {
		return nil, err
	}
	return modelWarnings(host, layer.Model, codexCatalog), nil
}

func modelWarnings(host, model string, codexCatalog []string) []string {
	switch {
	case model == "":
		return nil
	case host == "claude" && !slices.Contains(claudeAliases, model) && !strings.HasPrefix(model, "claude-"):
		return []string{fmt.Sprintf("model %q is not a Claude alias or claude- model id", model)}
	case host == "codex" && len(codexCatalog) > 0 && !slices.Contains(codexCatalog, model):
		return []string{fmt.Sprintf("model %q is not in the local Codex model catalog", model)}
	default:
		return nil
	}
}

// ValidateConfig checks a decoded settings file. The zero Config stands for
// an absent file and is valid.
func ValidateConfig(cfg contract.Config) error {
	if cfg.Version == 0 && cfg.Claude == nil && cfg.Codex == nil {
		return nil
	}
	if cfg.Version != contract.ConfigVersion {
		return fmt.Errorf("unsupported version %d (want %d)", cfg.Version, contract.ConfigVersion)
	}
	for _, host := range configurableHosts {
		for role, layer := range hostLayers(cfg, host) {
			if !knownRole(role) {
				return fmt.Errorf("%s: unknown role %q (use %s)", host, role, joinRoles())
			}
			if err := validateLayer(host, layer); err != nil {
				return fmt.Errorf("%s.%s: %w", host, role, err)
			}
		}
	}
	return nil
}

func validateLayer(host string, layer contract.Layer) error {
	if layer.Model != "" && (strings.HasPrefix(layer.Model, "-") || strings.ContainsFunc(layer.Model, func(r rune) bool { return r <= ' ' || r == 0x7f })) {
		return fmt.Errorf("model %q must not start with '-' or contain spaces or control characters", layer.Model)
	}
	if !SupportsEffort(host, layer.Effort) {
		return fmt.Errorf("effort %q is unsupported for %s (use %s)", layer.Effort, host, strings.Join(effortLadders[host], ", "))
	}
	return nil
}
