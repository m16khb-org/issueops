package mcpcli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	authoritycontract "issueops/internal/contract/authority"
	mcpcontract "issueops/internal/contract/mcp"
	authoritydomain "issueops/internal/domain/authority"
)

func TestValidateHTTPAddressAcceptsOnlyLiteralLoopbackWithPort(t *testing.T) {
	for address, ok := range map[string]bool{
		"127.0.0.1:47831": true,
		"[::1]:47831":     true,
		"127.0.0.2:1":     true,
		"localhost:47831": false,
		"0.0.0.0:47831":   false,
		"192.168.1.5:80":  false,
		"127.0.0.1":       false,
		"127.0.0.1:":      false,
		"":                false,
	} {
		err := ValidateHTTPAddress(address)
		if (err == nil) != ok {
			t.Errorf("ValidateHTTPAddress(%q) err=%v, want ok=%v", address, err, ok)
		}
	}
	if err := ValidateHTTPAddress("localhost:1"); err == nil || !strings.Contains(err.Error(), "loopback IP literal") {
		t.Fatalf("hostname rejection reason = %v", err)
	}
	if err := ValidateHTTPAddress("127.0.0.1:"); err == nil || !strings.Contains(err.Error(), "host:port") {
		t.Fatalf("empty port rejection reason = %v", err)
	}
}

func TestNewHTTPHandlerRejectsBadAddressAndShortBearerBeforeServing(t *testing.T) {
	deps := testTransportServices()
	good := strings.Repeat("A", 43)
	if _, err := NewHTTPHandler(deps, HTTPOptions{Address: "0.0.0.0:80", Bearer: good}); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("non-loopback address err=%v", err)
	}
	if _, err := NewHTTPHandler(deps, HTTPOptions{Address: "127.0.0.1:47831", Bearer: "short"}); err == nil || !strings.Contains(err.Error(), "256 bits") {
		t.Fatalf("short bearer err=%v", err)
	}
}

func TestValidateHTTPBearerRequiresDecodableThirtyTwoBytes(t *testing.T) {
	for name, bearer := range map[string]string{
		"empty":        "",
		"not base64":   "!!!!" + strings.Repeat("A", 40),
		"31 bytes":     strings.Repeat("A", 42),
		"padded input": strings.Repeat("A", 43) + "=",
	} {
		if err := validateHTTPBearer(bearer); err == nil {
			t.Errorf("%s: accepted %q", name, bearer)
		}
	}
	if err := validateHTTPBearer(strings.Repeat("A", 43)); err != nil {
		t.Fatalf("43 base64url chars (32 bytes) rejected: %v", err)
	}
}

func TestEnsureHTTPBearerRejectsRelativeStateDirAndLooseDirectory(t *testing.T) {
	if _, err := EnsureHTTPBearer("relative/state"); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative state dir err=%v", err)
	}
	state := canonicalTempDir(t)
	dir := filepath.Join(state, "mcp-http")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureHTTPBearer(state); err == nil || !strings.Contains(err.Error(), "owner-only directory") {
		t.Fatalf("group-readable state dir err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bearer")); !os.IsNotExist(err) {
		t.Fatalf("bearer created inside an unsafe directory: %v", err)
	}
}

