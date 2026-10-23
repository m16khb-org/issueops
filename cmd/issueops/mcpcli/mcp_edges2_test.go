package mcpcli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gatescontract "issueops/internal/contract/gates"
)

func TestGatesBaseCheckRequestDefaultsWriteOnAndHonoursExplicitFalse(t *testing.T) {
	absent := gatesBaseCheckRequest(map[string]any{})
	if !absent.WriteAllowed || absent.NetworkAllowed {
		t.Fatalf("defaults write=%v network=%v, want write on, network off", absent.WriteAllowed, absent.NetworkAllowed)
	}
	if off := gatesBaseCheckRequest(map[string]any{"write_allowed": false}); off.WriteAllowed {
		t.Fatal("explicit write_allowed=false ignored")
	}
	if off := gatesBaseCheckRequest(map[string]any{"write_allowed": "false"}); off.WriteAllowed {
		t.Fatal(`string "false" must disable write`)
	}
	got := gatesBaseCheckRequest(map[string]any{
		"workspace_root": "/r", "cwd": "/r/p", "files": []any{"a", "b"}, "env_allowlist": []any{"HOME"}, "network_allowed": true,
	})
	want := gatescontract.CheckRequest{WorkspaceRoot: "/r", CWD: "/r/p", Files: []string{"a", "b"}, EnvAllowlist: []string{"HOME"}, WriteAllowed: true, NetworkAllowed: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request=%+v want=%+v", got, want)
	}
}

func TestSDKDirectToolResultRendersContentMapsAndFallsBackToJSON(t *testing.T) {
	text := func(r *mcp.CallToolResult) []string {
		var out []string
		for _, c := range r.Content {
			out = append(out, c.(*mcp.TextContent).Text)
		}
		return out
	}
	typed := sdkDirectToolResult(map[string]any{"isError": true, "content": []map[string]any{
		{"type": "text", "text": "one"}, {"type": "image", "text": "skipped"}, {"type": "text", "text": "two"},
	}})
	if !typed.IsError || !reflect.DeepEqual(text(typed), []string{"one", "two"}) {
		t.Fatalf("typed content result = %+v %v", typed, text(typed))
	}
	loose := sdkDirectToolResult(map[string]any{"content": []any{map[string]any{"type": "text", "text": "x"}, "not-a-map"}})
	if loose.IsError || !reflect.DeepEqual(text(loose), []string{"x"}) {
		t.Fatalf("[]any content result = %+v", loose)
	}
	fallback := sdkDirectToolResult(map[string]any{"a": 1})
	if fallback.IsError || len(fallback.Content) != 1 || text(fallback)[0] != `{"a":1}` {
		t.Fatalf("fallback = %v", text(fallback))
	}
	if got := text(sdkDirectToolResult([]int{1})); !reflect.DeepEqual(got, []string{"[1]"}) {
		t.Fatalf("non-map fallback = %v", got)
	}
}

func TestJSONRPCIDKeySeparatesStringsFromNumbersAndRejectsOtherShapes(t *testing.T) {
	for raw, want := range map[string]string{
		`"7"`:  "s:7",
		`7`:    "n:7",
		`2.0`:  "n:2",
		`1e3`:  "n:1000",
		` 5 `:  "n:5",
		``:     "",
		`null`: "",
		`true`: "",
		`{}`:   "",
		`[1]`:  "",
		`{bad`: "",
	} {
		if got := jsonRPCIDKey(json.RawMessage(raw)); got != want {
			t.Errorf("jsonRPCIDKey(%q)=%q want %q", raw, got, want)
		}
	}
}

func TestDecodeSDKToolArgumentsTreatsAbsentAndNullAsEmpty(t *testing.T) {
	for name, raw := range map[string]json.RawMessage{"absent": nil, "null": json.RawMessage(`null`)} {
		got, err := decodeSDKToolArguments(raw)
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("%s: got=%v err=%v", name, got, err)
		}
	}
	got, err := decodeSDKToolArguments(json.RawMessage(`{"a":1}`))
	if err != nil || got["a"] != float64(1) {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if _, err := decodeSDKToolArguments(json.RawMessage(`[1]`)); err == nil || !strings.Contains(err.Error(), "invalid arguments") {
		t.Fatalf("array arguments err=%v", err)
	}
}
