// Package hookinput reads the few host hook stdin fields the context hooks
// need. Hosts send the current working directory as cwd; the repo, workspace,
// workspace_root, and project_dir keys and a nested hook_input envelope are
// read too, so the same reader serves Codex, Claude Code, and direct CLI use.
// SubagentStart input also names the starting agent as agent_type.
package hookinput

import (
	"encoding/json"
	"strings"
)

var repoKeys = []string{"repo", "cwd", "workspace", "workspace_root", "project_dir"}

func RepoFromHookInput(input []byte) string {
	return firstValue(input, repoKeys)
}

func AgentTypeFromHookInput(input []byte) string {
	return firstValue(input, []string{"agent_type"})
}

func firstValue(input []byte, keys []string) string {
	if len(strings.TrimSpace(string(input))) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err != nil {
		return ""
	}
	if value := firstKey(obj, keys); value != "" {
		return value
	}
	if nested, ok := obj["hook_input"].(map[string]any); ok {
		return firstKey(nested, keys)
	}
	return ""
}

func firstKey(obj map[string]any, keys []string) string {
	for _, key := range keys {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
