package mcpcli

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	docsadapter "issueops/internal/adapter/docs"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	docsapp "issueops/internal/application/docs"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestZZProbe(t *testing.T) {
	root := "/Users/m16khb/Workspace/issueops"
	t0 := time.Now()
	catalog := mcpcatalog.Build()
	t1 := time.Now()
	for _, tool := range catalog.Tools {
		name, _ := tool["name"].(string)
		_, _ = compileToolOutputSchema(catalog, name)
	}
	t2 := time.Now()
	for _, tool := range catalog.Tools {
		name, _ := tool["name"].(string)
		_, _ = prepareMCPToolInputSchema(catalog, name)
	}
	t3 := time.Now()
	server := mcp.NewServer(&mcp.Implementation{Name: "x", Version: "v"}, nil)
	registerAllTools(server, MCPDependencies{Catalog: catalog}, transportStdio, nil)
	t4 := time.Now()
	t.Logf("catalog.Build=%v output=%v input=%v registerAll=%v tools=%d", t1.Sub(t0), t2.Sub(t1), t3.Sub(t2), t4.Sub(t3), len(catalog.Tools))

	payload := (docsapp.Service{Observer: docsadapter.Observer{}, Now: time.Now}).Index(root, "v")
	output, outputErr := compileToolOutputSchema(catalog, "docs_index")
	for range 3 {
		a := time.Now()
		b, _ := json.MarshalIndent(payload, "", "  ")
		c := time.Now()
		encoded, _ := json.Marshal(payload)
		d := time.Now()
		var structured any
		_ = json.Unmarshal(encoded, &structured)
		e := time.Now()
		_ = output.Validate(structured)
		f := time.Now()
		res := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}, StructuredContent: structured}
		_, _ = json.Marshal(res)
		g := time.Now()
		t.Logf("indent=%v marshal=%v unmarshal=%v validate=%v resultMarshal=%v", c.Sub(a), d.Sub(c), e.Sub(d), f.Sub(e), g.Sub(f))
	}
	_ = outputErr
	h := sdkToolHandlerWithContext(catalog, func(context.Context, MCPToolCall) MCPToolOutcome { return mcpToolPayload(payload) }, "docs_index")
	for range 3 {
		a := time.Now()
		_, _ = h(t.Context(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{}`)}})
		t.Logf("handler=%v", time.Since(a))
	}
}
