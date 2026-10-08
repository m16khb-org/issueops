package hostprotocol

import (
	"fmt"
	"issueops/internal/domain/agentmodel"
	"issueops/internal/domain/nativehost"
	"path/filepath"
	"slices"
	"strings"
)

// BuildInteractiveArgv returns the exact native host argv used by terminal
// launchers. The executable stays caller-supplied and absolute; this function
// never substitutes cmux's host-specific convenience commands. extra holds
// launch arguments such as role agents and goes right before "--".
func BuildInteractiveArgv(host, executable, model, effort, prompt string, extra []string) ([]string, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	executable = strings.TrimSpace(executable)
	model = strings.TrimSpace(model)
	effort = strings.ToLower(strings.TrimSpace(effort))
	if !agentmodel.KnownHost(host) {
		return nil, fmt.Errorf("unsupported native host %q", host)
	}
	if !filepath.IsAbs(executable) || filepath.Clean(executable) != executable || strings.ContainsAny(executable, "\x00\r\n") {
		return nil, fmt.Errorf("native host executable must be an absolute literal path")
	}
	if !nativehost.ExecutableMatchesHost(host, executable) {
		return nil, fmt.Errorf("native host executable does not match %s", host)
	}
	if model == "" || strings.HasPrefix(model, "-") || strings.ContainsAny(model, "\x00\r\n") {
		return nil, fmt.Errorf("native host model is invalid")
	}
	if !agentmodel.SupportsEffort(host, effort) {
		return nil, fmt.Errorf("native host effort %q is unsupported for %s", effort, host)
	}

	argv := []string{executable, "--model", model}
	switch host {
	case "codex":
		if effort != "" {
			argv = append(argv, "-c", "model_reasoning_effort="+effort)
		}
		argv = append(argv, "--dangerously-bypass-approvals-and-sandbox")
	case "claude":
		if effort != "" {
			argv = append(argv, "--effort", effort)
		}
		argv = append(argv, "--dangerously-skip-permissions")
	case "omo":
		if effort != "" {
			argv[2] += ":" + effort
		}
		argv = append(argv, "--permission-preset", "full-access")
	}
	if slices.ContainsFunc(extra, func(argument string) bool { return argument == "" || strings.ContainsRune(argument, 0) }) {
		return nil, fmt.Errorf("native host launch argument is empty or contains NUL")
	}
	argv = append(argv, extra...)
	return append(argv, "--", prompt), nil
}
