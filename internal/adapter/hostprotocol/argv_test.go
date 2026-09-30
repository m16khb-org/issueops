package hostprotocol

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildInteractiveArgvPinsInstalledNativeHostContracts(t *testing.T) {
	prompt := "first line\n'quoted' \"double\" $() `ticks` \\backslash\n"
	tests := []struct {
		host       string
		executable string
		model      string
		effort     string
		want       []string
	}{
		{
			host: "codex", executable: "/opt/native/codex", model: "gpt-6-astra", effort: "high",
			want: []string{"/opt/native/codex", "--model", "gpt-6-astra", "-c", "model_reasoning_effort=high", "--dangerously-bypass-approvals-and-sandbox", "--", prompt},
		},
		{
			host: "claude", executable: "/opt/native/claude", model: "claude-sonnet-5-5", effort: "high",
			want: []string{"/opt/native/claude", "--model", "claude-sonnet-5-5", "--effort", "high", "--dangerously-skip-permissions", "--", prompt},
		},
		{
			host: "claude", executable: "/opt/native/claude", model: "claude-sonnet-5-5", effort: "xhigh",
			want: []string{"/opt/native/claude", "--model", "claude-sonnet-5-5", "--effort", "xhigh", "--dangerously-skip-permissions", "--", prompt},
		},
		{
			host: "omo", executable: "/opt/native/omo", model: "chatgpt-subscription/gpt-6-sol", effort: "xhigh",
			want: []string{"/opt/native/omo", "--model", "chatgpt-subscription/gpt-6-sol:xhigh", "--", prompt},
		},
	}
	for _, test := range tests {
		t.Run(test.host, func(t *testing.T) {
			got, err := BuildInteractiveArgv(test.host, test.executable, test.model, test.effort, prompt)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("argv=%q want=%q", got, test.want)
			}
			if test.host == "omo" && got[0] != test.executable {
				t.Fatalf("native Omo must use the absolute executable: %q", got)
			}
		})
	}
}

func TestBuildInteractiveArgvRejectsUnsupportedOrAmbiguousInputs(t *testing.T) {
	tests := []struct {
		name, host, executable, model, effort string
	}{
		{name: "unknown host", host: "opencode", executable: "/opt/native/opencode", model: "model"},
		{name: "relative executable", host: "codex", executable: "codex", model: "model"},
		{name: "wrong host executable", host: "codex", executable: "/usr/bin/python3", model: "model"},
		{name: "missing model", host: "codex", executable: "/opt/native/codex"},
		{name: "effort injection", host: "claude", executable: "/opt/native/claude", model: "model", effort: "high\n--danger"},
		{name: "model option injection", host: "omo", executable: "/opt/native/omo", model: "--help"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := BuildInteractiveArgv(test.host, test.executable, test.model, test.effort, "prompt"); err == nil {
				t.Fatal("unsafe argv accepted")
			}
		})
	}
}

func TestBuildInteractiveArgvPreservesEmptyPrompt(t *testing.T) {
	argv, err := BuildInteractiveArgv("codex", "/opt/native/codex", "model", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) == 0 || argv[len(argv)-1] != "" || strings.Join(argv, " ") == "" {
		t.Fatalf("empty prompt argument was lost: %#v", argv)
	}
}
