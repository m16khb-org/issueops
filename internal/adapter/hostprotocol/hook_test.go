package hostprotocol

import "testing"

func TestHookContextKeepsHostChannelsAndDefault(t *testing.T) {
	for _, tc := range []struct {
		host, user, context, message string
		hasMessage                   bool
	}{
		{"codex", "readable", "readable", "", false},
		{"codex", "", "compact", "", false},
		{"claude", "readable", "compact", "readable", true},
		{"claude", "", "compact", "", false},
		{"", "readable", "compact", "readable", true},
		{"unknown", "readable", "compact", "readable", true},
	} {
		t.Run(tc.host+"/"+tc.user, func(t *testing.T) {
			out := FormatHookContext(tc.host, "SessionStart", "compact", tc.user)
			specific, ok := out["hookSpecificOutput"].(map[string]any)
			if !ok || specific["hookEventName"] != "SessionStart" || specific["additionalContext"] != tc.context {
				t.Fatalf("context=%+v", out)
			}
			message, hasMessage := out["systemMessage"]
			if hasMessage != tc.hasMessage || (hasMessage && message != tc.message) {
				t.Fatalf("message=%+v", out)
			}
		})
	}
}
