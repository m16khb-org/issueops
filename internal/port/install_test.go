package port_test

import (
	"encoding/json"
	"strings"
	"testing"

	"issueops/internal/port"
)

func TestNativeInstallRequestDoesNotSerializeMCPBearer(t *testing.T) {
	request := port.NativeInstallRequest{
		MCPTransport: "http",
		MCPURL:       "http://127.0.0.1:47831/mcp",
		MCPBearer:    "private-bearer-fixture",
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), request.MCPBearer) {
		t.Fatal("serialized install request contains its MCP bearer")
	}
}
