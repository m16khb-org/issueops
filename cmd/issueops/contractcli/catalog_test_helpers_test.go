package contractcli

import (
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	mcpcontract "issueops/internal/contract/mcp"
)

func testConformanceCatalog() []mcpcontract.Tool { return mcpcatalog.AdvertisedTools() }

func init() {
	MCPTools = func() []map[string]any { return mcpcatalog.Build().Tools }
	ConfigureConformance(ConformanceDependencies{Catalog: testConformanceCatalog})
}
