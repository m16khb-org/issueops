package agentmodel

import "testing"

func TestImplementerDefaults(t *testing.T) {
	tests := []struct {
		host, model, effort string
		ok                  bool
	}{
		{host: "codex", model: "gpt-6-sol", effort: "high", ok: true},
		{host: "claude", model: "claude-sonnet-5-5", effort: "high", ok: true},
		{host: "omo", model: "chatgpt-subscription/gpt-6-sol", effort: "max", ok: true},
		{host: "unknown"},
	}
	for _, test := range tests {
		model, effort, ok := ImplementerDefaults(test.host)
		if model != test.model || effort != test.effort || ok != test.ok {
			t.Fatalf("host=%s defaults=(%q,%q,%v)", test.host, model, effort, ok)
		}
	}
}
