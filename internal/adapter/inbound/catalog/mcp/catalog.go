package mcp

import contract "issueops/internal/contract/mcp"

// catalogSection binds one catalog function to its dispatch handler group.
type catalogSection struct {
	group contract.DispatchGroup
	tools func() []contract.Tool
}

// catalogSections is the single ordered source of truth for the MCP tool
// catalog. Both the advertised tools/list (AdvertisedTools) and the
// name->handler routing table (DispatchMap) derive from this slice, so adding a
// tool means editing exactly one catalog function referenced here. The
// advertised order matches the stable mcp_tools.golden.json snapshot.
func catalogSections() []catalogSection {
	return []catalogSection{
		{contract.DispatchProject, contract.CoreProjectTools},
		{contract.DispatchPolicyState, contract.CommandPolicyTools},
		{contract.DispatchPolicyState, contract.StateTools},
		{contract.DispatchIssueOps, contract.IssueOpsBasicTools},
		{contract.DispatchLoop, contract.LoopTools},
		{contract.DispatchGates, contract.GatesTools},
		{contract.DispatchChannel, contract.ChannelTools},
		{contract.DispatchSelfLoop, contract.SelfLoopAdvertisedTools},
		{contract.DispatchAssistantWorker, contract.AdapterOwnedTools},
		{contract.DispatchPolicyState, contract.CommandPolicyAuditTools},
		{contract.DispatchAssistantWorker, contract.LocalAssistantTools},
	}
}

// AdvertisedTools returns every MCP tool advertised in tools/list, in the
// stable order pinned by mcp_tools.golden.json.
func AdvertisedTools() []contract.Tool {
	var out []contract.Tool
	for _, s := range catalogSections() {
		out = append(out, s.tools()...)
	}
	return out
}

// DispatchMap returns a map from every MCP tool name to its handler group.
// It derives from catalogSections so routing can never drift from the catalog:
// adding a tool to a section makes it both routable and listed.
func DispatchMap() map[string]contract.DispatchGroup {
	out := make(map[string]contract.DispatchGroup)
	for _, s := range catalogSections() {
		for _, t := range s.tools() {
			out[t.Name] = s.group
		}
	}
	return out
}

func ToolMaps(tools []contract.Tool) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		toolMap := map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		}
		if tool.OutputSchema != nil {
			toolMap["outputSchema"] = tool.OutputSchema
		}
		if tool.Annotations != nil {
			toolMap["annotations"] = tool.Annotations
		}
		out = append(out, toolMap)
	}
	return out
}

func ResourceMaps(resources []contract.Resource) []map[string]any {
	out := make([]map[string]any, 0, len(resources))
	for _, resource := range resources {
		out = append(out, map[string]any{
			"uri":         resource.URI,
			"name":        resource.Name,
			"description": resource.Description,
			"mimeType":    resource.MimeType,
		})
	}
	return out
}

func Build() contract.Catalog {
	return contract.Catalog{Tools: withAuthorityFields(ToolMaps(AdvertisedTools())), Resources: ResourceMaps(contract.Resources()), Dispatch: DispatchMap()}
}