func TestEnsureHTTPBearerRejectsWeakOrOversizedStoredSecret(t *testing.T) {
	for name, content := range map[string]string{
		"too short": "abc",
		"oversized": strings.Repeat("A", maxHTTPBearerSize+10),
	} {
		t.Run(name, func(t *testing.T) {
			state := canonicalTempDir(t)
			dir := filepath.Join(state, "mcp-http")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "bearer"), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := EnsureHTTPBearer(state); err == nil || !strings.Contains(err.Error(), "256 bits") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestEnsureHTTPBearerCreatesNoTempFilesAndTrimsWhitespace(t *testing.T) {
	state := canonicalTempDir(t)
	created, err := EnsureHTTPBearer(state)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(state, "mcp-http")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "bearer" {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bearer"), []byte(" "+created+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := EnsureHTTPBearer(state); err != nil || got != created {
		t.Fatalf("whitespace-wrapped bearer = %q err=%v", got, err)
	}
}

func TestStatusRecorderKeepsFirstStatusAndUnwrapsTarget(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: inner, status: http.StatusOK}
	rec.WriteHeader(http.StatusTeapot)
	rec.WriteHeader(http.StatusInternalServerError)
	if rec.status != http.StatusTeapot {
		t.Fatalf("recorded status = %d, want first (418)", rec.status)
	}
	if rec.Unwrap() != http.ResponseWriter(inner) {
		t.Fatal("Unwrap must return the underlying writer")
	}
	rec.Flush()
	if !inner.Flushed {
		t.Fatal("Flush not forwarded to a flushing writer")
	}
	// A writer without http.Flusher must not panic.
	(&statusRecorder{ResponseWriter: nonFlushingWriter{inner}}).Flush()
}

type nonFlushingWriter struct{ http.ResponseWriter }

func TestAuthorityErrorPayloadMapsEachErrorKindToItsCode(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		code any
	}{
		"request authority":      {authorityRequired("x"), authoritycontract.CodeRequired},
		"wrapped request":        {errors.Join(errors.New("ctx"), authorityInvalid("y")), authoritycontract.CodeInvalid},
		"domain authority":       {&authoritydomain.Error{Code: "authority_expired", Reason: "late"}, "authority_expired"},
		"invalid state sentinel": {authoritycontract.ErrInvalidState, authoritycontract.CodeInvalid},
		"unrelated":              {errors.New("boom"), nil},
	} {
		t.Run(name, func(t *testing.T) {
			payload := authorityErrorPayload(tc.err)
			if payload["ok"] != false || payload["error"] != tc.err.Error() {
				t.Fatalf("payload=%v", payload)
			}
			if got, present := payload["error_code"]; present != (tc.code != nil) || got != tc.code {
				t.Fatalf("error_code=%v present=%v, want %v", got, present, tc.code)
			}
		})
	}
}

func TestRecordRequestRootHandlesBlankIDAndMissingOrFailingLookup(t *testing.T) {
	ctx := t.Context()
	if root, err := recordRequestRoot(ctx, nil, RecordIssueOps, "  ", "/explicit", ""); err != nil || root != "/explicit" {
		t.Fatalf("blank id root=%q err=%v", root, err)
	}
	_, err := recordRequestRoot(ctx, nil, RecordIssueOps, "io-1", "", "")
	if payload := authorityErrorPayload(err); err == nil || payload["error_code"] != authoritycontract.CodeInvalid || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("nil lookup err=%v", err)
	}
	sentinel := errors.New("store offline")
	failing := func(context.Context, RecordKind, string) ([]string, error) { return nil, sentinel }
	if _, err := recordRequestRoot(ctx, failing, RecordLoop, "l-1", "", ""); !errors.Is(err, sentinel) {
		t.Fatalf("lookup failure not propagated: %v", err)
	}
	var gotKind RecordKind
	var gotID string
	vanished := func(_ context.Context, kind RecordKind, id string) ([]string, error) {
		gotKind, gotID = kind, id
		return []string{filepath.Join(t.TempDir(), "gone")}, nil
	}
	_, err = recordRequestRoot(ctx, vanished, RecordLoop, "l-2", "", "")
	if err == nil || !strings.Contains(err.Error(), "names no existing workspace") {
		t.Fatalf("vanished roots err=%v", err)
	}
	if gotKind != RecordLoop || gotID != "l-2" {
		t.Fatalf("lookup got kind=%q id=%q", gotKind, gotID)
	}
}

func TestCompileToolOutputSchemaResolvesOnlyDeclaredSchemas(t *testing.T) {
	catalog := mcpcontract.Catalog{Tools: []map[string]any{
		{"name": "plain"},
		{"name": "typed", "outputSchema": map[string]any{"type": "object", "required": []any{"ok"}}},
		{"name": "broken", "outputSchema": map[string]any{"type": "object", "properties": map[string]any{"a": map[string]any{"$ref": "#/nope"}}}},
		{"name": "unmarshalable", "outputSchema": map[string]any{"type": 5}},
	}}
	for _, name := range []string{"plain", "missing"} {
		if got, err := compileToolOutputSchema(catalog, name); got != nil || err != nil {
			t.Errorf("%s: schema=%v err=%v, want nil,nil", name, got, err)
		}
	}
	typed, err := compileToolOutputSchema(catalog, "typed")
	if err != nil || typed == nil {
		t.Fatalf("typed: %v %v", typed, err)
	}
	if typed.Validate(map[string]any{"ok": true}) != nil || typed.Validate(map[string]any{}) == nil {
		t.Fatal("compiled schema does not enforce required property")
	}
	for _, name := range []string{"broken", "unmarshalable"} {
		if got, err := compileToolOutputSchema(catalog, name); err == nil {
			t.Errorf("%s: invalid schema compiled: %v", name, got)
		}
	}
}

