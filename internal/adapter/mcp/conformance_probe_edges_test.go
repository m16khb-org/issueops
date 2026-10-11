package mcp

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	mcpcontract "issueops/internal/contract/mcp"
	fixturecontract "issueops/internal/contract/toolconformance"
)

func TestNewConformanceProbeRejectsIncompleteConfigWithSpecificReason(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(c *mcpcontract.ConformanceProbeConfig)
		want   string
	}{
		{"missing probe tool", func(c *mcpcontract.ConformanceProbeConfig) { c.ProbeTool = "" }, "probe_tool_required"},
		{"missing result path", func(c *mcpcontract.ConformanceProbeConfig) { c.ResultPath = "" }, "result_path_required"},
		{"missing expected arguments", func(c *mcpcontract.ConformanceProbeConfig) { c.ExpectedArguments = nil }, "expected_arguments_required"},
		{"schema hash mismatch", func(c *mcpcontract.ConformanceProbeConfig) { c.SchemaSHA = "deadbeef" }, "schema_hash_mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := testProbeConfig(t, filepath.Join(t.TempDir(), "capture.json"))
			tc.mutate(&config)
			if _, err := newConformanceProbe(config); err == nil || err.Error() != tc.want {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
			if _, err := NewConformanceProbeServer(config); err == nil || err.Error() != tc.want {
				t.Fatalf("server err=%v want %q", err, tc.want)
			}
			err := ServeConformanceProbe(t.Context(), strings.NewReader(""), &strings.Builder{}, config)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("serve err=%v want %q", err, tc.want)
			}
		})
	}
}

func TestNewConformanceProbeServerRejectsUnserializableSchemaOrExpectedArguments(t *testing.T) {
	config := testProbeConfig(t, filepath.Join(t.TempDir(), "capture.json"))
	bad := map[string]any{"type": "object", "bad": make(chan int)}

	withSchema := config
	withSchema.Schema = bad
	if _, err := NewConformanceProbeServer(withSchema); err == nil {
		t.Fatal("unserializable schema accepted")
	}
	withExpected := config
	withExpected.ExpectedArguments = bad
	if _, err := NewConformanceProbeServer(withExpected); err == nil {
		t.Fatal("unserializable expected arguments accepted")
	}
}

func TestCloneSchemaNilStaysNilAndCopiesDeeply(t *testing.T) {
	if got, err := cloneSchema(nil); got != nil || err != nil {
		t.Fatalf("nil clone = %v, %v", got, err)
	}
	source := map[string]any{"properties": map[string]any{"a": map[string]any{"type": "string"}}}
	clone, err := cloneSchema(source)
	if err != nil {
		t.Fatal(err)
	}
	clone["properties"].(map[string]any)["a"].(map[string]any)["type"] = "number"
	if source["properties"].(map[string]any)["a"].(map[string]any)["type"] != "string" {
		t.Fatal("clone shares nested maps with source")
	}
}

func TestRedactArgumentsHashesUnknownKeysAndMasksSensitiveOnes(t *testing.T) {
	schema := map[string]any{
		"properties": map[string]any{
			"items": map[string]any{
				"items": map[string]any{
					"properties": map[string]any{"name": map[string]any{}},
				},
			},
			"Access-Token": map[string]any{},
		},
	}
	value := map[string]any{
		"items": []any{
			map[string]any{"name": "ok", "stray": "kept-value", "client_secret": "x"},
			"password=hunter2",
			float64(7),
		},
		"Access-Token": "abc",
		"a/b":          true,
	}
	replacements := map[string]string{}
	got := redactArguments(value, schema, "", replacements).(map[string]any)

	slashKey := "unknown_key_" + sha256Hex([]byte("a/b"))[:12]
	strayKey := "unknown_key_" + sha256Hex([]byte("stray"))[:12]
	secretKey := "unknown_key_" + sha256Hex([]byte("client_secret"))[:12]
	if got["Access-Token"] != "<redacted>" {
		t.Fatalf("declared sensitive key not masked: %#v", got)
	}
	if got[slashKey] != true || len(got) != 3 {
		t.Fatalf("undeclared key not renamed: %#v", got)
	}
	list := got["items"].([]any)
	first := list[0].(map[string]any)
	if first["name"] != "ok" || first[strayKey] != "kept-value" || first[secretKey] != "<redacted>" || len(first) != 3 {
		t.Fatalf("element not redacted: %#v", first)
	}
	if list[1] != "<redacted>" || list[2] != float64(7) {
		t.Fatalf("scalar elements: %#v", list[1:])
	}
	want := map[string]string{
		"/a~1b":                  "/" + slashKey,
		"/items/0/stray":         "/items/0/" + strayKey,
		"/items/0/client_secret": "/items/0/" + secretKey,
	}
	if !sameJSON(replacements, want) {
		t.Fatalf("replacements=%v want=%v", replacements, want)
	}
}

func TestRedactDiagnosticPathsRewritesOnlyReplacedPathsWithoutMutatingInput(t *testing.T) {
	in := []fixturecontract.Diagnostic{{Path: "/secret_key"}, {Path: "/declared"}}
	out := redactDiagnosticPaths(in, map[string]string{"/secret_key": "/unknown_key_abc"})
	if out[0].Path != "/unknown_key_abc" || out[1].Path != "/declared" {
		t.Fatalf("out=%+v", out)
	}
	if in[0].Path != "/secret_key" {
		t.Fatalf("input mutated: %+v", in)
	}
	if got := redactDiagnosticPaths(nil, nil); len(got) != 0 {
		t.Fatalf("nil diagnostics -> %+v", got)
	}
}

func TestPointerPathEscapesTildeBeforeSlash(t *testing.T) {
	if got := pointerPath("/p", "a~/b"); got != "/p/a~0~1b" {
		t.Fatalf("got %q", got)
	}
}

