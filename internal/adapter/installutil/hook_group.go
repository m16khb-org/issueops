package installutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateHookConfigForMerge checks that config's hook catalog and each of
// the knownEvents entries have the expected shape (object of group arrays), so
// a merge never overwrites a malformed host file. A config without "hooks" is
// valid.
func ValidateHookConfigForMerge(config map[string]any, knownEvents []string) error {
	hooksValue, present := config["hooks"]
	if !present {
		return nil
	}
	hooks, ok := hooksValue.(map[string]any)
	if !ok {
		return fmt.Errorf("installed hook catalog is malformed")
	}
	for _, event := range knownEvents {
		if groups, present := hooks[event]; present {
			if _, ok := groups.([]any); !ok {
				return fmt.Errorf("installed hook event %s is malformed", event)
			}
		}
	}
	return nil
}

// HookGroupContainsAgentHarness reports whether a host hook group already holds
// an issueops lifecycle hook command, so installers can remove their own
// prior hook while preserving third-party groups.
func HookGroupContainsAgentHarness(group any) bool {
	m, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, hook := range hooks {
		hm, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		if target, ok := hookEntryTarget(hm); ok && isAgentHarnessExecutable(target) {
			return true
		}
	}
	return false
}

// HookGroupContainsCommand reports whether any hook entry in group has a
// command that starts with commandPrefix.
func HookGroupContainsCommand(group any, commandPrefix string) bool {
	m, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, hook := range hooks {
		hm, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		if command, ok := hm["command"].(string); ok && strings.HasPrefix(command, commandPrefix) {
			return true
		}
	}
	return false
}

func isAgentHarnessExecutable(executable string) bool {
	return strings.TrimSuffix(filepath.Base(executable), ".exe") == "issueops"
}
