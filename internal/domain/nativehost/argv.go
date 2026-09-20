package nativehost

import (
	"fmt"
	"path/filepath"
	"strings"
)

var supportedEfforts = map[string]map[string]bool{
	"codex":  {"": true, "minimal": true, "low": true, "medium": true, "high": true, "xhigh": true, "max": true},
	"claude": {"": true, "low": true, "medium": true, "high": true, "max": true},
	"omo":    {"": true, "off": true, "minimal": true, "low": true, "medium": true, "high": true, "xhigh": true, "max": true},
}

// BuildInteractiveArgv returns the exact native host argv used by terminal
// launchers. The executable stays caller-supplied and absolute; this function
// never substitutes cmux's host-specific convenience commands.
func BuildInteractiveArgv(host, executable, model, effort, prompt string) ([]string, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	executable = strings.TrimSpace(executable)
	model = strings.TrimSpace(model)
	effort = strings.ToLower(strings.TrimSpace(effort))
	if supportedEfforts[host] == nil {
		return nil, fmt.Errorf("unsupported native host %q", host)
	}
	if !filepath.IsAbs(executable) || filepath.Clean(executable) != executable || strings.ContainsAny(executable, "\x00\r\n") {
		return nil, fmt.Errorf("native host executable must be an absolute literal path")
	}
	if model == "" || strings.HasPrefix(model, "-") || strings.ContainsAny(model, "\x00\r\n") {
		return nil, fmt.Errorf("native host model is invalid")
	}
	if !supportedEfforts[host][effort] {
		return nil, fmt.Errorf("native host effort %q is unsupported for %s", effort, host)
	}

	argv := []string{executable, "--model", model}
	switch host {
	case "codex":
		if effort != "" {
			argv = append(argv, "-c", "model_reasoning_effort="+effort)
		}
	case "claude":
		if effort != "" {
			argv = append(argv, "--effort", effort)
		}
	case "omo":
		if effort != "" {
			argv[2] += ":" + effort
		}
	}
	return append(argv, "--", prompt), nil
}
