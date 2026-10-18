package hostprotocol

import (
	"encoding/json"
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
	if err := validateInteractiveHost(host, executable, model, effort); err != nil {
		return nil, err
	}
	if slices.ContainsFunc(extra, func(argument string) bool { return argument == "" || strings.ContainsRune(argument, 0) }) {
		return nil, fmt.Errorf("native host launch argument is empty or contains NUL")
	}
	if host == "opencode" {
		return opencodeInteractiveArgv(executable, model, prompt, extra), nil
	}

	argv := []string{executable, "--model", model}
	argv = appendHostLaunchFlags(argv, host, effort)
	argv = append(argv, extra...)
	return append(argv, "--", prompt), nil
}

// opencodeInteractiveArgv opens a private --standalone server so the session
// can own a lease. The TUI has no model flag: the model reaches the private
// server through OPENCODE_CONFIG_CONTENT, which a root-model variant cannot
// carry, so effort is not passed. An empty model keeps the user's choice.
func opencodeInteractiveArgv(executable, model, prompt string, extra []string) []string {
	var argv []string
	if model != "" {
		content, _ := json.Marshal(map[string]string{"model": model})
		argv = append(argv, "/usr/bin/env", "OPENCODE_CONFIG_CONTENT="+string(content))
	}
	argv = append(argv, executable, "--standalone", "--auto")
	argv = append(argv, extra...)
	return append(argv, "--prompt", prompt)
}

func validateInteractiveHost(host, executable, model, effort string) error {
	if !agentmodel.KnownHost(host) {
		return fmt.Errorf("unsupported native host %q", host)
	}
	if !filepath.IsAbs(executable) || filepath.Clean(executable) != executable || strings.ContainsAny(executable, "\x00\r\n") {
		return fmt.Errorf("native host executable must be an absolute literal path")
	}
	if !nativehost.ExecutableMatchesHost(host, executable) {
		return fmt.Errorf("native host executable does not match %s", host)
	}
	invalidModel := strings.HasPrefix(model, "-") || strings.ContainsAny(model, "\x00\r\n")
	if invalidModel || (model == "" && host != "opencode") {
		return fmt.Errorf("native host model is invalid")
	}
	if !agentmodel.SupportsEffort(host, effort) {
		return fmt.Errorf("native host effort %q is unsupported for %s", effort, host)
	}
	return nil
}

// appendHostLaunchFlags adds the host-specific effort and permission flags.
// argv must already hold {executable, "--model", model}; omo folds effort into the model.
func appendHostLaunchFlags(argv []string, host, effort string) []string {
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
	case "omp":
		if effort != "" {
			argv = append(argv, "--thinking", effort)
		}
		argv = append(argv, "--auto-approve")
	}
	return argv
}
