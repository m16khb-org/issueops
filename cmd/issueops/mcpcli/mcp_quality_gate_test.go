package mcpcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPAPIDocStaticGateFailureReturnsNormalPayload(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "users"), 0o755); err != nil {
		t.Fatal(err)
	}
	controller := `import { Controller, Get, Param } from '@nestjs/common'
import { ApiOperation, ApiResponse } from '@nestjs/swagger'

@Controller('users')
export class UsersController {
  @Get(':id')
  @ApiOperation({ summary: 'Find user', description: 'Find user' })
  @ApiResponse({ status: 200, description: 'ok' })
  find(@Param('id') id: string) {
    return { id }
  }
}
`
	file := filepath.Join(root, "src", "users", "users.controller.ts")
	if err := os.WriteFile(file, []byte(controller), 0o644); err != nil {
		t.Fatal(err)
	}

	params, err := json.Marshal(map[string]any{
		"name": "api_doc_static_check",
		"arguments": map[string]any{
			"repo":  root,
			"files": []string{"src/users/users.controller.ts"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, rpcErr := testHandleToolCall(params)
	if rpcErr != nil {
		t.Fatalf("quality gate failure should be a normal MCP payload, got rpc error: %+v", rpcErr)
	}
	text := extractSingleTextResult(t, payload)
	if !strings.Contains(text, `"ok": false`) || !strings.Contains(text, `"violations"`) || !strings.Contains(text, `"missing_api_param"`) {
		t.Fatalf("unexpected payload: %s", text)
	}
}

func TestQualityGateSentinelsAreRecognizedAsNormalMCPOutcomes(t *testing.T) {
	for _, mode := range []string{"pending", "review-fail", "static-fail"} {
		deps := apiDocOutcomeDeps(t, "gate", mode)
		tool := "api_doc_review"
		if mode == "static-fail" {
			tool = "api_doc_static_check"
		}
		args := map[string]any{}
		if mode == "review-fail" {
			args["result_file"] = "result"
		}
		raw, _ := json.Marshal(map[string]any{"name": tool, "arguments": args})
		payload, rpcErr := HandleToolCallWithDependencies(raw, deps)
		if rpcErr != nil {
			t.Fatalf("%s gate became protocol error: %v", mode, rpcErr)
		}
		if text := extractSingleTextResult(t, payload); !strings.Contains(text, `"ok": false`) {
			t.Fatalf("%s gate outcome=%s", mode, text)
		}
	}
	if !isSelfVerificationGateError(errSelfVerificationGateFailed) {
		t.Fatal("self-verification gate sentinel should be recognized")
	}
}

func extractSingleTextResult(t *testing.T, payload any) string {
	t.Helper()
	result, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T", payload)
	}
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) != 1 {
		t.Fatalf("unexpected content: %#v", result["content"])
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatalf("unexpected text content: %#v", content[0])
	}
	return text
}
