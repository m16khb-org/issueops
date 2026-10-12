//go:build unix

package hostprobe

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"issueops/internal/port"
)

// hostsMutRunOmoTool runs the omo runner with the given probe tool and returns
// the --tools value passed to the host and the run result.
func hostsMutRunOmoTool(t *testing.T, probeTool string) (string, port.HostProbeResult) {
	t.Helper()
	harnessBinary, err := filepath.Abs(filepath.Join("testdata", "issueops"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "episode")
	sourceAgentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceAgentDir, "auth.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := omoProbeRequest()
	request.ProbeTool = probeTool
	var toolsValue string
	process := &omoFakeProcess{run: func(_ context.Context, command CommandRequest) (CommandOutput, error) {
		index := slices.Index(command.Argv, "--tools")
		if index < 0 || index+1 >= len(command.Argv) {
			t.Fatalf("argv has no --tools value: %#v", command.Argv)
		}
		toolsValue = command.Argv[index+1]
		writeOmoCapture(t, filepath.Join(root, "result.json"), request)
		return CommandOutput{Stdout: omoSuccessfulStream(request, toolsValue)}, nil
	}}
	runner := newTestOmoRunner(harnessBinary, omoTestLifecycleExtension(harnessBinary), Dependencies{
		Process:  process,
		LookPath: func(string) (string, error) { return "/test/bin/omo", nil },
		TempDir: func(_, _ string) (string, error) {
			return root, os.Mkdir(root, 0o755)
		},
		Environ: func() []string { return []string{"PATH=/test/bin"} },
		Getenv: func(name string) string {
			if name == "OMO_CODING_AGENT_DIR" {
				return sourceAgentDir
			}
			return ""
		},
	})
	result := runner.Run(context.Background(), request)
	return toolsValue, result
}

func TestOmoRunnerSanitizesProbeToolNameForNativeToolAllowlist(t *testing.T) {
	tools, result := hostsMutRunOmoTool(t, "x.y z")
	if want := "mcp_issueops_probe_x_y_z"; tools != want {
		t.Fatalf("--tools = %q, want %q", tools, want)
	}
	if !result.Completed || result.CallCount != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestOmoRunnerKeepsToolNameAtLimitAndShortensLongerOnes(t *testing.T) {
	const prefix = "mcp_issueops_probe_"
	atLimit := strings.Repeat("a", 64-len(prefix))
	tools, result := hostsMutRunOmoTool(t, atLimit)
	if tools != prefix+atLimit || !result.Completed {
		t.Fatalf("limit-sized --tools = %q, result = %+v", tools, result)
	}

	longer := strings.Repeat("a", 64-len(prefix)) + "tail9"
	tools, result = hostsMutRunOmoTool(t, longer)
	if len(tools) > 64 || tools == prefix+longer || !strings.HasPrefix(tools, prefix) || !strings.HasSuffix(tools, "tail9") || !result.Completed {
		t.Fatalf("shortened --tools = %q (%d), result = %+v", tools, len(tools), result)
	}
}

func TestObserveOmoStreamAcceptsExactlyMaxOutputBytes(t *testing.T) {
	request := omoProbeRequest()
	target := "mcp_issueops_probe_" + request.ProbeTool
	base := string(omoSuccessfulStream(request, target))
	if len(base) >= MaxOutputBytes {
		t.Skip("base stream too large to pad")
	}
	exact := base + strings.Repeat(" ", MaxOutputBytes-len(base))
	if _, err := observeOmoStream([]byte(exact), target); err != nil {
		t.Fatalf("stream of exactly MaxOutputBytes rejected: %v", err)
	}
	if _, err := observeOmoStream([]byte(exact+" "), target); err == nil {
		t.Fatal("stream over MaxOutputBytes accepted")
	}
}

func TestOmoOutputDropsDisplayEventOfExactlyMaxOutputBytes(t *testing.T) {
	prefix := `{"type":"message_update","text":"`
	suffix := `"}`
	event := prefix + strings.Repeat("x", MaxOutputBytes-len(prefix)-len(suffix)) + suffix
	output := &boundedBuffer{limit: MaxOutputBytes}
	projection := &omoOutput{output: output}
	if _, err := projection.Write([]byte(event + "\n" + `{"type":"unknown_event"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	if output.truncated || output.String() != `{"type":"unknown_event"}`+"\n" {
		t.Fatalf("output = %q truncated=%t", output.String(), output.truncated)
	}
}

func TestOmpProbeToolNameAcceptsNameOfExactlyLimit(t *testing.T) {
	name, ok := ompProbeToolName(strings.Repeat("a", 44))
	if !ok || len(name) != ompToolNameLimit {
		t.Fatalf("name = %q (%d), ok=%t", name, len(name), ok)
	}
}

func TestReadOmoAuthFileEnforcesSizeLimitInclusively(t *testing.T) {
	const limit = 8
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	got, err := readOmoAuthFile(write("exact.json", "12345678"), limit)
	if err != nil || string(got) != "12345678" {
		t.Fatalf("exact-size auth = %q, %v", got, err)
	}
	if _, err := readOmoAuthFile(write("over.json", "123456789"), limit); err == nil {
		t.Fatal("oversized auth accepted")
	}
}