func TestValidatedStructuredContentOrdersFailuresAndHidesPayload(t *testing.T) {
	resolved, err := (&jsonschema.Schema{Type: "object", Required: []string{"ok"}}).Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	schemaErr, encodeErr := errors.New("schema broke"), errors.New("encode broke")
	if _, err := validatedStructuredContent(resolved, schemaErr, []byte(`{"ok":1}`), encodeErr); !errors.Is(err, schemaErr) {
		t.Fatalf("schema error must win: %v", err)
	}
	if _, err := validatedStructuredContent(resolved, nil, nil, encodeErr); !errors.Is(err, encodeErr) {
		t.Fatalf("encode error: %v", err)
	}
	if _, err := validatedStructuredContent(resolved, nil, []byte(`{nope`), nil); err == nil {
		t.Fatal("invalid JSON accepted")
	}
	_, err = validatedStructuredContent(resolved, nil, []byte(`{"leak":"s3cret-value"}`), nil)
	if err == nil || err.Error() != "output does not match outputSchema" || strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("mismatch err=%v", err)
	}
	got, err := validatedStructuredContent(resolved, nil, []byte(`{"ok":true}`), nil)
	if m, _ := got.(map[string]any); err != nil || m["ok"] != true {
		t.Fatalf("valid structured content = %v err=%v", got, err)
	}
}

func TestResolveHandlerGroupRejectsUnmappedToolsAndGroupsIgnoreForeignNames(t *testing.T) {
	deps := testTransportServices()
	deps.Catalog = mcpcontract.Catalog{Dispatch: map[string]mcpcontract.DispatchGroup{
		"foreign_gates":   mcpcontract.DispatchGates,
		"foreign_loop":    mcpcontract.DispatchLoop,
		"foreign_channel": mcpcontract.DispatchChannel,
		"foreign_issue":   mcpcontract.DispatchIssueOps,
	}}
	for name := range deps.Catalog.Dispatch {
		out := resolveHandlerGroup(deps, name)(t.Context(), MCPToolCall{Name: name})
		if out.Handled || out.Err != nil || out.Payload != nil {
			t.Errorf("%s: group claimed a tool it does not own: %+v", name, out)
		}
	}
	out := resolveHandlerGroup(deps, "not_registered")(t.Context(), MCPToolCall{Name: "not_registered"})
	if !out.Handled || out.Err == nil || out.Err.Code != -32602 || out.Err.Message != "Unknown tool" {
		t.Fatalf("unmapped tool outcome = %+v", out)
	}
}

func TestGroupOutcomeHelpersWrapErrorsAsToolErrorsAndPassPayloads(t *testing.T) {
	boom := errors.New("disk full")
	for name, outcome := range map[string]func(any, error, string) MCPToolOutcome{
		"gates": gatesMCPOutcome, "loop": loopMCPOutcome, "channel": channelMCPOutcome,
	} {
		failed := outcome(nil, boom, "Op failed")
		payload, _ := failed.Payload.(map[string]any)
		if !failed.Handled || !failed.IsError || failed.Err != nil || payload["ok"] != false || payload["error"] != "Op failed: disk full" {
			t.Errorf("%s error outcome = %+v", name, failed)
		}
		okOutcome := outcome(map[string]any{"n": 1}, nil, "Op failed")
		if !okOutcome.Handled || okOutcome.IsError || okOutcome.Payload.(map[string]any)["n"] != 1 {
			t.Errorf("%s success outcome = %+v", name, okOutcome)
		}
	}
}

func TestSDKResourceHandlerRejectsUnknownResourceURI(t *testing.T) {
	_, err := sdkResourceHandler(resourceConfigForTest())(t.Context(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "issueops://does-not-exist"},
	})
	if err == nil {
		t.Fatal("unknown resource URI resolved")
	}
}

func TestDecodeJSONRPCMessagesReadsSingleBatchAndIgnoresNonJSON(t *testing.T) {
	single := decodeJSONRPCMessages([]byte(" \t{\"id\":7,\"method\":\"tools/list\",\"params\":{}}\r\n"))
	if len(single) != 1 || single[0].Method != "tools/list" || string(single[0].ID) != "7" {
		t.Fatalf("single=%+v", single)
	}
	batch := decodeJSONRPCMessages([]byte(`[{"id":"a","method":"x"},{"method":"notify"}]`))
	if len(batch) != 2 || batch[0].Method != "x" || string(batch[0].ID) != `"a"` || len(batch[1].ID) != 0 {
		t.Fatalf("batch=%+v", batch)
	}
	for name, line := range map[string]string{
		"blank":            "  \n",
		"broken object":    `{"id":`,
		"broken batch":     `[{"id":1},`,
		"batch of scalars": `[1,2]`,
		"bare scalar":      `42`,
	} {
		if got := decodeJSONRPCMessages([]byte(line)); got != nil {
			t.Errorf("%s: got %+v, want nil", name, got)
		}
	}
}