func TestSensitiveArgumentKeyNormalizesCaseAndDashes(t *testing.T) {
	for key, want := range map[string]bool{
		"API-Key": true, "x-auth-token": true, "DB_PASSWORD": true, "my-api_key": true,
		"Authorization": true, "client-credential": true, "name": false, "keyboard": false,
	} {
		if got := sensitiveArgumentKey(key); got != want {
			t.Errorf("sensitiveArgumentKey(%q)=%v want %v", key, got, want)
		}
	}
}

func TestCaptureCollisionCodeDistinguishesStaleRunTokenFromOtherCollisions(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	current := sha256Hex([]byte("run"))
	cases := []struct {
		name, path, want string
	}{
		{"unreadable path", dir, "result_collision"},
		{"invalid json", write("bad.json", "{"), "result_collision"},
		{"missing token hash", write("empty.json", `{"fixture_id":"x"}`), "result_collision"},
		{"same run token", write("same.json", `{"run_token_sha256":"`+current+`"}`), "result_collision"},
		{"different run token", write("stale.json", `{"run_token_sha256":"other"}`), "stale_run_token"},
	}
	for _, tc := range cases {
		if got := captureCollisionCode(tc.path, "run"); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestCaptureArgumentsRewritesDiagnosticPathsOfUnknownKeys(t *testing.T) {
	config := testProbeConfig(t, filepath.Join(t.TempDir(), "capture.json"))
	capture, err := captureArguments(config, []byte(`{"leaky_name":1}`))
	if err != nil {
		t.Fatal(err)
	}
	hashed := "unknown_key_" + sha256Hex([]byte("leaky_name"))[:12]
	if _, ok := capture.CanonicalArguments.(map[string]any)[hashed]; !ok {
		t.Fatalf("canonical=%#v", capture.CanonicalArguments)
	}
	for _, d := range capture.Diagnostics {
		if strings.Contains(d.Path, "leaky_name") {
			t.Fatalf("diagnostic leaks raw key: %+v", d)
		}
	}
	if capture.RawSHA256 == "" || capture.RunTokenSHA256 != sha256Hex([]byte("token")) || capture.FixtureID != config.FixtureID {
		t.Fatalf("capture=%+v", capture)
	}
}

func TestCaptureArgumentsAcceptsExactlyTheRawLimit(t *testing.T) {
	config := testProbeConfig(t, filepath.Join(t.TempDir(), "capture.json"))
	pad := strings.Repeat("a", conformanceRawLimit-len(`{"p":""}`))
	raw := []byte(`{"p":"` + pad + `"}`)
	if len(raw) != conformanceRawLimit {
		t.Fatalf("fixture len=%d", len(raw))
	}
	if _, err := captureArguments(config, raw); err != nil {
		t.Fatalf("raw at limit rejected: %v", err)
	}
}

func TestWriteCaptureFailsWhenParentIsNotADirectory(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeCapture(filepath.Join(blocker, "capture.json"), ConformanceCapture{}); err == nil {
		t.Fatal("capture written beneath a regular file")
	}
}

func TestProbeHandleReportsWriteFailureAndConsumesTheCall(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := testProbeConfig(t, filepath.Join(blocker, "capture.json"))
	probe, err := newConformanceProbe(config)
	if err != nil {
		t.Fatal(err)
	}
	result, err := probe.handle(t.Context(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: []byte(`{}`)}})
	if err != nil || !result.IsError || strings.Contains(probeText(result), `"ok":true`) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	// The failed call was still consumed, so a retry takes the second-call path;
	// the marker also cannot be written beneath a regular file.
	second, _ := probe.handle(t.Context(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: []byte(`{}`)}})
	if !second.IsError || !strings.Contains(probeText(second), "multiple_call_marker_failed") {
		t.Fatalf("second=%s", probeText(second))
	}
}

func TestProbeSecondCallRemovesCaptureWhenMarkerCannotBeWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.json")
	config := testProbeConfig(t, path)
	probe, err := newConformanceProbe(config)
	if err != nil {
		t.Fatal(err)
	}
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: []byte(`{}`)}}
	if result, _ := probe.handle(context.Background(), req); result.IsError {
		t.Fatalf("first call failed: %s", probeText(result))
	}
	// A directory squatting on the marker path makes the atomic rename fail.
	if err := os.Mkdir(path+".multiple", 0o700); err != nil {
		t.Fatal(err)
	}
	result, _ := probe.handle(context.Background(), req)
	if !result.IsError || !strings.Contains(probeText(result), "multiple_call_marker_failed") {
		t.Fatalf("result=%s", probeText(result))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("capture must be removed when double call cannot be recorded: %v", err)
	}
}

func TestWritePrivateJSONRejectsBadInputAndReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "marker.json")
	if err := writePrivateJSON(target, make(chan int)); err == nil {
		t.Fatal("unserializable value accepted")
	}
	if err := writePrivateJSON(target, map[string]int{"n": 1}); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateJSON(target, map[string]int{"n": 2}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != `{"n":2}` {
		t.Fatalf("data=%q err=%v", data, err)
	}
	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v err=%v", info, err)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temp files left behind: %v err=%v", entries, err)
	}
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateJSON(filepath.Join(blocker, "m.json"), 1); err == nil {
		t.Fatal("marker written beneath a regular file")
	}
}

func TestWriteMultipleCallMarkerRecordsCountAndHashedToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.json")
	if err := writeMultipleCallMarker(path, "tok", 3); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path + ".multiple")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("tok"))
	want := `{"call_count":3,"run_token_sha256":"` + fmtHash(sum) + `"}`
	if string(data) != want {
		t.Fatalf("marker=%s want=%s", data, want)
	}
}
