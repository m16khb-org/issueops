package hostprotocol

// FormatHookContext gives every host the same model-facing text and adds the
// user-facing notice as systemMessage only where the host shows it apart from
// the model context. Codex already shows additionalContext in its TUI, so it
// gets no systemMessage. Catalog hooks default to Claude for an empty or
// unknown host.
func FormatHookContext(host, eventName, additionalContext, userView string) map[string]any {
	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     eventName,
			"additionalContext": additionalContext,
		},
	}
	if host != "codex" && userView != "" {
		output["systemMessage"] = userView
	}
	return output
}
