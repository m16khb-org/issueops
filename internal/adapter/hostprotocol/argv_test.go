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
			want: []string{"/opt/native/omo", "--model", "chatgpt-subscription/gpt-6-sol:xhigh", "--permission-preset", "full-access", "--", prompt},
		},
		{
			host: "omp", executable: "/Users/u/.bun/bin/omp", model: "anthropic/claude-opus-5-5", effort: "high",
			want: []string{"/Users/u/.bun/bin/omp", "--model", "anthropic/claude-opus-5-5", "--thinking", "high", "--auto-approve", "--", prompt},
		},
	}
	for _, test := range tests {
		t.Run(test.host, func(t *testing.T) {
			got, err := BuildInteractiveArgv(test.host, test.executable, test.model, test.effort, prompt, nil)
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
		{name: "unknown host", host: "gemini", executable: "/opt/native/gemini", model: "model"},
		{name: "relative executable", host: "codex", executable: "codex", model: "model"},
		{name: "wrong host executable", host: "codex", executable: "/usr/bin/python3", model: "model"},
		{name: "missing model", host: "codex", executable: "/opt/native/codex"},
		{name: "effort injection", host: "claude", executable: "/opt/native/claude", model: "model", effort: "high\n--danger"},
		{name: "model option injection", host: "omo", executable: "/opt/native/omo", model: "--help"},
		{name: "unsupported omp effort", host: "omp", executable: "/opt/native/omp", model: "model", effort: "auto"},
		{name: "wrong opencode executable", host: "opencode", executable: "/opt/native/omp", model: "opencode/m"},
		{name: "opencode model option injection", host: "opencode", executable: "/opt/native/opencode", model: "--help"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := BuildInteractiveArgv(test.host, test.executable, test.model, test.effort, "prompt", nil); err == nil {
				t.Fatal("unsafe argv accepted")
			}
		})
	}
}

func TestBuildInteractiveArgvPreservesEmptyPrompt(t *testing.T) {
	argv, err := BuildInteractiveArgv("codex", "/opt/native/codex", "model", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) == 0 || argv[len(argv)-1] != "" || strings.Join(argv, " ") == "" {
		t.Fatalf("empty prompt argument was lost: %#v", argv)
	}
}

func TestBuildInteractiveArgvExtraArgs(t *testing.T) {
	extra := []string{"--agents", `{"issueops-research":{"model":"claude-sonnet-5-5"}}`}
	got, err := BuildInteractiveArgv("claude", "/opt/native/claude", "claude-opus-5-5", "high", "go", extra)
	want := []string{"/opt/native/claude", "--model", "claude-opus-5-5", "--effort", "high", "--dangerously-skip-permissions", "--agents", `{"issueops-research":{"model":"claude-sonnet-5-5"}}`, "--", "go"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("argv=%q err=%v", got, err)
	}
	if _, err := BuildInteractiveArgv("claude", "/opt/native/claude", "m", "", "go", []string{"a\x00b"}); err == nil {
		t.Fatal("a NUL launch argument must be rejected")
	}
}

func TestBuildInteractiveArgvOpencodeStandalone(t *testing.T) {
	prompt := "first line\n'quoted' $() `ticks`"
	got, err := BuildInteractiveArgv("opencode", "/opt/native/opencode", "opencode/deepseek-v4.1-flash-free", "max", prompt, []string{"--continue"})
	want := []string{
		"/usr/bin/env", `OPENCODE_CONFIG_CONTENT={"model":"opencode/deepseek-v4.1-flash-free"}`,
		"/opt/native/opencode", "--standalone", "--auto", "--continue", "--prompt", prompt,
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("argv=%q err=%v, want %q", got, err, want)
	}
	// The TUI has no model flag and drops a root-model variant, so effort
	// never reaches the argv; an unset model leaves the user's choice alone.
	got, err = BuildInteractiveArgv("opencode", "/opt/native/opencode", "", "", "", nil)
	want = []string{"/opt/native/opencode", "--standalone", "--auto", "--prompt", ""}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("argv without model=%q err=%v, want %q", got, err, want)
	}
}
