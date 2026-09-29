package hostprotocol

// FormatHookContext preserves each host's model-facing and user-facing channels.
// Catalog hooks default to Claude for an empty or unknown host.
func FormatHookContext(host, eventName, additionalContext, userView string) map[string]any {
	context := additionalContext
	if host == "codex" && userView != "" {
		context = userView
	}
	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     eventName,
			"additionalContext": context,
		},
	}
	if host != "codex" && userView != "" {
		output["systemMessage"] = userView
	}
	return output
}
